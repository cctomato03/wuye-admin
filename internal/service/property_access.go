// 门禁授权：按人员/房屋/层级计算设备集合 → 生成授权明细 → 下发 → 失败重试 → 到期解除。
//
// 下发规则：
//   - 住户/访客：房屋 → 楼层 → 单元 → 楼栋 → 区域 → 小区，逐级收集绑定在该层级的设备（去重）
//   - 物业人员：小区下全部启用门禁的设备
//   - 其他人员：登记时手工勾选的设备
//
// 第三方对接：Issue/Revoke 走 AccessIssuer 接口，默认 stub 实现（不调用外部系统），
// 接真实平台时实现该接口并在 bootstrap 注入即可，业务代码不用改。
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
)

// 人员类型
const (
	PersonTypeResident = 1
	PersonTypeEmployee = 2
	PersonTypeVisitor  = 3
	PersonTypeOther    = 4
)

// 下发类型
const (
	GrantTypeFace  = 1
	GrantTypeCard  = 2
	GrantTypePhone = 3
)

// 下发状态
const (
	GrantStatusPending = 1
	GrantStatusIssuing = 2
	GrantStatusSuccess = 3
	GrantStatusFailed  = 4
	GrantStatusRevoked = 5
	GrantStatusExpired = 6
)

// ConfigKeyAccessStubSuccess 门禁下发占位实现是否返回成功（默认 false：等待对接真实平台）。
const ConfigKeyAccessStubSuccess = "property.access.stub_success"

// IssueRequest 下发请求（第三方平台对接契约）。
type IssueRequest struct {
	CommunityID uint64
	Device      entity.Device
	PersonType  int
	PersonID    uint64
	PersonName  string
	GrantType   int
	CardNo      string
	Phone       string
	FaceURL     string
	ExpireAt    *time.Time
}

// IssueResponse 下发结果。
type IssueResponse struct {
	Success bool
	Msg     string
}

// RevokeRequest 解除请求。
type RevokeRequest struct {
	CommunityID uint64
	Device      entity.Device
	PersonType  int
	PersonID    uint64
	GrantType   int
	CardNo      string
	Phone       string
}

// AccessIssuer 门禁平台对接接口（第三方实现）。
type AccessIssuer interface {
	Issue(ctx context.Context, req IssueRequest) (IssueResponse, error)
	Revoke(ctx context.Context, req RevokeRequest) error
}

// stubIssuer 占位实现：不调用任何第三方系统。
// 由参数 property.access.stub_success 控制返回成功/失败，便于演示与联调。
type stubIssuer struct {
	success bool
}

func (i *stubIssuer) Issue(ctx context.Context, req IssueRequest) (IssueResponse, error) {
	if i.success {
		return IssueResponse{Success: true, Msg: "已下发（占位实现，未对接真实门禁平台）"}, nil
	}
	return IssueResponse{Success: false, Msg: "门禁平台未对接：请完成第三方接口对接后重试"}, nil
}

func (i *stubIssuer) Revoke(ctx context.Context, req RevokeRequest) error { return nil }

// accessIssuer 返回当前使用的下发实现。
func (s *Service) accessIssuer(ctx context.Context) AccessIssuer {
	return &stubIssuer{success: s.configBool(ctx, ConfigKeyAccessStubSuccess, false)}
}

// bindPair 绑定层级 + 对象 ID
type bindPair struct {
	level int
	id    uint64
}

// GetDeviceTree 收集某个房屋关联的全部门禁设备（自下而上逐级追溯）。
func (s *Service) collectHouseDevices(ctx context.Context, communityID, houseID uint64) ([]entity.Device, error) {
	var h entity.House
	if err := s.Repo.DB.WithContext(ctx).Where("id = ? AND community_id = ?", houseID, communityID).First(&h).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	// 层级模型：房屋挂节点，从节点沿 path 一路向上，收集绑定在各级节点上的设备；
	// 同时兼容历史 bind_level/bind_id（老数据里的 ID 已随迁移保留为节点 ID）。
	nodeIDs := []uint64{communityID}
	if h.NodeID != 0 {
		var node entity.StructureNode
		if err := s.Repo.DB.WithContext(ctx).Where("id = ?", h.NodeID).First(&node).Error; err == nil {
			nodeIDs = append(nodeIDs, parsePathIDs(node.Path)...)
			nodeIDs = append(nodeIDs, node.ID)
		}
	}
	return s.collectDevicesByNodes(ctx, communityID, nodeIDs)
}

// collectCommunityDevices 小区全部门禁设备（物业人员用）。
func (s *Service) collectCommunityDevices(ctx context.Context, communityID uint64) ([]entity.Device, error) {
	return s.collectDevicesByNodes(ctx, communityID, nil)
}

// collectDevicesByNodes 按层级节点收集设备：
//   - nodeIDs 为空：返回小区全部启用设备
//   - 否则：bind_node_id ∈ nodeIDs 或 (bind_level>1 且 bind_id ∈ nodeIDs) 或 (bind_level=1 且 bind_id=小区ID)
func (s *Service) collectDevicesByNodes(ctx context.Context, communityID uint64, nodeIDs []uint64) ([]entity.Device, error) {
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Device{}).
		Where("community_id = ? AND access_enabled = 1 AND status = 1", communityID)
	if len(nodeIDs) > 0 {
		cond := s.Repo.DB.WithContext(ctx).Where("bind_node_id IN ?", nodeIDs).
			Or("(bind_level = 1 AND bind_id = ?)", communityID)
		cond = cond.Or("(bind_level > 1 AND bind_id IN ?)", nodeIDs)
		tx = tx.Where(cond)
	}
	var list []entity.Device
	if err := tx.Order("id ASC").Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// collectDevices 按 (层级, 对象ID) 收集设备；pairs 为空表示小区全部设备。
func (s *Service) collectDevices(ctx context.Context, communityID uint64, pairs []bindPair) ([]entity.Device, error) {
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Device{}).
		Where("community_id = ? AND access_enabled = 1 AND status = 1", communityID)
	if len(pairs) > 0 {
		// 用 OR 组合多个层级条件，仍能命中 idx_comm_bind
		cond := s.Repo.DB.WithContext(ctx)
		first := true
		for _, p := range pairs {
			if first {
				cond = cond.Where("(bind_level = ? AND bind_id = ?)", p.level, p.id)
				first = false
				continue
			}
			cond = cond.Or("bind_level = ? AND bind_id = ?", p.level, p.id)
		}
		tx = tx.Where(cond)
	}
	var list []entity.Device
	if err := tx.Order("id ASC").Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// ============================ 住户 ============================

// syncResidentAccess 住户登记/续期后同步门禁授权：
// 以"该住户在该小区所有有效房屋关系"为依据，展开 (房屋设备 × 下发类型)。
func (s *Service) syncResidentAccess(ctx context.Context, communityID, residentID uint64, p *principal.Info) {
	var rels []entity.ResidentHouse
	if err := s.Repo.DB.WithContext(ctx).
		Where("community_id = ? AND resident_id = ? AND status = 1", communityID, residentID).
		Find(&rels).Error; err != nil || len(rels) == 0 {
		return
	}
	var resident entity.Resident
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", residentID).First(&resident).Error; err != nil {
		return
	}
	var faces []entity.ResidentFace
	_ = s.Repo.DB.WithContext(ctx).Where("community_id = ? AND resident_id = ?", communityID, residentID).
		Order("is_primary DESC, id DESC").Find(&faces).Error
	faceURL := resident.PhotoURL
	if faceURL == "" && len(faces) > 0 {
		faceURL = faces[0].FileURL
	}
	var cards []entity.AccessCard
	_ = s.Repo.DB.WithContext(ctx).Where("community_id = ? AND person_type = ? AND person_id = ? AND status = 1",
		communityID, PersonTypeResident, residentID).Find(&cards).Error

	for _, rel := range rels {
		devices, err := s.collectHouseDevices(ctx, communityID, rel.HouseID)
		if err != nil {
			continue
		}
		houseName := s.houseNameOf(ctx, rel.HouseID)
		grants := make([]*entity.AccessGrant, 0, len(devices)*3)
		for _, d := range devices {
			if faceURL != "" {
				grants = append(grants, s.newGrant(communityID, rel.TenantID, PersonTypeResident, residentID, resident.Name,
					rel.HouseID, houseName, GrantTypeFace, d, "", resident.Phone, faceURL, rel.EndDate, p))
			}
			for _, c := range cards {
				grants = append(grants, s.newGrant(communityID, rel.TenantID, PersonTypeResident, residentID, resident.Name,
					rel.HouseID, houseName, GrantTypeCard, d, c.CardNo, resident.Phone, faceURL, rel.EndDate, p))
			}
			if resident.Phone != "" {
				grants = append(grants, s.newGrant(communityID, rel.TenantID, PersonTypeResident, residentID, resident.Name,
					rel.HouseID, houseName, GrantTypePhone, d, "", resident.Phone, faceURL, rel.EndDate, p))
			}
		}
		s.persistAndIssue(ctx, grants, p)
	}
}

// revokeResidentAccess 撤销住户门禁权限（解除关系/删除/到期时调用）。
// houseID=0 表示撤销该小区下全部授权。
func (s *Service) revokeResidentAccess(ctx context.Context, communityID, residentID, houseID uint64, p *principal.Info) {
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.AccessGrant{}).
		Where("community_id = ? AND person_type = ? AND person_id = ? AND status IN ?",
			communityID, PersonTypeResident, residentID, []int{GrantStatusPending, GrantStatusIssuing, GrantStatusSuccess, GrantStatusFailed})
	if houseID != 0 {
		tx = tx.Where("house_id = ?", houseID)
	}
	var grants []entity.AccessGrant
	if err := tx.Find(&grants).Error; err != nil || len(grants) == 0 {
		return
	}
	s.revokeGrantsInternal(ctx, &grants, GrantStatusRevoked, p)
}

// ============================ 访客 ============================

// syncVisitorAccess 访客登记后按受访房屋下发门禁（有效期=访客预约结束时间）。
func (s *Service) syncVisitorAccess(ctx context.Context, v *entity.Visitor, p *principal.Info) {
	if v.VisitHouseID == 0 {
		return
	}
	devices, err := s.collectHouseDevices(ctx, v.CommunityID, v.VisitHouseID)
	if err != nil {
		return
	}
	houseName := s.houseNameOf(ctx, v.VisitHouseID)
	grants := make([]*entity.AccessGrant, 0, len(devices)*2)
	for _, d := range devices {
		if v.FaceURL != "" {
			grants = append(grants, s.newGrant(v.CommunityID, v.TenantID, PersonTypeVisitor, v.ID, v.Name,
				v.VisitHouseID, houseName, GrantTypeFace, d, "", v.Phone, v.FaceURL, v.EndTime, p))
		}
		if v.Phone != "" {
			grants = append(grants, s.newGrant(v.CommunityID, v.TenantID, PersonTypeVisitor, v.ID, v.Name,
				v.VisitHouseID, houseName, GrantTypePhone, d, "", v.Phone, v.FaceURL, v.EndTime, p))
		}
	}
	s.persistAndIssue(ctx, grants, p)
}

// revokeVisitorAccess 访客离场/取消/过期时撤销。
func (s *Service) revokeVisitorAccess(ctx context.Context, communityID, visitorID uint64, p *principal.Info) {
	var grants []entity.AccessGrant
	if err := s.Repo.DB.WithContext(ctx).
		Where("community_id = ? AND person_type = ? AND person_id = ? AND status IN ?",
			communityID, PersonTypeVisitor, visitorID,
			[]int{GrantStatusPending, GrantStatusIssuing, GrantStatusSuccess, GrantStatusFailed}).
		Find(&grants).Error; err != nil || len(grants) == 0 {
		return
	}
	s.revokeGrantsInternal(ctx, &grants, GrantStatusRevoked, p)
}

// ============================ 物业人员 ============================

// syncEmployeeAccess 物业人员：下发小区全部门禁设备（人脸/手机拨号）。
func (s *Service) syncEmployeeAccess(ctx context.Context, communityID uint64, emp *entity.Employee, p *principal.Info) {
	devices, err := s.collectCommunityDevices(ctx, communityID)
	if err != nil {
		return
	}
	faceURL := emp.FaceURL
	if faceURL == "" && emp.UserID != 0 {
		var u entity.User
		if err := s.Repo.DB.WithContext(ctx).Where("id = ?", emp.UserID).First(&u).Error; err == nil {
			faceURL = u.Avatar
		}
	}
	var cards []entity.AccessCard
	_ = s.Repo.DB.WithContext(ctx).Where("community_id = ? AND person_type = ? AND person_id = ? AND status = 1",
		communityID, PersonTypeEmployee, emp.ID).Find(&cards).Error
	grants := make([]*entity.AccessGrant, 0, len(devices)*3)
	for _, d := range devices {
		if faceURL != "" {
			grants = append(grants, s.newGrant(communityID, emp.TenantID, PersonTypeEmployee, emp.ID, emp.Name,
				0, "", GrantTypeFace, d, "", emp.Phone, faceURL, nil, p))
		}
		for _, c := range cards {
			grants = append(grants, s.newGrant(communityID, emp.TenantID, PersonTypeEmployee, emp.ID, emp.Name,
				0, "", GrantTypeCard, d, c.CardNo, emp.Phone, faceURL, nil, p))
		}
		if emp.Phone != "" {
			grants = append(grants, s.newGrant(communityID, emp.TenantID, PersonTypeEmployee, emp.ID, emp.Name,
				0, "", GrantTypePhone, d, "", emp.Phone, faceURL, nil, p))
		}
	}
	s.persistAndIssue(ctx, grants, p)
}

// ============================ 其他人员 ============================

// syncOtherPersonAccess 其他人员：只下发登记时勾选的设备。
func (s *Service) syncOtherPersonAccess(ctx context.Context, person *entity.OtherPerson, p *principal.Info) {
	var pds []entity.OtherPersonDevice
	if err := s.Repo.DB.WithContext(ctx).Where("person_id = ?", person.ID).Find(&pds).Error; err != nil || len(pds) == 0 {
		return
	}
	dids := make([]uint64, 0, len(pds))
	for _, pd := range pds {
		dids = append(dids, pd.DeviceID)
	}
	var devices []entity.Device
	if err := s.Repo.DB.WithContext(ctx).Where("community_id = ? AND status = 1 AND id IN ?", person.CommunityID, dids).
		Find(&devices).Error; err != nil {
		return
	}
	grants := make([]*entity.AccessGrant, 0, len(devices)*3)
	expire := person.ValidTo
	for _, d := range devices {
		if person.FaceURL != "" {
			grants = append(grants, s.newGrant(person.CommunityID, person.TenantID, PersonTypeOther, person.ID, person.Name,
				0, "", GrantTypeFace, d, "", person.Phone, person.FaceURL, expire, p))
		}
		if person.CardNo != "" {
			grants = append(grants, s.newGrant(person.CommunityID, person.TenantID, PersonTypeOther, person.ID, person.Name,
				0, "", GrantTypeCard, d, person.CardNo, person.Phone, person.FaceURL, expire, p))
		}
		if person.Phone != "" {
			grants = append(grants, s.newGrant(person.CommunityID, person.TenantID, PersonTypeOther, person.ID, person.Name,
				0, "", GrantTypePhone, d, "", person.Phone, person.FaceURL, expire, p))
		}
	}
	s.persistAndIssue(ctx, grants, p)
}

// revokeOtherPersonAccess 撤销其他人员门禁。
func (s *Service) revokeOtherPersonAccess(ctx context.Context, communityID, personID uint64, p *principal.Info) {
	var grants []entity.AccessGrant
	if err := s.Repo.DB.WithContext(ctx).
		Where("community_id = ? AND person_type = ? AND person_id = ? AND status IN ?",
			communityID, PersonTypeOther, personID,
			[]int{GrantStatusPending, GrantStatusIssuing, GrantStatusSuccess, GrantStatusFailed}).
		Find(&grants).Error; err != nil || len(grants) == 0 {
		return
	}
	s.revokeGrantsInternal(ctx, &grants, GrantStatusRevoked, p)
}

// ============================ 通用下发 ============================

// newGrant 构造一条授权明细（不落库）。
func (s *Service) newGrant(communityID, tenantID uint64, personType int, personID uint64, personName string,
	houseID uint64, houseName string, grantType int, d entity.Device,
	cardNo, phone, faceURL string, expire *time.Time, p *principal.Info) *entity.AccessGrant {
	now := time.Now()
	return &entity.AccessGrant{
		Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now},
		TenantID:    tenantID,
		CommunityID: communityID,
		PersonType:  personType,
		PersonID:    personID,
		PersonName:  personName,
		HouseID:     houseID,
		HouseName:   houseName,
		GrantType:   grantType,
		DeviceID:    d.ID,
		DeviceName:  d.Name,
		CardNo:      cardNo,
		Phone:       phone,
		FaceURL:     faceURL,
		Status:      GrantStatusPending,
		ExpireTime:  expire,
	}
}

// persistAndIssue 去重落库并立即下发。
func (s *Service) persistAndIssue(ctx context.Context, grants []*entity.AccessGrant, p *principal.Info) {
	if len(grants) == 0 {
		return
	}
	// 去重：同 (人员, 类型, 设备) 已有未撤销的授权则跳过
	var exist []entity.AccessGrant
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.AccessGrant{}).
		Where("person_type = ? AND person_id = ? AND status IN ?",
			grants[0].PersonType, grants[0].PersonID,
			[]int{GrantStatusPending, GrantStatusIssuing, GrantStatusSuccess, GrantStatusFailed}).
		Find(&exist).Error; err != nil {
		return
	}
	key := func(personType int, personID uint64, grantType int, deviceID uint64) string {
		return fmt.Sprintf("%d-%d-%d-%d", personType, personID, grantType, deviceID)
	}
	seen := make(map[string]struct{}, len(exist))
	for _, e := range exist {
		seen[key(e.PersonType, e.PersonID, e.GrantType, e.DeviceID)] = struct{}{}
	}
	batchNo := newBatchNo("G")
	fresh := make([]*entity.AccessGrant, 0, len(grants))
	for _, g := range grants {
		k := key(g.PersonType, g.PersonID, g.GrantType, g.DeviceID)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		g.BatchNo = batchNo
		fresh = append(fresh, g)
	}
	if len(fresh) == 0 {
		return
	}
	if err := s.Repo.DB.WithContext(ctx).CreateInBatches(fresh, 500).Error; err != nil {
		return
	}
	_, _ = s.issueGrants(ctx, fresh, p, batchNo, 1)
}

// issueGrants 调第三方下发，逐条更新结果，并写批次任务。
func (s *Service) issueGrants(ctx context.Context, grants []*entity.AccessGrant, p *principal.Info, batchNo string, action int) (success, failed int) {
	issuer := s.accessIssuer(ctx)
	now := time.Now()
	for _, g := range grants {
		var d entity.Device
		_ = s.Repo.DB.WithContext(ctx).Where("id = ?", g.DeviceID).First(&d).Error
		resp, err := issuer.Issue(ctx, IssueRequest{
			CommunityID: g.CommunityID,
			Device:      d,
			PersonType:  g.PersonType,
			PersonID:    g.PersonID,
			PersonName:  g.PersonName,
			GrantType:   g.GrantType,
			CardNo:      g.CardNo,
			Phone:       g.Phone,
			FaceURL:     g.FaceURL,
			ExpireAt:    g.ExpireTime,
		})
		status := GrantStatusSuccess
		msg := resp.Msg
		if err != nil {
			status = GrantStatusFailed
			msg = fmtErr(err)
		} else if !resp.Success {
			status = GrantStatusFailed
		}
		if status == GrantStatusSuccess {
			success++
		} else {
			failed++
		}
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.AccessGrant{}).Where("id = ?", g.ID).Updates(map[string]any{
			"status":          status,
			"last_msg":        truncateStr(msg, 500),
			"last_issue_time": now,
			"update_time":     now,
			"batch_no":        batchNo,
		}).Error
		g.Status = status
		g.LastMsg = msg
		g.LastIssueTime = &now
	}
	// 批次任务
	task := &entity.AccessTask{
		Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now},
		TenantID:    firstNonZero(grantTenant(grants), p.TenantID),
		CommunityID: firstGrantCommunity(grants),
		BatchNo:     batchNo,
		PersonType:  firstGrantPersonType(grants),
		PersonID:    firstGrantPersonID(grants),
		PersonName:  firstGrantPersonName(grants),
		Action:      action,
		Total:       len(grants),
		Success:     success,
		Failed:      failed,
		Status:      2,
		Message:     fmt.Sprintf("成功 %d 条，失败 %d 条", success, failed),
		StartTime:   &now,
		EndTime:     &now,
	}
	if failed > 0 {
		task.Status = 3
	}
	if success == 0 && failed > 0 {
		task.Status = 4
	}
	_ = s.Repo.DB.WithContext(ctx).Create(task).Error
	return success, failed
}

func grantTenant(grants []*entity.AccessGrant) uint64 {
	if len(grants) == 0 {
		return 0
	}
	return grants[0].TenantID
}

func firstGrantCommunity(grants []*entity.AccessGrant) uint64 {
	if len(grants) == 0 {
		return 0
	}
	return grants[0].CommunityID
}

func firstGrantPersonType(grants []*entity.AccessGrant) int {
	if len(grants) == 0 {
		return 0
	}
	return grants[0].PersonType
}

func firstGrantPersonID(grants []*entity.AccessGrant) uint64 {
	if len(grants) == 0 {
		return 0
	}
	return grants[0].PersonID
}

func firstGrantPersonName(grants []*entity.AccessGrant) string {
	if len(grants) == 0 {
		return ""
	}
	return grants[0].PersonName
}

func firstNonZero(v, def uint64) uint64 {
	if v != 0 {
		return v
	}
	return def
}

// revokeGrantsInternal 批量解除并调用第三方。
func (s *Service) revokeGrantsInternal(ctx context.Context, grants *[]entity.AccessGrant, status int, p *principal.Info) error {
	issuer := s.accessIssuer(ctx)
	now := time.Now()
	for i := range *grants {
		g := (*grants)[i]
		var d entity.Device
		_ = s.Repo.DB.WithContext(ctx).Where("id = ?", g.DeviceID).First(&d).Error
		_ = issuer.Revoke(ctx, RevokeRequest{
			CommunityID: g.CommunityID,
			Device:      d,
			PersonType:  g.PersonType,
			PersonID:    g.PersonID,
			GrantType:   g.GrantType,
			CardNo:      g.CardNo,
			Phone:       g.Phone,
		})
		if err := s.Repo.DB.WithContext(ctx).Model(&entity.AccessGrant{}).Where("id = ?", g.ID).Updates(map[string]any{
			"status":       status,
			"revoked_time": now,
			"update_time":  now,
		}).Error; err != nil {
			return errs.ErrDB.WithErr(err)
		}
	}
	return nil
}

// ============================ 对外接口 ============================

// AccessGrantQuery 授权明细查询。
type AccessGrantQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	PersonType  *int         `form:"personType"`
	PersonID    jsonx.Uint64 `form:"personId"`
	PersonName  string       `form:"personName"`
	GrantType   *int         `form:"grantType"`
	Status      *int         `form:"status"`
	DeviceID    jsonx.Uint64 `form:"deviceId"`
	Keyword     string       `form:"keyword"`
}

// ListAccessGrants 授权明细分页。
func (s *Service) ListAccessGrants(ctx context.Context, q *AccessGrantQuery, p *principal.Info) ([]entity.AccessGrant, int64, error) {
	q.Normalize()
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
	}
	tx := s.scoped(ctx, &entity.AccessGrant{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.PersonType != nil {
		tx = tx.Where("person_type = ?", *q.PersonType)
	}
	if q.PersonID.Uint64() != 0 {
		tx = tx.Where("person_id = ?", q.PersonID.Uint64())
	}
	if q.PersonName != "" {
		tx = tx.Where("person_name LIKE ?", likePrefix(q.PersonName))
	}
	if q.GrantType != nil {
		tx = tx.Where("grant_type = ?", *q.GrantType)
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	if q.DeviceID.Uint64() != 0 {
		tx = tx.Where("device_id = ?", q.DeviceID.Uint64())
	}
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(person_name LIKE ? OR device_name LIKE ? OR card_no LIKE ? OR phone LIKE ?)", kw+"%", kw+"%", kw+"%", kw+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.AccessGrant, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// AccessStats 授权统计（按状态分组）。
type AccessStats struct {
	Total   int64 `json:"total"`
	Pending int64 `json:"pending"`
	Success int64 `json:"success"`
	Failed  int64 `json:"failed"`
	Revoked int64 `json:"revoked"`
	Expired int64 `json:"expired"`
}

// AccessGrantStats 统计当前小区授权情况。
func (s *Service) AccessGrantStats(ctx context.Context, communityID uint64, p *principal.Info) (*AccessStats, error) {
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	var rows []struct {
		Status int
		Count  int64
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.AccessGrant{}).
		Select("status, COUNT(*) AS count").Where("community_id = ?", communityID).
		Group("status").Scan(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	st := &AccessStats{}
	for _, r := range rows {
		st.Total += r.Count
		switch r.Status {
		case GrantStatusPending, GrantStatusIssuing:
			st.Pending += r.Count
		case GrantStatusSuccess:
			st.Success += r.Count
		case GrantStatusFailed:
			st.Failed += r.Count
		case GrantStatusRevoked:
			st.Revoked += r.Count
		case GrantStatusExpired:
			st.Expired += r.Count
		}
	}
	return st, nil
}

// RetryGrants 失败重试（可指定授权 ID 列表，为空则重试当前可见小区全部失败记录）。
func (s *Service) RetryGrants(ctx context.Context, grantIDs []uint64, communityID uint64, p *principal.Info) (int, int, error) {
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.AccessGrant{})
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return 0, 0, errs.ErrDataForbidden
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	if len(grantIDs) > 0 {
		tx = tx.Where("id IN ?", grantIDs)
	} else {
		if communityID == 0 {
			return 0, 0, errs.Invalid("请指定要重试的授权记录或小区")
		}
		if err := s.MustCommunity(ctx, p, communityID); err != nil {
			return 0, 0, err
		}
		tx = tx.Where("community_id = ? AND status = ?", communityID, GrantStatusFailed).Limit(1000)
	}
	var grants []entity.AccessGrant
	if err := tx.Find(&grants).Error; err != nil {
		return 0, 0, errs.ErrDB.WithErr(err)
	}
	if len(grants) == 0 {
		return 0, 0, errs.Invalid("没有需要重试的授权记录")
	}
	for i := range grants {
		grants[i].RetryCount++
	}
	// 批量更新重试次数
	idsToTouch := make([]uint64, 0, len(grants))
	for _, g := range grants {
		idsToTouch = append(idsToTouch, g.ID)
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.AccessGrant{}).Where("id IN ?", idsToTouch).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
	ptr := make([]*entity.AccessGrant, 0, len(grants))
	for i := range grants {
		ptr = append(ptr, &grants[i])
	}
	success, failed := s.issueGrants(ctx, ptr, p, newBatchNo("R"), 3)
	return success, failed, nil
}

// RevokeGrants 手工解除授权。
func (s *Service) RevokeGrants(ctx context.Context, grantIDs []uint64, p *principal.Info) (int, error) {
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.AccessGrant{})
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return 0, errs.ErrDataForbidden
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	if len(grantIDs) == 0 {
		return 0, errs.Invalid("请选择要解除的授权")
	}
	tx = tx.Where("id IN ? AND status IN ?", grantIDs, []int{GrantStatusPending, GrantStatusIssuing, GrantStatusSuccess, GrantStatusFailed})
	var grants []entity.AccessGrant
	if err := tx.Find(&grants).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	if len(grants) == 0 {
		return 0, errs.Invalid("没有可解除的授权记录")
	}
	if err := s.revokeGrantsInternal(ctx, &grants, GrantStatusRevoked, p); err != nil {
		return 0, err
	}
	return len(grants), nil
}

// IssuePersonAccess 手工触发某人员权限下发（按人员类型分派规则）。
func (s *Service) IssuePersonAccess(ctx context.Context, communityID uint64, personType int, personID uint64, p *principal.Info) (int, error) {
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return 0, err
	}
	switch personType {
	case PersonTypeResident:
		s.syncResidentAccess(ctx, communityID, personID, p)
	case PersonTypeVisitor:
		var v entity.Visitor
		if err := s.Repo.DB.WithContext(ctx).Where("id = ? AND community_id = ?", personID, communityID).First(&v).Error; err != nil {
			return 0, errs.ErrNotFound
		}
		s.syncVisitorAccess(ctx, &v, p)
	case PersonTypeEmployee:
		var e entity.Employee
		if err := s.Repo.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", personID, p.TenantID).First(&e).Error; err != nil {
			return 0, errs.ErrNotFound
		}
		s.syncEmployeeAccess(ctx, communityID, &e, p)
	case PersonTypeOther:
		var o entity.OtherPerson
		if err := s.Repo.DB.WithContext(ctx).Where("id = ? AND community_id = ?", personID, communityID).First(&o).Error; err != nil {
			return 0, errs.ErrNotFound
		}
		s.syncOtherPersonAccess(ctx, &o, p)
	default:
		return 0, errs.Invalid("未知的人员类型")
	}
	var n int64
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.AccessGrant{}).
		Where("community_id = ? AND person_type = ? AND person_id = ?", communityID, personType, personID).Count(&n).Error
	return int(n), nil
}

// ============================ 门禁卡 ============================

// AccessCardReq 门禁卡入参。
type AccessCardReq struct {
	CommunityID jsonx.Uint64 `json:"communityId" binding:"required"`
	CardNo      string       `json:"cardNo" binding:"required,max=64"`
	PersonType  int          `json:"personType" binding:"required"`
	PersonID    jsonx.Uint64 `json:"personId" binding:"required"`
	PersonName  string       `json:"personName" binding:"max=64"`
	Remark      string       `json:"remark" binding:"max=500"`
}

// CreateAccessCard 登记门禁卡（登记后会触发刷卡权限下发）。
func (s *Service) CreateAccessCard(ctx context.Context, req *AccessCardReq, p *principal.Info) (uint64, error) {
	if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
		return 0, err
	}
	com, err := s.findCommunity(ctx, p, req.CommunityID.Uint64())
	if err != nil {
		return 0, err
	}
	now := time.Now()
	c := &entity.AccessCard{
		Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:    com.TenantID,
		CommunityID: com.ID,
		CardNo:      strings.TrimSpace(req.CardNo),
		PersonType:  req.PersonType,
		PersonID:    req.PersonID.Uint64(),
		PersonName:  req.PersonName,
		Status:      1,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(c).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	// 触发下发
	s.IssuePersonAccess(ctx, com.ID, req.PersonType, req.PersonID.Uint64(), p)
	return c.ID, nil
}

// DeleteAccessCard 注销门禁卡。
func (s *Service) DeleteAccessCard(ctx context.Context, cardID uint64, p *principal.Info) error {
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.AccessCard{}).Where("id = ?", cardID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return errs.ErrNotFound
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	var c entity.AccessCard
	if err := tx.First(&c).Error; err != nil {
		return errs.ErrNotFound
	}
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.AccessCard{}, c.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	// 撤销该卡对应授权
	var grants []entity.AccessGrant
	_ = s.Repo.DB.WithContext(ctx).Where("community_id = ? AND person_type = ? AND person_id = ? AND grant_type = ? AND card_no = ? AND status IN ?",
		c.CommunityID, c.PersonType, c.PersonID, GrantTypeCard, c.CardNo,
		[]int{GrantStatusPending, GrantStatusIssuing, GrantStatusSuccess, GrantStatusFailed}).Find(&grants).Error
	if len(grants) > 0 {
		_ = s.revokeGrantsInternal(ctx, &grants, GrantStatusRevoked, p)
	}
	return nil
}

// ListAccessCards 门禁卡列表。
func (s *Service) ListAccessCards(ctx context.Context, communityID uint64, personType *int, p *principal.Info) ([]entity.AccessCard, error) {
	if communityID != 0 {
		if err := s.MustCommunity(ctx, p, communityID); err != nil {
			return nil, err
		}
	}
	tx := s.scoped(ctx, &entity.AccessCard{}, p, "")
	if communityID != 0 {
		tx = tx.Where("community_id = ?", communityID)
	}
	if personType != nil {
		tx = tx.Where("person_type = ?", *personType)
	}
	var list []entity.AccessCard
	if err := tx.Order("id DESC").Limit(500).Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// ============================ 定时任务 ============================

// ExpireAccessGrantsJob 到期授权自动解除。
func (s *Service) ExpireAccessGrantsJob(ctx context.Context, _ map[string]any) (string, error) {
	n, err := s.ExpireAccessGrants(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("解除到期门禁授权 %d 条", n), nil
}

// ExpireAccessGrants 把已过期的授权置为"过期解除"并调用第三方解除。
func (s *Service) ExpireAccessGrants(ctx context.Context) (int, error) {
	var grants []entity.AccessGrant
	if err := s.Repo.DB.WithContext(ctx).
		Where("status IN ? AND expire_time IS NOT NULL AND expire_time < ?",
			[]int{GrantStatusPending, GrantStatusIssuing, GrantStatusSuccess, GrantStatusFailed}, time.Now()).
		Limit(2000).Find(&grants).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	if len(grants) == 0 {
		return 0, nil
	}
	operator := &principal.Info{UserID: 0, Username: "system", Nickname: "系统任务"}
	if err := s.revokeGrantsInternal(ctx, &grants, GrantStatusExpired, operator); err != nil {
		return 0, err
	}
	return len(grants), nil
}

// houseNameOf 房屋全称（找不到返回空串）。
func (s *Service) houseNameOf(ctx context.Context, houseID uint64) string {
	if houseID == 0 {
		return ""
	}
	m := s.houseNameMap(ctx, []uint64{houseID})
	return m[houseID]
}

// truncateStr 截断字符串（数据库列长度保护）。
func truncateStr(v string, n int) string {
	if len(v) <= n {
		return v
	}
	return v[:n]
}
