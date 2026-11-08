// 访客 / 其他人员（保洁、维修、施工等）/ 单位（办公、商铺）管理。
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

// ============================ 访客 ============================

// 访客状态
const (
	VisitorStatusWaiting  = 1
	VisitorStatusInside   = 2
	VisitorStatusLeft     = 3
	VisitorStatusExpired  = 4
	VisitorStatusCanceled = 5
)

// VisitorQuery 访客查询。
type VisitorQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	Keyword     string       `form:"keyword"`
	Name        string       `form:"name"`
	Phone       string       `form:"phone"`
	Status      *int         `form:"status"`
	HouseID     jsonx.Uint64 `form:"houseId"`
	StartDate   string       `form:"startDate"`
	EndDate     string       `form:"endDate"`
}

// ListVisitors 访客分页列表。
func (s *Service) ListVisitors(ctx context.Context, q *VisitorQuery, p *principal.Info) ([]entity.Visitor, int64, error) {
	q.Normalize()
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
	}
	tx := s.scoped(ctx, &entity.Visitor{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.Name != "" {
		tx = tx.Where("name LIKE ?", likePrefix(q.Name))
	}
	if q.Phone != "" {
		tx = tx.Where("phone LIKE ?", likePrefix(q.Phone))
	}
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(name LIKE ? OR phone LIKE ? OR visit_no = ?)", kw+"%", kw+"%", kw)
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	if q.HouseID.Uint64() != 0 {
		tx = tx.Where("visit_house_id = ?", q.HouseID.Uint64())
	}
	if q.StartDate != "" {
		if d, err := parseDate(q.StartDate); err == nil && d != nil {
			tx = tx.Where("create_time >= ?", *d)
		}
	}
	if q.EndDate != "" {
		if d, err := parseDate(q.EndDate); err == nil && d != nil {
			tx = tx.Where("create_time <= ?", endOfDay(*d))
		}
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Visitor, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	for i := range list {
		list[i].IDNo = mask.IDCard(list[i].IDNo)
	}
	return list, total, nil
}

// VisitorAccompanyReq 随行人员。
type VisitorAccompanyReq struct {
	Name     string `json:"name" binding:"max=64"`
	IDNo     string `json:"idNo" binding:"max=64"`
	Phone    string `json:"phone" binding:"max=20"`
	Relation string `json:"relation" binding:"max=32"`
}

// VisitorReq 访客登记入参。
type VisitorReq struct {
	CommunityID  jsonx.Uint64          `json:"communityId" binding:"required"`
	Name         string                `json:"name" binding:"required,max=64"`
	Phone        string                `json:"phone" binding:"max=20"`
	IDNo         string                `json:"idNo" binding:"max=64"`
	Company      string                `json:"company" binding:"max=100"`
	VisitType    int                   `json:"visitType"`
	VisitReason  string                `json:"visitReason" binding:"max=200"`
	VisitHouseID jsonx.Uint64          `json:"visitHouseId"`
	ResidentID   jsonx.Uint64          `json:"residentId"`
	VehicleNo    string                `json:"vehicleNo" binding:"max=20"`
	PeopleCount  int                   `json:"peopleCount"`
	StartTime    string                `json:"startTime"`
	EndTime      string                `json:"endTime"`
	FaceURL      string                `json:"faceUrl" binding:"max=500"`
	Accompanies  []VisitorAccompanyReq `json:"accompanies"`
	Remark       string                `json:"remark" binding:"max=500"`
}

// CreateVisitor 访客登记（登记即下发门禁权限，有效期 = 预约结束时间）。
func (s *Service) CreateVisitor(ctx context.Context, req *VisitorReq, p *principal.Info) (uint64, error) {
	if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
		return 0, err
	}
	com, err := s.findCommunity(ctx, p, req.CommunityID.Uint64())
	if err != nil {
		return 0, err
	}
	var house *entity.House
	var residentName string
	if req.VisitHouseID.Uint64() != 0 {
		h, err := s.houseBelongsTo(ctx, com.ID, req.VisitHouseID.Uint64())
		if err != nil {
			return 0, err
		}
		house = h
	}
	// 受访住户：默认取该房屋的主业主
	residentID := req.ResidentID.Uint64()
	if residentID == 0 && house != nil {
		var rel entity.ResidentHouse
		if err := s.Repo.DB.WithContext(ctx).Where("community_id = ? AND house_id = ? AND rel_role = 1 AND status = 1", com.ID, house.ID).
			First(&rel).Error; err == nil {
			residentID = rel.ResidentID
		}
	}
	if residentID != 0 {
		var r entity.Resident
		if err := s.Repo.DB.WithContext(ctx).Where("id = ?", residentID).First(&r).Error; err == nil {
			residentName = r.Name
		}
	}
	start, err := parseDateTime(req.StartTime)
	if err != nil {
		return 0, err
	}
	end, err := parseDateTime(req.EndTime)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	if start == nil {
		start = &now
	}
	if end == nil {
		t := start.Add(4 * time.Hour)
		end = &t
	}
	if end.Before(*start) {
		return 0, errs.Invalid("离开时间不能早于到访时间")
	}
	visitNo := "V" + now.Format("20060102150405") + id.NewString()[15:]
	v := &entity.Visitor{
		Base:           entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:       com.TenantID,
		CommunityID:    com.ID,
		VisitNo:        visitNo,
		Name:           strings.TrimSpace(req.Name),
		Phone:          req.Phone,
		IDNo:           strings.TrimSpace(req.IDNo),
		Company:        req.Company,
		VisitType:      pmDefault(req.VisitType, 1),
		VisitReason:    req.VisitReason,
		VisitHouseID:   req.VisitHouseID.Uint64(),
		VisitHouseName: "",
		ResidentID:     residentID,
		ResidentName:   residentName,
		VehicleNo:      req.VehicleNo,
		PeopleCount:    pmDefault(req.PeopleCount, 1),
		StartTime:      start,
		EndTime:        end,
		Status:         VisitorStatusWaiting,
		OperatorID:     p.UserID,
		OperatorName:   p.Nickname,
		FaceURL:        req.FaceURL,
	}
	if house != nil {
		v.VisitHouseName = house.FullName
	}
	if err := s.Repo.DB.WithContext(ctx).Create(v).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	// 随行人员
	if len(req.Accompanies) > 0 {
		rows := make([]*entity.VisitorAccompany, 0, len(req.Accompanies))
		for _, a := range req.Accompanies {
			if strings.TrimSpace(a.Name) == "" {
				continue
			}
			rows = append(rows, &entity.VisitorAccompany{
				Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now},
				TenantID:    com.TenantID,
				CommunityID: com.ID,
				VisitorID:   v.ID,
				Name:        a.Name,
				IDNo:        a.IDNo,
				Phone:       a.Phone,
				Relation:    a.Relation,
			})
		}
		if len(rows) > 0 {
			_ = s.Repo.DB.WithContext(ctx).CreateInBatches(rows, 200).Error
			if v.PeopleCount < len(rows)+1 {
				v.PeopleCount = len(rows) + 1
				_ = s.Repo.DB.WithContext(ctx).Model(&entity.Visitor{}).Where("id = ?", v.ID).
					UpdateColumn("people_count", v.PeopleCount).Error
			}
		}
	}
	s.syncVisitorAccess(ctx, v, p)
	return v.ID, nil
}

// VisitorDetail 访客详情（含随行人员）。
type VisitorDetail struct {
	entity.Visitor
	Accompanies []entity.VisitorAccompany `json:"accompanies"`
}

// GetVisitor 访客详情。
func (s *Service) GetVisitor(ctx context.Context, visitorID uint64, p *principal.Info) (*VisitorDetail, error) {
	v, err := s.visitorScoped(ctx, p, visitorID)
	if err != nil {
		return nil, err
	}
	vo := &VisitorDetail{Visitor: *v}
	_ = s.Repo.DB.WithContext(ctx).Where("visitor_id = ?", v.ID).Find(&vo.Accompanies).Error
	return vo, nil
}

// visitorScoped 带作用域读取访客。
func (s *Service) visitorScoped(ctx context.Context, p *principal.Info, visitorID uint64) (*entity.Visitor, error) {
	var v entity.Visitor
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Visitor{}).Where("id = ?", visitorID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return nil, errs.ErrNotFound
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	if err := tx.First(&v).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &v, nil
}

// ChangeVisitorStatus 访客状态流转：进入 / 离场 / 取消。
func (s *Service) ChangeVisitorStatus(ctx context.Context, visitorID uint64, action string, p *principal.Info) error {
	v, err := s.visitorScoped(ctx, p, visitorID)
	if err != nil {
		return err
	}
	now := time.Now()
	fields := map[string]any{"update_by": p.UserID, "update_time": now}
	switch action {
	case "enter":
		if v.Status != VisitorStatusWaiting {
			return errs.Invalid("只有「待到访」的访客可以登记进入")
		}
		fields["status"] = VisitorStatusInside
		fields["enter_time"] = now
	case "leave":
		if v.Status != VisitorStatusInside && v.Status != VisitorStatusWaiting {
			return errs.Invalid("该访客已离场或已失效")
		}
		fields["status"] = VisitorStatusLeft
		fields["leave_time"] = now
	case "cancel":
		if v.Status == VisitorStatusLeft || v.Status == VisitorStatusCanceled {
			return errs.Invalid("该访客已离场/取消")
		}
		fields["status"] = VisitorStatusCanceled
	default:
		return errs.Invalid("不支持的操作")
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Visitor{}).Where("id = ?", v.ID).Updates(fields).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	// 离场/取消 → 撤销门禁
	if action == "leave" || action == "cancel" {
		s.revokeVisitorAccess(ctx, v.CommunityID, v.ID, p)
	}
	return nil
}

// DeleteVisitor 删除访客登记（软删除 + 撤销权限）。
func (s *Service) DeleteVisitor(ctx context.Context, visitorID uint64, p *principal.Info) error {
	v, err := s.visitorScoped(ctx, p, visitorID)
	if err != nil {
		return err
	}
	s.revokeVisitorAccess(ctx, v.CommunityID, v.ID, p)
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.Visitor{}, v.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	_ = s.Repo.DB.WithContext(ctx).Where("visitor_id = ?", v.ID).Delete(&entity.VisitorAccompany{}).Error
	return nil
}

// VisitorExpireJob 定时任务：过期访客自动置为已过期并撤销门禁。
func (s *Service) VisitorExpireJob(ctx context.Context, _ map[string]any) (string, error) {
	var list []entity.Visitor
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Visitor{}).
		Where("status IN ? AND end_time IS NOT NULL AND end_time < ?",
			[]int{VisitorStatusWaiting, VisitorStatusInside}, time.Now()).
		Limit(1000).Find(&list).Error; err != nil {
		return "", errs.ErrDB.WithErr(err)
	}
	now := time.Now()
	for _, v := range list {
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.Visitor{}).Where("id = ?", v.ID).
			Updates(map[string]any{"status": VisitorStatusExpired, "leave_time": now, "update_time": now}).Error
		operator := &principal.Info{UserID: 0, Username: "system", Nickname: "系统任务"}
		s.revokeVisitorAccess(ctx, v.CommunityID, v.ID, operator)
	}
	return fmt.Sprintf("处理过期访客 %d 条", len(list)), nil
}

// ============================ 其他人员类别 ============================

// PersonCategoryReq 类别入参。
type PersonCategoryReq struct {
	CommunityID jsonx.Uint64 `json:"communityId"`
	Name        string       `json:"name" binding:"required,max=64"`
	Code        string       `json:"code" binding:"max=64"`
	NeedFace    bool         `json:"needFace"`
	NeedCard    bool         `json:"needCard"`
	NeedPhone   bool         `json:"needPhone"`
	DefaultDays int          `json:"defaultDays"`
	Sort        int          `json:"sort"`
	Status      int          `json:"status"`
	Remark      string       `json:"remark" binding:"max=500"`
}

// ListPersonCategories 类别列表（含公司级共享类别）。
func (s *Service) ListPersonCategories(ctx context.Context, communityID uint64, p *principal.Info) ([]entity.PersonCategory, error) {
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.PersonCategory{})
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if communityID != 0 {
		tx = tx.Where("community_id IN ?", []uint64{0, communityID})
	}
	var list []entity.PersonCategory
	if err := tx.Order("sort ASC, id ASC").Limit(200).Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// CreatePersonCategory 新增类别。
func (s *Service) CreatePersonCategory(ctx context.Context, req *PersonCategoryReq, p *principal.Info) (uint64, error) {
	cid := req.CommunityID.Uint64()
	if cid != 0 {
		if err := s.MustCommunity(ctx, p, cid); err != nil {
			return 0, err
		}
	}
	now := time.Now()
	c := &entity.PersonCategory{
		Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:    p.TenantID,
		CommunityID: cid,
		Name:        strings.TrimSpace(req.Name),
		Code:        req.Code,
		NeedFace:    req.NeedFace,
		NeedCard:    req.NeedCard,
		NeedPhone:   req.NeedPhone,
		DefaultDays: pmDefault(req.DefaultDays, 30),
		Sort:        req.Sort,
		Status:      pmDefault(req.Status, 1),
	}
	if err := s.Repo.DB.WithContext(ctx).Create(c).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	return c.ID, nil
}

// UpdatePersonCategory 修改类别。
func (s *Service) UpdatePersonCategory(ctx context.Context, categoryID uint64, req *PersonCategoryReq, p *principal.Info) error {
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.PersonCategory{}).Where("id = ?", categoryID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	var c entity.PersonCategory
	if err := tx.First(&c).Error; err != nil {
		return errs.ErrNotFound
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.PersonCategory{}).Where("id = ?", c.ID).Updates(map[string]any{
		"name":         strings.TrimSpace(req.Name),
		"code":         req.Code,
		"need_face":    req.NeedFace,
		"need_card":    req.NeedCard,
		"need_phone":   req.NeedPhone,
		"default_days": pmDefault(req.DefaultDays, 30),
		"sort":         req.Sort,
		"status":       pmDefault(req.Status, 1),
		"remark":       req.Remark,
		"update_by":    p.UserID,
		"update_time":  time.Now(),
	}).Error; err != nil {
		return wrapTxErr(err)
	}
	return nil
}

// DeletePersonCategory 删除类别（被引用时拒绝）。
func (s *Service) DeletePersonCategory(ctx context.Context, categoryID uint64, p *principal.Info) error {
	var n int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.OtherPerson{}).Where("category_id = ?", categoryID).Count(&n).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if n > 0 {
		return errs.Invalid("该类别已被人员使用，不能删除")
	}
	tx := s.Repo.DB.WithContext(ctx).Where("id = ?", categoryID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if err := tx.Delete(&entity.PersonCategory{}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// ============================ 其他人员 ============================

// OtherPersonQuery 其他人员查询。
type OtherPersonQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	CategoryID  jsonx.Uint64 `form:"categoryId"`
	Keyword     string       `form:"keyword"`
	Status      *int         `form:"status"`
	Expiring    *bool        `form:"expiring"`
}

// OtherPersonVO 其他人员出参（含设备 ID 列表）。
type OtherPersonVO struct {
	entity.OtherPerson
	CategoryName string            `json:"categoryName"`
	DeviceIDs    jsonx.Uint64Slice `json:"deviceIds"`
}

// ListOtherPeople 其他人员分页列表。
func (s *Service) ListOtherPeople(ctx context.Context, q *OtherPersonQuery, p *principal.Info) ([]OtherPersonVO, int64, error) {
	q.Normalize()
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
	}
	tx := s.scoped(ctx, &entity.OtherPerson{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.CategoryID.Uint64() != 0 {
		tx = tx.Where("category_id = ?", q.CategoryID.Uint64())
	}
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(name LIKE ? OR phone LIKE ? OR company_name LIKE ?)", kw+"%", kw+"%", kw+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	if q.Expiring != nil && *q.Expiring {
		tx = tx.Where("status = 1 AND valid_to IS NOT NULL AND valid_to <= ?", today().AddDate(0, 0, 7))
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.OtherPerson, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	out := make([]OtherPersonVO, 0, len(list))
	if len(list) == 0 {
		return out, total, nil
	}
	catIDs := make([]uint64, 0, len(list))
	personIDs := make([]uint64, 0, len(list))
	for _, o := range list {
		catIDs = append(catIDs, o.CategoryID)
		personIDs = append(personIDs, o.ID)
	}
	catNames := s.nameMap(ctx, &entity.PersonCategory{}, catIDs)
	devMap := map[uint64][]uint64{}
	var pds []entity.OtherPersonDevice
	if err := s.Repo.DB.WithContext(ctx).Where("person_id IN ?", personIDs).Find(&pds).Error; err == nil {
		for _, pd := range pds {
			devMap[pd.PersonID] = append(devMap[pd.PersonID], pd.DeviceID)
		}
	}
	for _, o := range list {
		out = append(out, OtherPersonVO{OtherPerson: o, CategoryName: catNames[o.CategoryID], DeviceIDs: devMap[o.ID]})
	}
	return out, total, nil
}

// OtherPersonReq 其他人员入参。
type OtherPersonReq struct {
	CommunityID jsonx.Uint64      `json:"communityId" binding:"required"`
	CategoryID  jsonx.Uint64      `json:"categoryId"`
	Name        string            `json:"name" binding:"required,max=64"`
	Phone       string            `json:"phone" binding:"max=20"`
	IDNo        string            `json:"idNo" binding:"max=64"`
	CompanyName string            `json:"companyName" binding:"max=100"`
	WorkContent string            `json:"workContent" binding:"max=200"`
	ValidFrom   string            `json:"validFrom"`
	ValidTo     string            `json:"validTo"`
	FaceURL     string            `json:"faceUrl" binding:"max=500"`
	CardNo      string            `json:"cardNo" binding:"max=64"`
	DeviceIDs   jsonx.Uint64Slice `json:"deviceIds"`
	Status      int               `json:"status"`
	Remark      string            `json:"remark" binding:"max=500"`
}

// CreateOtherPerson 登记其他人员（勾选门禁设备后立即下发）。
func (s *Service) CreateOtherPerson(ctx context.Context, req *OtherPersonReq, p *principal.Info) (uint64, error) {
	if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
		return 0, err
	}
	com, err := s.findCommunity(ctx, p, req.CommunityID.Uint64())
	if err != nil {
		return 0, err
	}
	from, err := parseDate(req.ValidFrom)
	if err != nil {
		return 0, err
	}
	to, err := parseDate(req.ValidTo)
	if err != nil {
		return 0, err
	}
	if to == nil {
		t := today().AddDate(0, 0, 30)
		to = &t
	}
	if from == nil {
		t := today()
		from = &t
	}
	if to.Before(*from) {
		return 0, errs.Invalid("有效期结束日期不能早于开始日期")
	}
	now := time.Now()
	o := &entity.OtherPerson{
		Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:    com.TenantID,
		CommunityID: com.ID,
		CategoryID:  req.CategoryID.Uint64(),
		Name:        strings.TrimSpace(req.Name),
		Phone:       req.Phone,
		IDNo:        strings.TrimSpace(req.IDNo),
		CompanyName: req.CompanyName,
		WorkContent: req.WorkContent,
		ValidFrom:   from,
		ValidTo:     to,
		FaceURL:     req.FaceURL,
		CardNo:      req.CardNo,
		Status:      pmDefault(req.Status, 1),
	}
	if err := s.Repo.DB.WithContext(ctx).Create(o).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	if err := s.replaceOtherPersonDevices(ctx, com.ID, o.ID, req.DeviceIDs.ToUint64(), p); err != nil {
		return 0, err
	}
	s.syncOtherPersonAccess(ctx, o, p)
	return o.ID, nil
}

// UpdateOtherPerson 修改其他人员。
func (s *Service) UpdateOtherPerson(ctx context.Context, personID uint64, req *OtherPersonReq, p *principal.Info) error {
	o, err := s.otherPersonScoped(ctx, p, personID)
	if err != nil {
		return err
	}
	from, err := parseDate(req.ValidFrom)
	if err != nil {
		return err
	}
	to, err := parseDate(req.ValidTo)
	if err != nil {
		return err
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.OtherPerson{}).Where("id = ?", o.ID).Updates(map[string]any{
		"category_id":  req.CategoryID.Uint64(),
		"name":         strings.TrimSpace(req.Name),
		"phone":        req.Phone,
		"id_no":        strings.TrimSpace(req.IDNo),
		"company_name": req.CompanyName,
		"work_content": req.WorkContent,
		"valid_from":   from,
		"valid_to":     to,
		"face_url":     req.FaceURL,
		"card_no":      req.CardNo,
		"status":       pmDefault(req.Status, 1),
		"remark":       req.Remark,
		"update_by":    p.UserID,
		"update_time":  time.Now(),
	}).Error; err != nil {
		return wrapTxErr(err)
	}
	if req.DeviceIDs != nil {
		if err := s.replaceOtherPersonDevices(ctx, o.CommunityID, o.ID, req.DeviceIDs.ToUint64(), p); err != nil {
			return err
		}
	}
	// 先撤销旧授权，再按最新设备/资料重新下发
	s.revokeOtherPersonAccess(ctx, o.CommunityID, o.ID, p)
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", o.ID).First(o).Error; err == nil {
		s.syncOtherPersonAccess(ctx, o, p)
	}
	return nil
}

// replaceOtherPersonDevices 重置人员绑定的门禁设备。
func (s *Service) replaceOtherPersonDevices(ctx context.Context, communityID, personID uint64, deviceIDs []uint64, p *principal.Info) error {
	if err := s.Repo.DB.WithContext(ctx).Where("person_id = ?", personID).Delete(&entity.OtherPersonDevice{}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if len(deviceIDs) == 0 {
		return nil
	}
	// 校验设备都属于该小区
	var valid []struct{ ID uint64 }
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Device{}).Select("id").
		Where("community_id = ? AND id IN ?", communityID, deviceIDs).Scan(&valid).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if len(valid) != len(deviceIDs) {
		return errs.Invalid("存在不属于当前小区的门禁设备")
	}
	now := time.Now()
	rows := make([]*entity.OtherPersonDevice, 0, len(deviceIDs))
	for _, did := range deviceIDs {
		rows = append(rows, &entity.OtherPersonDevice{
			Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now},
			TenantID:    p.TenantID,
			CommunityID: communityID,
			PersonID:    personID,
			DeviceID:    did,
		})
	}
	if err := s.Repo.DB.WithContext(ctx).CreateInBatches(rows, 200).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// otherPersonScoped 带作用域读取其他人员。
func (s *Service) otherPersonScoped(ctx context.Context, p *principal.Info, personID uint64) (*entity.OtherPerson, error) {
	var o entity.OtherPerson
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.OtherPerson{}).Where("id = ?", personID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return nil, errs.ErrNotFound
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	if err := tx.First(&o).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &o, nil
}

// DeleteOtherPerson 删除其他人员（撤销门禁）。
func (s *Service) DeleteOtherPerson(ctx context.Context, personID uint64, p *principal.Info) error {
	o, err := s.otherPersonScoped(ctx, p, personID)
	if err != nil {
		return err
	}
	s.revokeOtherPersonAccess(ctx, o.CommunityID, o.ID, p)
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.OtherPerson{}, o.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	_ = s.Repo.DB.WithContext(ctx).Where("person_id = ?", o.ID).Delete(&entity.OtherPersonDevice{}).Error
	return nil
}

// OtherPersonExpireJob 定时任务：其他人员有效期到期自动停用并解除门禁。
func (s *Service) OtherPersonExpireJob(ctx context.Context, _ map[string]any) (string, error) {
	var list []entity.OtherPerson
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.OtherPerson{}).
		Where("status = 1 AND valid_to IS NOT NULL AND valid_to < ?", today()).Limit(1000).Find(&list).Error; err != nil {
		return "", errs.ErrDB.WithErr(err)
	}
	operator := &principal.Info{UserID: 0, Username: "system", Nickname: "系统任务"}
	for _, o := range list {
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.OtherPerson{}).Where("id = ?", o.ID).
			Updates(map[string]any{"status": 3, "update_time": time.Now()}).Error
		s.revokeOtherPersonAccess(ctx, o.CommunityID, o.ID, operator)
	}
	return fmt.Sprintf("处理到期其他人员 %d 条", len(list)), nil
}

// ============================ 单位/商铺 ============================

// MerchantQuery 单位查询。
type MerchantQuery struct {
	entity.PageQuery
	CommunityID  jsonx.Uint64 `form:"communityId"`
	Keyword      string       `form:"keyword"`
	MerchantType *int         `form:"merchantType"`
	BuildingID   jsonx.Uint64 `form:"buildingId"`
	Status       *int         `form:"status"`
}

// ListMerchants 单位分页列表。
func (s *Service) ListMerchants(ctx context.Context, q *MerchantQuery, p *principal.Info) ([]entity.Merchant, int64, error) {
	q.Normalize()
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
	}
	tx := s.scoped(ctx, &entity.Merchant{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(name LIKE ? OR contact_name LIKE ? OR contact_phone LIKE ?)", kw+"%", kw+"%", kw+"%")
	}
	if q.MerchantType != nil {
		tx = tx.Where("merchant_type = ?", *q.MerchantType)
	}
	if q.BuildingID.Uint64() != 0 {
		tx = tx.Where("building_id = ?", q.BuildingID.Uint64())
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Merchant, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// MerchantReq 单位入参。
type MerchantReq struct {
	CommunityID     jsonx.Uint64 `json:"communityId" binding:"required"`
	Name            string       `json:"name" binding:"required,max=100"`
	MerchantType    int          `json:"merchantType"`
	BuildingID      jsonx.Uint64 `json:"buildingId"`
	HouseID         jsonx.Uint64 `json:"houseId"`
	ContactName     string       `json:"contactName" binding:"max=64"`
	ContactPhone    string       `json:"contactPhone" binding:"max=20"`
	IDNo            string       `json:"idNo" binding:"max=64"`
	Industry        string       `json:"industry" binding:"max=64"`
	BusinessLicense string       `json:"businessLicense" binding:"max=64"`
	Area            float64      `json:"area"`
	EmployeeCount   int          `json:"employeeCount"`
	StartDate       string       `json:"startDate"`
	EndDate         string       `json:"endDate"`
	Status          int          `json:"status"`
	Remark          string       `json:"remark" binding:"max=500"`
}

// CreateMerchant 新增单位。
func (s *Service) CreateMerchant(ctx context.Context, req *MerchantReq, p *principal.Info) (uint64, error) {
	if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
		return 0, err
	}
	com, err := s.findCommunity(ctx, p, req.CommunityID.Uint64())
	if err != nil {
		return 0, err
	}
	var house *entity.House
	if req.HouseID.Uint64() != 0 {
		h, err := s.houseBelongsTo(ctx, com.ID, req.HouseID.Uint64())
		if err != nil {
			return 0, err
		}
		house = h
	}
	if req.BuildingID.Uint64() != 0 {
		if _, err := s.buildingBelongsTo(ctx, com.ID, req.BuildingID.Uint64()); err != nil {
			return 0, err
		}
	}
	start, err := parseDate(req.StartDate)
	if err != nil {
		return 0, err
	}
	end, err := parseDate(req.EndDate)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	m := &entity.Merchant{
		Base:            entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:        com.TenantID,
		CommunityID:     com.ID,
		Name:            strings.TrimSpace(req.Name),
		MerchantType:    pmDefault(req.MerchantType, 1),
		BuildingID:      req.BuildingID.Uint64(),
		HouseID:         req.HouseID.Uint64(),
		ContactName:     req.ContactName,
		ContactPhone:    req.ContactPhone,
		IDNo:            req.IDNo,
		Industry:        req.Industry,
		BusinessLicense: req.BusinessLicense,
		Area:            req.Area,
		EmployeeCount:   req.EmployeeCount,
		StartDate:       start,
		EndDate:         end,
		Status:          pmDefault(req.Status, 1),
	}
	if house != nil {
		m.HouseName = house.FullName
		if m.Area == 0 {
			m.Area = house.AreaBuild
		}
		if m.BuildingID == 0 {
			m.BuildingID = house.BuildingID
		}
	}
	if err := s.Repo.DB.WithContext(ctx).Create(m).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	return m.ID, nil
}

// UpdateMerchant 修改单位。
func (s *Service) UpdateMerchant(ctx context.Context, merchantID uint64, req *MerchantReq, p *principal.Info) error {
	m, err := s.merchantScoped(ctx, p, merchantID)
	if err != nil {
		return err
	}
	var house *entity.House
	if req.HouseID.Uint64() != 0 {
		h, err := s.houseBelongsTo(ctx, m.CommunityID, req.HouseID.Uint64())
		if err != nil {
			return err
		}
		house = h
	}
	start, err := parseDate(req.StartDate)
	if err != nil {
		return err
	}
	end, err := parseDate(req.EndDate)
	if err != nil {
		return err
	}
	fields := map[string]any{
		"name":             strings.TrimSpace(req.Name),
		"merchant_type":    pmDefault(req.MerchantType, 1),
		"building_id":      req.BuildingID.Uint64(),
		"house_id":         req.HouseID.Uint64(),
		"contact_name":     req.ContactName,
		"contact_phone":    req.ContactPhone,
		"id_no":            req.IDNo,
		"industry":         req.Industry,
		"business_license": req.BusinessLicense,
		"area":             req.Area,
		"employee_count":   req.EmployeeCount,
		"start_date":       start,
		"end_date":         end,
		"status":           pmDefault(req.Status, 1),
		"remark":           req.Remark,
		"update_by":        p.UserID,
		"update_time":      time.Now(),
	}
	if house != nil {
		fields["house_name"] = house.FullName
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Merchant{}).Where("id = ?", m.ID).Updates(fields).Error; err != nil {
		return wrapTxErr(err)
	}
	return nil
}

// DeleteMerchant 删除单位。
func (s *Service) DeleteMerchant(ctx context.Context, merchantID uint64, p *principal.Info) error {
	m, err := s.merchantScoped(ctx, p, merchantID)
	if err != nil {
		return err
	}
	var n int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Bill{}).Where("merchant_id = ? AND status IN (1,2)", m.ID).Count(&n).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if n > 0 {
		return errs.Invalid("该单位还有未结清账单，不能删除")
	}
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.Merchant{}, m.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// merchantScoped 带作用域读取单位。
func (s *Service) merchantScoped(ctx context.Context, p *principal.Info, merchantID uint64) (*entity.Merchant, error) {
	var m entity.Merchant
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Merchant{}).Where("id = ?", merchantID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return nil, errs.ErrNotFound
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	if err := tx.First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &m, nil
}
