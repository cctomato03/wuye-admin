// 住户管理：人员档案 + 房屋关系（业主/家人/租户）+ 有效期/续期/还原 + 人脸 + 历史记录。
//
// 隔离要点：
//   - pm_resident 是租户级人员档案（跨小区共享，证件号唯一）；
//   - 房屋关系（pm_resident_house）是小区级的，物业人员只能看到被授权小区内的关系；
//   - 按证件号回显只返回基础资料 + "在其他登记过的小区数量"，绝不返回其它小区的房屋明细。
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
	"web-basic/pkg/jsonx"
	"web-basic/pkg/mask"
)

// 关系状态
const (
	RelStatusValid    = 1 // 有效
	RelStatusExpired  = 2 // 已到期
	RelStatusReleased = 3 // 已解除
)

// 关系变更动作
const (
	RelActionBind    = 1
	RelActionRenew   = 2
	RelActionExpire  = 3
	RelActionRelease = 4
	RelActionDelete  = 5
	RelActionRestore = 6
)

// ResidentQuery 住户列表查询。
type ResidentQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	HouseID     jsonx.Uint64 `form:"houseId"`
	BuildingID  jsonx.Uint64 `form:"buildingId"`
	Keyword     string       `form:"keyword"` // 姓名/手机号/证件号前缀
	Name        string       `form:"name"`
	IDNo        string       `form:"idNo"`
	Phone       string       `form:"phone"`
	RelRole     *int         `form:"relRole"`
	RelStatus   *int         `form:"relStatus"`
	Expiring    *bool        `form:"expiring"` // 仅看 30 天内到期
}

// ResidentRow 住户列表行（关系 + 人员 + 房屋）。
type ResidentRow struct {
	RelID         uint64     `json:"relId,string"`
	ResidentID    uint64     `json:"residentId,string"`
	CommunityID   uint64     `json:"communityId,string"`
	CommunityName string     `json:"communityName"`
	HouseID       uint64     `json:"houseId,string"`
	HouseName     string     `json:"houseName"`
	Name          string     `json:"name"`
	IDNo          string     `json:"idNo"`
	Phone         string     `json:"phone"`
	Gender        int        `json:"gender"`
	PhotoURL      string     `json:"photoUrl"`
	RelRole       int        `json:"relRole"`
	Relation      string     `json:"relation"`
	IsMain        bool       `json:"isMain"`
	StartDate     *time.Time `json:"startDate"`
	EndDate       *time.Time `json:"endDate"`
	RelStatus     int        `json:"relStatus"`
}

// ListResidents 住户（房屋关系）分页列表。
func (s *Service) ListResidents(ctx context.Context, q *ResidentQuery, p *principal.Info) ([]ResidentRow, int64, error) {
	q.Normalize()
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
	}
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Table("pm_resident_house rh").
		Joins("JOIN pm_resident r ON r.id = rh.resident_id AND r.deleted = 0").
		Joins("LEFT JOIN pm_house h ON h.id = rh.house_id").
		Joins("LEFT JOIN pm_community c ON c.id = rh.community_id").
		Where("rh.deleted = 0")
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("rh.tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return []ResidentRow{}, 0, nil
		}
		tx = tx.Where("rh.community_id IN ?", ids)
	}
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("rh.community_id = ?", q.CommunityID.Uint64())
	}
	if q.HouseID.Uint64() != 0 {
		tx = tx.Where("rh.house_id = ?", q.HouseID.Uint64())
	}
	if q.BuildingID.Uint64() != 0 {
		tx = tx.Where("h.building_id = ?", q.BuildingID.Uint64())
	}
	if q.Name != "" {
		tx = tx.Where("r.name LIKE ?", likePrefix(q.Name))
	}
	if q.IDNo != "" {
		tx = tx.Where("r.id_no LIKE ?", likePrefix(q.IDNo))
	}
	if q.Phone != "" {
		tx = tx.Where("r.phone LIKE ?", likePrefix(q.Phone))
	}
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(r.name LIKE ? OR r.phone LIKE ? OR r.id_no LIKE ?)", kw+"%", kw+"%", kw+"%")
	}
	if q.RelRole != nil {
		tx = tx.Where("rh.rel_role = ?", *q.RelRole)
	}
	if q.RelStatus != nil {
		tx = tx.Where("rh.status = ?", *q.RelStatus)
	}
	if q.Expiring != nil && *q.Expiring {
		tx = tx.Where("rh.status = 1 AND rh.end_date IS NOT NULL AND rh.end_date <= ?", today().AddDate(0, 0, 30))
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]ResidentRow, 0, q.PageSize)
	err := tx.Select(`rh.id AS rel_id, rh.resident_id, rh.community_id, c.name AS community_name,
			rh.house_id, IFNULL(h.full_name, '') AS house_name, r.name, r.id_no, r.phone, r.gender, r.photo_url,
			rh.rel_role, rh.relation, rh.is_main, rh.start_date, rh.end_date, rh.status AS rel_status`).
		Order("rh.community_id ASC, rh.house_id ASC, rh.rel_role ASC").Offset(q.Offset()).Limit(q.PageSize).
		Scan(&list).Error
	if err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	// 证件号脱敏（列表不展示完整证件号，详情/编辑再取完整值）
	for i := range list {
		list[i].IDNo = mask.IDCard(list[i].IDNo)
	}
	return list, total, nil
}

// ResidentVO 住户详情。
type ResidentVO struct {
	entity.Resident
	Houses              []ResidentHouseVO     `json:"houses"`
	Faces               []entity.ResidentFace `json:"faces"`
	OtherCommunityCount int                   `json:"otherCommunityCount"` // 其它小区登记数（不含名称，防越权探测）
}

// ResidentHouseVO 住户在某小区的房屋关系。
type ResidentHouseVO struct {
	entity.ResidentHouse
	HouseName     string `json:"houseName"`
	CommunityName string `json:"communityName"`
}

// GetResident 住户详情（房屋关系只返回当前用户可见的小区）。
func (s *Service) GetResident(ctx context.Context, residentID uint64, p *principal.Info) (*ResidentVO, error) {
	r, err := s.residentScoped(ctx, p, residentID)
	if err != nil {
		return nil, err
	}
	vo := &ResidentVO{Resident: *r}
	all, ids := s.communityScope(p)
	relTx := s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).Where("resident_id = ?", r.ID)
	if t := s.tenantIDOf(p); t != 0 {
		relTx = relTx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return vo, nil
		}
		relTx = relTx.Where("community_id IN ?", ids)
	}
	var rels []entity.ResidentHouse
	if err := relTx.Order("status ASC, id DESC").Find(&rels).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	houseNames := map[uint64]string{}
	commNames := map[uint64]string{}
	if len(rels) > 0 {
		hid := make([]uint64, 0, len(rels))
		cid := make([]uint64, 0, len(rels))
		for _, rel := range rels {
			hid = append(hid, rel.HouseID)
			cid = append(cid, rel.CommunityID)
		}
		houseNames = s.houseNameMap(ctx, hid)
		commNames = s.nameMap(ctx, &entity.Community{}, cid)
	}
	visibleCommunity := map[uint64]struct{}{}
	for _, rel := range rels {
		visibleCommunity[rel.CommunityID] = struct{}{}
		vo.Houses = append(vo.Houses, ResidentHouseVO{ResidentHouse: rel, HouseName: houseNames[rel.HouseID], CommunityName: commNames[rel.CommunityID]})
	}
	// 其它小区登记数量（只给数量，不给名称与房屋，避免跨小区信息泄露）
	var otherCount int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).
		Where("resident_id = ? AND status = 1", r.ID).Count(&otherCount).Error; err == nil {
		vo.OtherCommunityCount = int(otherCount) - len(visibleCommunity)
		if vo.OtherCommunityCount < 0 {
			vo.OtherCommunityCount = 0
		}
	}
	var faces []entity.ResidentFace
	_ = s.Repo.DB.WithContext(ctx).Where("resident_id = ?", r.ID).Order("is_primary DESC, id DESC").Find(&faces).Error
	vo.Faces = faces
	return vo, nil
}

// houseNameMap 房屋 ID → 全称。
func (s *Service) houseNameMap(ctx context.Context, ids []uint64) map[uint64]string {
	out := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return out
	}
	var rows []struct {
		ID       uint64
		FullName string
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.House{}).Select("id, full_name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		out[r.ID] = r.FullName
	}
	return out
}

// residentScoped 带作用域读取住户：必须有至少一个可见小区的房屋关系，否则视为不存在。
func (s *Service) residentScoped(ctx context.Context, p *principal.Info, residentID uint64) (*entity.Resident, error) {
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Resident{}).Where("id = ?", residentID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return nil, errs.ErrNotFound
		}
		sub := s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).
			Select("resident_id").Where("community_id IN ?", ids)
		tx = tx.Where("id IN (?)", sub)
	}
	var r entity.Resident
	if err := tx.First(&r).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &r, nil
}

// ResidentReq 新增/修改住户入参。
type ResidentReq struct {
	Name             string `json:"name" binding:"required,max=64"`
	IDType           int    `json:"idType"`
	IDNo             string `json:"idNo" binding:"required,max=64"`
	Gender           int    `json:"gender"`
	Birthday         string `json:"birthday"`
	Phone            string `json:"phone" binding:"max=20"`
	Mobile2          string `json:"mobile2" binding:"max=20"`
	Nation           string `json:"nation" binding:"max=32"`
	NativePlace      string `json:"nativePlace" binding:"max=64"`
	HouseholdAddr    string `json:"householdAddr" binding:"max=255"`
	LiveAddr         string `json:"liveAddr" binding:"max=255"`
	PhotoURL         string `json:"photoUrl" binding:"max=500"`
	EmergencyContact string `json:"emergencyContact" binding:"max=64"`
	EmergencyPhone   string `json:"emergencyPhone" binding:"max=20"`
	Status           int    `json:"status"`
	Remark           string `json:"remark" binding:"max=500"`

	// 房屋关系（新增时可一并登记）
	CommunityID jsonx.Uint64 `json:"communityId"`
	HouseID     jsonx.Uint64 `json:"houseId"`
	RelRole     int          `json:"relRole"`
	Relation    string       `json:"relation"`
	StartDate   string       `json:"startDate"`
	EndDate     string       `json:"endDate"`
	IsMain      bool         `json:"isMain"`
}

// CreateResidentResult 新增住户结果。
type CreateResidentResult struct {
	ResidentID uint64 `json:"residentId,string"`
	RelationID uint64 `json:"relationId,string"`
	// Reused 为 true 表示该证件在本物业公司已存在档案，本次只是新增了房屋关系
	Reused bool `json:"reused"`
}

// CreateResident 登记住户（可同时绑定房屋）。
func (s *Service) CreateResident(ctx context.Context, req *ResidentReq, p *principal.Info) (*CreateResidentResult, error) {
	name := strings.TrimSpace(req.Name)
	idNo := strings.TrimSpace(req.IDNo)
	if name == "" || idNo == "" {
		return nil, errs.Invalid("姓名与证件号码为必填")
	}
	idType := pmDefault(req.IDType, 1)
	tenantID := p.TenantID
	cid := req.CommunityID.Uint64()
	if cid != 0 {
		if err := s.MustCommunity(ctx, p, cid); err != nil {
			return nil, err
		}
		com, err := s.findCommunity(ctx, p, cid)
		if err != nil {
			return nil, err
		}
		tenantID = com.TenantID
	}
	birthday, err := parseDate(req.Birthday)
	if err != nil {
		return nil, err
	}

	result := &CreateResidentResult{}
	now := time.Now()
	// 1) 同租户内按证件号复用已有档案（一人一档，跨小区共享）
	var exist entity.Resident
	err = s.Repo.DB.WithContext(ctx).Where("tenant_id = ? AND id_type = ? AND id_no = ?", tenantID, idType, idNo).First(&exist).Error
	switch {
	case err == nil:
		result.ResidentID = exist.ID
		result.Reused = true
		// 回填/更新基础资料（允许按需修改）
		if err := s.Repo.DB.WithContext(ctx).Model(&entity.Resident{}).Where("id = ?", exist.ID).Updates(map[string]any{
			"name":              name,
			"gender":            req.Gender,
			"birthday":          birthday,
			"phone":             req.Phone,
			"mobile2":           req.Mobile2,
			"nation":            req.Nation,
			"native_place":      req.NativePlace,
			"household_addr":    req.HouseholdAddr,
			"live_addr":         req.LiveAddr,
			"photo_url":         req.PhotoURL,
			"emergency_contact": req.EmergencyContact,
			"emergency_phone":   req.EmergencyPhone,
			"update_by":         p.UserID,
			"update_time":       now,
		}).Error; err != nil {
			return nil, errs.ErrDB.WithErr(err)
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		r := &entity.Resident{
			Base:             entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
			TenantID:         tenantID,
			Name:             name,
			IDType:           idType,
			IDNo:             idNo,
			Gender:           req.Gender,
			Birthday:         birthday,
			Phone:            req.Phone,
			Mobile2:          req.Mobile2,
			Nation:           req.Nation,
			NativePlace:      req.NativePlace,
			HouseholdAddr:    req.HouseholdAddr,
			LiveAddr:         req.LiveAddr,
			PhotoURL:         req.PhotoURL,
			EmergencyContact: req.EmergencyContact,
			EmergencyPhone:   req.EmergencyPhone,
			FirstCommunityID: cid,
			Status:           pmDefault(req.Status, 1),
		}
		if err := s.Repo.DB.WithContext(ctx).Create(r).Error; err != nil {
			return nil, wrapTxErr(err)
		}
		result.ResidentID = r.ID
	default:
		return nil, errs.ErrDB.WithErr(err)
	}

	// 2) 绑定房屋关系
	if req.HouseID.Uint64() != 0 {
		relID, err := s.bindHouse(ctx, p, &BindHouseReq{
			ResidentID:  jsonx.Uint64(result.ResidentID),
			HouseID:     req.HouseID,
			RelRole:     pmDefault(req.RelRole, 1),
			Relation:    req.Relation,
			StartDate:   req.StartDate,
			EndDate:     req.EndDate,
			IsMain:      req.IsMain,
			CommunityID: req.CommunityID,
		})
		if err != nil {
			return nil, err
		}
		result.RelationID = relID
	}
	return result, nil
}

// UpdateResident 修改住户基础资料。
func (s *Service) UpdateResident(ctx context.Context, residentID uint64, req *ResidentReq, p *principal.Info) error {
	r, err := s.residentScoped(ctx, p, residentID)
	if err != nil {
		return err
	}
	birthday, err := parseDate(req.Birthday)
	if err != nil {
		return err
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Resident{}).Where("id = ?", r.ID).Updates(map[string]any{
		"name":              strings.TrimSpace(req.Name),
		"id_type":           pmDefault(req.IDType, r.IDType),
		"id_no":             strings.TrimSpace(req.IDNo),
		"gender":            req.Gender,
		"birthday":          birthday,
		"phone":             req.Phone,
		"mobile2":           req.Mobile2,
		"nation":            req.Nation,
		"native_place":      req.NativePlace,
		"household_addr":    req.HouseholdAddr,
		"live_addr":         req.LiveAddr,
		"photo_url":         req.PhotoURL,
		"emergency_contact": req.EmergencyContact,
		"emergency_phone":   req.EmergencyPhone,
		"status":            pmDefault(req.Status, 1),
		"remark":            req.Remark,
		"update_by":         p.UserID,
		"update_time":       time.Now(),
	}).Error; err != nil {
		return wrapTxErr(err)
	}
	return nil
}

// DeleteResident 删除住户（软删除）：同时解除可见小区内的房屋关系并撤销门禁权限。
func (s *Service) DeleteResident(ctx context.Context, residentID uint64, p *principal.Info) error {
	r, err := s.residentScoped(ctx, p, residentID)
	if err != nil {
		return err
	}
	all, ids := s.communityScope(p)
	relTx := s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).Where("resident_id = ? AND status = 1", r.ID)
	if !all {
		if len(ids) == 0 {
			return errs.ErrNotFound
		}
		relTx = relTx.Where("community_id IN ?", ids)
	}
	var rels []entity.ResidentHouse
	if err := relTx.Find(&rels).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	now := time.Now()
	for _, rel := range rels {
		if err := s.releaseRelation(ctx, p, &rel, RelActionDelete, now); err != nil {
			return err
		}
	}
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.Resident{}, r.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// BindHouseReq 绑定房屋入参。
type BindHouseReq struct {
	ResidentID  jsonx.Uint64 `json:"residentId" binding:"required"`
	HouseID     jsonx.Uint64 `json:"houseId" binding:"required"`
	CommunityID jsonx.Uint64 `json:"communityId"`
	RelRole     int          `json:"relRole" binding:"omitempty,oneof=1 2 3"`
	Relation    string       `json:"relation" binding:"max=32"`
	StartDate   string       `json:"startDate"`
	EndDate     string       `json:"endDate"`
	IsMain      bool         `json:"isMain"`
	Remark      string       `json:"remark" binding:"max=500"`
}

// BindHousePublic 绑定住户与房屋（对外入口，供 API 层调用）。
func (s *Service) BindHousePublic(ctx context.Context, req *BindHouseReq, p *principal.Info) (uint64, error) {
	return s.bindHouse(ctx, p, req)
}

// bindHouse 绑定住户与房屋（含有效期校验与门禁同步）。
func (s *Service) bindHouse(ctx context.Context, p *principal.Info, req *BindHouseReq) (uint64, error) {
	r, err := s.residentScoped(ctx, p, req.ResidentID.Uint64())
	if err != nil {
		return 0, err
	}
	h, err := s.findHouse(ctx, p, req.HouseID.Uint64())
	if err != nil {
		return 0, err
	}
	role := pmDefault(req.RelRole, 1)
	start, err := parseDate(req.StartDate)
	if err != nil {
		return 0, err
	}
	end, err := parseDate(req.EndDate)
	if err != nil {
		return 0, err
	}
	if role == 3 && end == nil {
		return 0, errs.Invalid("租户必须填写到期日期")
	}
	if end != nil && start != nil && end.Before(*start) {
		return 0, errs.Invalid("到期日期不能早于开始日期")
	}
	// 已有同房屋同住户的有效关系则直接返回
	var exist entity.ResidentHouse
	if err := s.Repo.DB.WithContext(ctx).Where("community_id = ? AND house_id = ? AND resident_id = ? AND status = 1",
		h.CommunityID, h.ID, r.ID).First(&exist).Error; err == nil {
		return exist.ID, nil
	}
	now := time.Now()
	rel := &entity.ResidentHouse{
		Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:    h.TenantID,
		CommunityID: h.CommunityID,
		ResidentID:  r.ID,
		HouseID:     h.ID,
		RelRole:     role,
		Relation:    req.Relation,
		IsMain:      req.IsMain,
		StartDate:   start,
		EndDate:     end,
		Status:      RelStatusValid,
		BindTime:    &now,
	}
	if rel.StartDate == nil {
		t := today()
		rel.StartDate = &t
	}
	if err := s.Repo.DB.WithContext(ctx).Create(rel).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	s.logRel(ctx, rel, RelActionBind, nil, rel.EndDate, p)
	s.recalcHouseCounts(ctx, h.ID)
	// 从房屋层级向上追溯门禁设备并下发权限（异步失败不影响登记）
	s.syncResidentAccess(ctx, h.CommunityID, r.ID, p)
	return rel.ID, nil
}

// ListResidentHouses 某住户可见的房屋关系。
func (s *Service) ListResidentHouses(ctx context.Context, residentID uint64, p *principal.Info) ([]ResidentHouseVO, error) {
	vo, err := s.GetResident(ctx, residentID, p)
	if err != nil {
		return nil, err
	}
	return vo.Houses, nil
}

// HouseResidentVO 房屋下的住户（横向列表用）。
type HouseResidentVO struct {
	entity.ResidentHouse
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	IDNo     string `json:"idNo"`
	PhotoURL string `json:"photoUrl"`
}

// ListHouseResidents 某房屋下的住户（含已解除的历史关系可选用 status 过滤）。
func (s *Service) ListHouseResidents(ctx context.Context, houseID uint64, status *int, p *principal.Info) ([]HouseResidentVO, error) {
	h, err := s.findHouse(ctx, p, houseID)
	if err != nil {
		return nil, err
	}
	tx := s.Repo.DB.WithContext(ctx).Table("pm_resident_house rh").
		Joins("JOIN pm_resident r ON r.id = rh.resident_id AND r.deleted = 0").
		Where("rh.house_id = ? AND rh.deleted = 0", h.ID)
	if status != nil {
		tx = tx.Where("rh.status = ?", *status)
	}
	list := make([]HouseResidentVO, 0, 8)
	if err := tx.Select(`rh.*, r.name, r.phone, r.id_no, r.photo_url`).
		Order("rh.status ASC, rh.rel_role ASC").Scan(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	for i := range list {
		list[i].IDNo = mask.IDCard(list[i].IDNo)
	}
	return list, nil
}

// UnbindHouse 解除关系（保留历史记录）。
func (s *Service) UnbindHouse(ctx context.Context, relID uint64, p *principal.Info) error {
	rel, err := s.relScoped(ctx, p, relID)
	if err != nil {
		return err
	}
	if rel.Status != RelStatusValid {
		return errs.Invalid("该关系已失效，无需解除")
	}
	return s.releaseRelation(ctx, p, rel, RelActionRelease, time.Now())
}

// releaseRelation 解除关系：置为已解除 + 写历史 + 重算计数 + 撤销门禁。
func (s *Service) releaseRelation(ctx context.Context, p *principal.Info, rel *entity.ResidentHouse, action int, now time.Time) error {
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).Where("id = ?", rel.ID).Updates(map[string]any{
		"status":      RelStatusReleased,
		"unbind_time": now,
		"update_by":   p.UserID,
		"update_time": now,
	}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	s.logRel(ctx, rel, action, rel.EndDate, rel.EndDate, p)
	s.recalcHouseCounts(ctx, rel.HouseID)
	s.revokeResidentAccess(ctx, rel.CommunityID, rel.ResidentID, rel.HouseID, p)
	return nil
}

// RenewHouseReq 续期入参。
type RenewHouseReq struct {
	EndDate string `json:"endDate" binding:"required"`
	RelRole int    `json:"relRole"`
	Remark  string `json:"remark" binding:"max=500"`
}

// RenewHouse 续期：更新到期日并恢复有效状态。
func (s *Service) RenewHouse(ctx context.Context, relID uint64, req *RenewHouseReq, p *principal.Info) error {
	rel, err := s.relScoped(ctx, p, relID)
	if err != nil {
		return err
	}
	end, err := parseDate(req.EndDate)
	if err != nil {
		return err
	}
	if end == nil || end.Before(today()) {
		return errs.Invalid("续期后的到期日期必须晚于今天")
	}
	before := rel.EndDate
	fields := map[string]any{
		"end_date":    end,
		"status":      RelStatusValid,
		"unbind_time": nil,
		"update_by":   p.UserID,
		"update_time": time.Now(),
	}
	if req.RelRole != 0 {
		fields["rel_role"] = req.RelRole
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).Where("id = ?", rel.ID).Updates(fields).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	s.logRel(ctx, rel, RelActionRenew, before, end, p)
	s.recalcHouseCounts(ctx, rel.HouseID)
	s.syncResidentAccess(ctx, rel.CommunityID, rel.ResidentID, p)
	return nil
}

// RestoreHouse 还原已删除/已到期的关系。
func (s *Service) RestoreHouse(ctx context.Context, relID uint64, p *principal.Info) error {
	rel, err := s.relScoped(ctx, p, relID)
	if err != nil {
		return err
	}
	if rel.Status == RelStatusValid {
		return errs.Invalid("该关系已是有效状态")
	}
	// 已过期的关系必须先续期
	if rel.EndDate != nil && rel.EndDate.Before(today()) {
		return errs.Invalid("该关系已过期，请使用「续期」先更新到期日期")
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).Where("id = ?", rel.ID).Updates(map[string]any{
		"status":      RelStatusValid,
		"unbind_time": nil,
		"update_by":   p.UserID,
		"update_time": time.Now(),
	}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	s.logRel(ctx, rel, RelActionRestore, rel.EndDate, rel.EndDate, p)
	s.recalcHouseCounts(ctx, rel.HouseID)
	s.syncResidentAccess(ctx, rel.CommunityID, rel.ResidentID, p)
	return nil
}

// relScoped 带作用域读取房屋关系。
func (s *Service) relScoped(ctx context.Context, p *principal.Info, relID uint64) (*entity.ResidentHouse, error) {
	var rel entity.ResidentHouse
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).Where("id = ?", relID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return nil, errs.ErrNotFound
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	if err := tx.First(&rel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &rel, nil
}

// logRel 写关系变更历史。
func (s *Service) logRel(ctx context.Context, rel *entity.ResidentHouse, action int, beforeEnd, afterEnd *time.Time, p *principal.Info) {
	now := time.Now()
	log := &entity.ResidentHouseLog{
		Base:         entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now},
		TenantID:     rel.TenantID,
		CommunityID:  rel.CommunityID,
		ResidentID:   rel.ResidentID,
		HouseID:      rel.HouseID,
		RelID:        rel.ID,
		Action:       action,
		BeforeEnd:    beforeEnd,
		AfterEnd:     afterEnd,
		BeforeRole:   rel.RelRole,
		AfterRole:    rel.RelRole,
		OperatorID:   p.UserID,
		OperatorName: p.Nickname,
		OperateTime:  now,
	}
	if log.OperatorName == "" {
		log.OperatorName = p.Username
	}
	_ = s.Repo.DB.WithContext(context.WithoutCancel(ctx)).Create(log).Error
}

// ListResidentHistory 住户关系历史（含续期/到期/解除/删除/还原）。
func (s *Service) ListResidentHistory(ctx context.Context, residentID uint64, p *principal.Info) ([]entity.ResidentHouseLog, error) {
	if _, err := s.residentScoped(ctx, p, residentID); err != nil {
		return nil, err
	}
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouseLog{}).Where("resident_id = ?", residentID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return []entity.ResidentHouseLog{}, nil
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	var list []entity.ResidentHouseLog
	if err := tx.Order("id DESC").Limit(200).Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// recalcHouseCounts 重算房屋在册业主数/住户数。
func (s *Service) recalcHouseCounts(ctx context.Context, houseID uint64) {
	if houseID == 0 {
		return
	}
	var owners, residents int64
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).
		Where("house_id = ? AND status = 1 AND rel_role = 1", houseID).Count(&owners).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).
		Where("house_id = ? AND status = 1", houseID).Count(&residents).Error
	status := 1
	if residents > 0 {
		status = 2
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.House{}).Where("id = ?", houseID).
		Updates(map[string]any{"owner_count": owners, "resident_count": residents, "house_status": status}).Error
}

// ExpireRelations 把已过期的有效关系置为"已到期"（定时任务调用）。
func (s *Service) ExpireRelations(ctx context.Context) (int, error) {
	now := time.Now()
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).
		Where("status = 1 AND end_date IS NOT NULL AND end_date < ?", today()).Limit(5000)
	var rels []entity.ResidentHouse
	if err := tx.Find(&rels).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	for i := range rels {
		rel := rels[i]
		if err := s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).Where("id = ?", rel.ID).Updates(map[string]any{
			"status":      RelStatusExpired,
			"unbind_time": now,
			"update_time": now,
		}).Error; err != nil {
			return i, errs.ErrDB.WithErr(err)
		}
		operator := &principal.Info{UserID: 0, Username: "system", Nickname: "系统任务"}
		s.logRel(ctx, &rel, RelActionExpire, rel.EndDate, rel.EndDate, operator)
		s.recalcHouseCounts(ctx, rel.HouseID)
		// 到期解除门禁
		s.revokeResidentAccess(ctx, rel.CommunityID, rel.ResidentID, rel.HouseID, operator)
	}
	return len(rels), nil
}

// ============================ 证件回显 ============================

// ResidentLookupVO 按证件号回显结果（跨小区只回显基础资料，不含其它小区房屋）。
type ResidentLookupVO struct {
	Found         bool   `json:"found"`
	ResidentID    uint64 `json:"residentId,string"`
	Name          string `json:"name"`
	Gender        int    `json:"gender"`
	Phone         string `json:"phone"`
	PhotoURL      string `json:"photoUrl"`
	IDType        int    `json:"idType"`
	IDNo          string `json:"idNo"`
	Birthday      string `json:"birthday"`
	Nation        string `json:"nation"`
	NativePlace   string `json:"nativePlace"`
	HouseholdAddr string `json:"householdAddr"`
	// RegisteredCommunityCount 当前登录人可见小区内的登记数
	RegisteredCommunityCount int `json:"registeredCommunityCount"`
	// HasOtherCommunity 是否还在其它（未授权）小区登记过——只给布尔值，不泄露具体小区
	HasOtherCommunity bool `json:"hasOtherCommunity"`
}

// LookupResidentByIDNo 按证件号查询可回显的住户资料。
func (s *Service) LookupResidentByIDNo(ctx context.Context, idType int, idNo string, p *principal.Info) (*ResidentLookupVO, error) {
	idNo = strings.TrimSpace(idNo)
	if idNo == "" {
		return nil, errs.Invalid("请填写证件号码")
	}
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Resident{}).Where("id_no = ?", idNo)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if idType != 0 {
		tx = tx.Where("id_type = ?", idType)
	}
	var r entity.Resident
	if err := tx.First(&r).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &ResidentLookupVO{Found: false}, nil
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	vo := &ResidentLookupVO{
		Found:         true,
		ResidentID:    r.ID,
		Name:          r.Name,
		Gender:        r.Gender,
		Phone:         r.Phone,
		PhotoURL:      r.PhotoURL,
		IDType:        r.IDType,
		IDNo:          r.IDNo,
		Nation:        r.Nation,
		NativePlace:   r.NativePlace,
		HouseholdAddr: r.HouseholdAddr,
	}
	if r.Birthday != nil {
		vo.Birthday = r.Birthday.Format("2006-01-02")
	}
	var total, visible int64
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).
		Where("resident_id = ? AND status = 1", r.ID).Count(&total).Error
	all, ids := s.communityScope(p)
	if all {
		visible = total
	} else if len(ids) > 0 {
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).
			Where("resident_id = ? AND status = 1 AND community_id IN ?", r.ID, ids).Count(&visible).Error
	}
	vo.RegisteredCommunityCount = int(visible)
	vo.HasOtherCommunity = total > visible
	return vo, nil
}

// ============================ 人脸 ============================

// FaceReq 人脸照片入参。
type FaceReq struct {
	CommunityID jsonx.Uint64 `json:"communityId" binding:"required"`
	FileID      jsonx.Uint64 `json:"fileId"`
	FileURL     string       `json:"fileUrl" binding:"required,max=500"`
	IsPrimary   bool         `json:"isPrimary"`
	Remark      string       `json:"remark" binding:"max=500"`
}

// AddResidentFace 新增住户人脸（会触发门禁人脸下发）。
func (s *Service) AddResidentFace(ctx context.Context, residentID uint64, req *FaceReq, p *principal.Info) (uint64, error) {
	r, err := s.residentScoped(ctx, p, residentID)
	if err != nil {
		return 0, err
	}
	if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
		return 0, err
	}
	now := time.Now()
	f := &entity.ResidentFace{
		Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:    r.TenantID,
		CommunityID: req.CommunityID.Uint64(),
		ResidentID:  r.ID,
		FileID:      req.FileID.Uint64(),
		FileURL:     req.FileURL,
		Status:      1,
		IsPrimary:   req.IsPrimary,
	}
	if req.IsPrimary {
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.ResidentFace{}).
			Where("resident_id = ? AND community_id = ?", r.ID, req.CommunityID.Uint64()).
			Updates(map[string]any{"is_primary": false}).Error
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.Resident{}).Where("id = ?", r.ID).
			Updates(map[string]any{"photo_url": req.FileURL}).Error
	}
	if err := s.Repo.DB.WithContext(ctx).Create(f).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	s.syncResidentAccess(ctx, req.CommunityID.Uint64(), r.ID, p)
	return f.ID, nil
}

// DeleteResidentFace 删除人脸照片。
func (s *Service) DeleteResidentFace(ctx context.Context, faceID uint64, p *principal.Info) error {
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.ResidentFace{}).Where("id = ?", faceID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return errs.ErrNotFound
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	var f entity.ResidentFace
	if err := tx.First(&f).Error; err != nil {
		return errs.ErrNotFound
	}
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.ResidentFace{}, f.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// ListResidentFaces 住户人脸列表。
func (s *Service) ListResidentFaces(ctx context.Context, residentID uint64, p *principal.Info) ([]entity.ResidentFace, error) {
	if _, err := s.residentScoped(ctx, p, residentID); err != nil {
		return nil, err
	}
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.ResidentFace{}).Where("resident_id = ?", residentID)
	if !all {
		if len(ids) == 0 {
			return []entity.ResidentFace{}, nil
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	var list []entity.ResidentFace
	if err := tx.Order("is_primary DESC, id DESC").Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// ResidentExpireJob 定时任务入口：处理到期住户关系。
func (s *Service) ResidentExpireJob(ctx context.Context, _ map[string]any) (string, error) {
	n, err := s.ExpireRelations(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("处理到期住户关系 %d 条", n), nil
}
