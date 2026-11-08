// 车辆管理：车位 / 业主车、工作车、临时车 / 车辆进出记录。
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
	"web-basic/pkg/jsonx"
)

// ============================ 车位 ============================

// ParkingSpaceQuery 车位查询。
type ParkingSpaceQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	Keyword     string       `form:"keyword"`
	SpaceType   *int         `form:"spaceType"`
	Status      *int         `form:"status"`
}

// ListParkingSpaces 车位分页列表。
func (s *Service) ListParkingSpaces(ctx context.Context, q *ParkingSpaceQuery, p *principal.Info) ([]entity.ParkingSpace, int64, error) {
	q.Normalize()
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
	}
	tx := s.scoped(ctx, &entity.ParkingSpace{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(code LIKE ? OR space_no LIKE ? OR owner_name LIKE ?)", kw+"%", kw+"%", kw+"%")
	}
	if q.SpaceType != nil {
		tx = tx.Where("space_type = ?", *q.SpaceType)
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.ParkingSpace, 0, q.PageSize)
	if err := tx.Order("code ASC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// ParkingSpaceReq 车位入参。
type ParkingSpaceReq struct {
	CommunityID jsonx.Uint64 `json:"communityId" binding:"required"`
	RegionID    jsonx.Uint64 `json:"regionId"`
	Code        string       `json:"code" binding:"required,max=64"`
	SpaceNo     string       `json:"spaceNo" binding:"max=64"`
	SpaceType   int          `json:"spaceType"`
	Area        float64      `json:"area"`
	IsSale      bool         `json:"isSale"`
	FeeStandard float64      `json:"feeStandard"`
	Status      int          `json:"status"`
	Remark      string       `json:"remark" binding:"max=500"`
}

// CreateParkingSpace 新增车位。
func (s *Service) CreateParkingSpace(ctx context.Context, req *ParkingSpaceReq, p *principal.Info) (uint64, error) {
	if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
		return 0, err
	}
	com, err := s.findCommunity(ctx, p, req.CommunityID.Uint64())
	if err != nil {
		return 0, err
	}
	now := time.Now()
	ps := &entity.ParkingSpace{
		Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:    com.TenantID,
		CommunityID: com.ID,
		RegionID:    req.RegionID.Uint64(),
		Code:        strings.TrimSpace(req.Code),
		SpaceNo:     req.SpaceNo,
		SpaceType:   pmDefault(req.SpaceType, 1),
		Area:        req.Area,
		IsSale:      req.IsSale,
		FeeStandard: req.FeeStandard,
		Status:      pmDefault(req.Status, 1),
	}
	if err := s.Repo.DB.WithContext(ctx).Create(ps).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	return ps.ID, nil
}

// UpdateParkingSpace 修改车位。
func (s *Service) UpdateParkingSpace(ctx context.Context, spaceID uint64, req *ParkingSpaceReq, p *principal.Info) error {
	ps, err := s.parkingSpaceScoped(ctx, p, spaceID)
	if err != nil {
		return err
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.ParkingSpace{}).Where("id = ?", ps.ID).Updates(map[string]any{
		"region_id":    req.RegionID.Uint64(),
		"code":         strings.TrimSpace(req.Code),
		"space_no":     req.SpaceNo,
		"space_type":   pmDefault(req.SpaceType, 1),
		"area":         req.Area,
		"is_sale":      req.IsSale,
		"fee_standard": req.FeeStandard,
		"status":       pmDefault(req.Status, 1),
		"remark":       req.Remark,
		"update_by":    p.UserID,
		"update_time":  time.Now(),
	}).Error; err != nil {
		return wrapTxErr(err)
	}
	return nil
}

// DeleteParkingSpace 删除车位（已被车辆占用时拒绝）。
func (s *Service) DeleteParkingSpace(ctx context.Context, spaceID uint64, p *principal.Info) error {
	ps, err := s.parkingSpaceScoped(ctx, p, spaceID)
	if err != nil {
		return err
	}
	var n int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Vehicle{}).Where("parking_space_id = ? AND status != 3", ps.ID).Count(&n).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if n > 0 {
		return errs.Invalid("该车位已被车辆绑定，不能删除")
	}
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.ParkingSpace{}, ps.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// parkingSpaceScoped 带作用域读取车位。
func (s *Service) parkingSpaceScoped(ctx context.Context, p *principal.Info, spaceID uint64) (*entity.ParkingSpace, error) {
	var ps entity.ParkingSpace
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.ParkingSpace{}).Where("id = ?", spaceID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return nil, errs.ErrNotFound
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	if err := tx.First(&ps).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &ps, nil
}

// ============================ 车辆 ============================

// VehicleQuery 车辆查询。
type VehicleQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	Keyword     string       `form:"keyword"`
	PlateNo     string       `form:"plateNo"`
	VehicleType *int         `form:"vehicleType"`
	Status      *int         `form:"status"`
	ResidentID  jsonx.Uint64 `form:"residentId"`
	HouseID     jsonx.Uint64 `form:"houseId"`
}

// ListVehicles 车辆分页列表。
func (s *Service) ListVehicles(ctx context.Context, q *VehicleQuery, p *principal.Info) ([]entity.Vehicle, int64, error) {
	q.Normalize()
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
	}
	tx := s.scoped(ctx, &entity.Vehicle{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.PlateNo != "" {
		tx = tx.Where("plate_no LIKE ?", likePrefix(strings.ToUpper(q.PlateNo)))
	}
	if q.Keyword != "" {
		kw := strings.ToUpper(strings.TrimSpace(q.Keyword))
		tx = tx.Where("(plate_no LIKE ? OR brand LIKE ? OR house_name LIKE ?)", kw+"%", kw+"%", kw+"%")
	}
	if q.VehicleType != nil {
		tx = tx.Where("vehicle_type = ?", *q.VehicleType)
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	if q.ResidentID.Uint64() != 0 {
		tx = tx.Where("resident_id = ?", q.ResidentID.Uint64())
	}
	if q.HouseID.Uint64() != 0 {
		tx = tx.Where("house_id = ?", q.HouseID.Uint64())
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Vehicle, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// VehicleReq 车辆入参。
type VehicleReq struct {
	CommunityID    jsonx.Uint64 `json:"communityId" binding:"required"`
	PlateNo        string       `json:"plateNo" binding:"required,max=20"`
	PlateColor     int          `json:"plateColor"`
	VehicleType    int          `json:"vehicleType"`
	OwnerType      int          `json:"ownerType"`
	ResidentID     jsonx.Uint64 `json:"residentId"`
	HouseID        jsonx.Uint64 `json:"houseId"`
	EmployeeID     jsonx.Uint64 `json:"employeeId"`
	MerchantID     jsonx.Uint64 `json:"merchantId"`
	ParkingSpaceID jsonx.Uint64 `json:"parkingSpaceId"`
	Brand          string       `json:"brand" binding:"max=64"`
	Model          string       `json:"model" binding:"max=64"`
	Color          string       `json:"color" binding:"max=32"`
	EnergyType     int          `json:"energyType"`
	ValidFrom      string       `json:"validFrom"`
	ValidTo        string       `json:"validTo"`
	Status         int          `json:"status"`
	Remark         string       `json:"remark" binding:"max=500"`
}

// normalizePlate 车牌统一大写去空格。
func normalizePlate(v string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(v), " ", ""))
}

// CreateVehicle 登记车辆。
func (s *Service) CreateVehicle(ctx context.Context, req *VehicleReq, p *principal.Info) (uint64, error) {
	if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
		return 0, err
	}
	com, err := s.findCommunity(ctx, p, req.CommunityID.Uint64())
	if err != nil {
		return 0, err
	}
	plate := normalizePlate(req.PlateNo)
	if plate == "" {
		return 0, errs.Invalid("请填写车牌号")
	}
	var dup int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Vehicle{}).
		Where("community_id = ? AND plate_no = ? AND status != 3", com.ID, plate).Count(&dup).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	if dup > 0 {
		return 0, errs.Invalid("该车牌已在本小区登记")
	}
	var house *entity.House
	if req.HouseID.Uint64() != 0 {
		h, err := s.houseBelongsTo(ctx, com.ID, req.HouseID.Uint64())
		if err != nil {
			return 0, err
		}
		house = h
	}
	var space *entity.ParkingSpace
	if req.ParkingSpaceID.Uint64() != 0 {
		ps, err := s.parkingSpaceScoped(ctx, p, req.ParkingSpaceID.Uint64())
		if err != nil {
			return 0, err
		}
		space = ps
	}
	from, err := parseDate(req.ValidFrom)
	if err != nil {
		return 0, err
	}
	to, err := parseDate(req.ValidTo)
	if err != nil {
		return 0, err
	}
	vType := pmDefault(req.VehicleType, 1)
	if vType == 3 && to == nil {
		// 临时车默认 7 天有效
		t := today().AddDate(0, 0, 7)
		to = &t
	}
	now := time.Now()
	v := &entity.Vehicle{
		Base:           entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:       com.TenantID,
		CommunityID:    com.ID,
		PlateNo:        plate,
		PlateColor:     pmDefault(req.PlateColor, 1),
		VehicleType:    vType,
		OwnerType:      pmDefault(req.OwnerType, 1),
		ResidentID:     req.ResidentID.Uint64(),
		HouseID:        req.HouseID.Uint64(),
		EmployeeID:     req.EmployeeID.Uint64(),
		MerchantID:     req.MerchantID.Uint64(),
		ParkingSpaceID: req.ParkingSpaceID.Uint64(),
		Brand:          req.Brand,
		Model:          req.Model,
		Color:          req.Color,
		EnergyType:     req.EnergyType,
		ValidFrom:      from,
		ValidTo:        to,
		Status:         pmDefault(req.Status, 1),
	}
	if house != nil {
		v.HouseName = house.FullName
	}
	if space != nil {
		v.ParkingSpaceNo = space.SpaceNo
		if space.SpaceNo == "" {
			v.ParkingSpaceNo = space.Code
		}
	}
	if err := s.Repo.DB.WithContext(ctx).Create(v).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	// 车位状态同步
	if space != nil {
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.ParkingSpace{}).Where("id = ?", space.ID).Updates(map[string]any{
			"status": 4, "owner_resident_id": req.ResidentID.Uint64(), "update_time": now,
		}).Error
	}
	return v.ID, nil
}

// UpdateVehicle 修改车辆。
func (s *Service) UpdateVehicle(ctx context.Context, vehicleID uint64, req *VehicleReq, p *principal.Info) error {
	v, err := s.vehicleScoped(ctx, p, vehicleID)
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
	fields := map[string]any{
		"plate_no":     normalizePlate(req.PlateNo),
		"plate_color":  pmDefault(req.PlateColor, 1),
		"vehicle_type": pmDefault(req.VehicleType, 1),
		"owner_type":   pmDefault(req.OwnerType, 1),
		"resident_id":  req.ResidentID.Uint64(),
		"house_id":     req.HouseID.Uint64(),
		"employee_id":  req.EmployeeID.Uint64(),
		"merchant_id":  req.MerchantID.Uint64(),
		"brand":        req.Brand,
		"model":        req.Model,
		"color":        req.Color,
		"energy_type":  req.EnergyType,
		"valid_from":   from,
		"valid_to":     to,
		"status":       pmDefault(req.Status, 1),
		"remark":       req.Remark,
		"update_by":    p.UserID,
		"update_time":  time.Now(),
	}
	if req.HouseID.Uint64() != 0 {
		if h, err := s.houseBelongsTo(ctx, v.CommunityID, req.HouseID.Uint64()); err == nil && h != nil {
			fields["house_name"] = h.FullName
		}
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Vehicle{}).Where("id = ?", v.ID).Updates(fields).Error; err != nil {
		return wrapTxErr(err)
	}
	return nil
}

// DeleteVehicle 删除车辆（软删除）并释放车位。
func (s *Service) DeleteVehicle(ctx context.Context, vehicleID uint64, p *principal.Info) error {
	v, err := s.vehicleScoped(ctx, p, vehicleID)
	if err != nil {
		return err
	}
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.Vehicle{}, v.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if v.ParkingSpaceID != 0 {
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.ParkingSpace{}).Where("id = ?", v.ParkingSpaceID).Updates(map[string]any{
			"status": 1, "owner_resident_id": 0, "update_time": time.Now(),
		}).Error
	}
	return nil
}

// vehicleScoped 带作用域读取车辆。
func (s *Service) vehicleScoped(ctx context.Context, p *principal.Info, vehicleID uint64) (*entity.Vehicle, error) {
	var v entity.Vehicle
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Vehicle{}).Where("id = ?", vehicleID)
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

// VehicleAccessLogQuery 进出记录查询（大表：必须带时间范围或车牌）。
type VehicleAccessLogQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	PlateNo     string       `form:"plateNo"`
	Direction   *int         `form:"direction"`
	StartDate   string       `form:"startDate"`
	EndDate     string       `form:"endDate"`
}

// ListVehicleAccessLogs 车辆进出记录（默认近 30 天，避免大表全扫）。
func (s *Service) ListVehicleAccessLogs(ctx context.Context, q *VehicleAccessLogQuery, p *principal.Info) ([]entity.VehicleAccessLog, int64, error) {
	q.Normalize()
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
	}
	tx := s.scoped(ctx, &entity.VehicleAccessLog{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	start := time.Now().AddDate(0, 0, -30)
	if q.StartDate != "" {
		if d, err := parseDate(q.StartDate); err == nil && d != nil {
			start = *d
		}
	}
	tx = tx.Where("pass_time >= ?", start)
	if q.EndDate != "" {
		if d, err := parseDate(q.EndDate); err == nil && d != nil {
			tx = tx.Where("pass_time <= ?", endOfDay(*d))
		}
	}
	if q.PlateNo != "" {
		tx = tx.Where("plate_no LIKE ?", likePrefix(normalizePlate(q.PlateNo)))
	}
	if q.Direction != nil {
		tx = tx.Where("direction = ?", *q.Direction)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.VehicleAccessLog, 0, q.PageSize)
	if err := tx.Order("pass_time DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// VehicleExpireJob 定时任务：临时车/工作车到期置为已过期。
func (s *Service) VehicleExpireJob(ctx context.Context, _ map[string]any) (string, error) {
	res := s.Repo.DB.WithContext(ctx).Model(&entity.Vehicle{}).
		Where("status = 1 AND valid_to IS NOT NULL AND valid_to < ?", today()).
		UpdateColumn("status", 3)
	if res.Error != nil {
		return "", errs.ErrDB.WithErr(res.Error)
	}
	return "处理到期车辆 " + itoa64(res.RowsAffected) + " 辆", nil
}

// itoa64 简易整数转字符串（避免引入 strconv 造成命名冲突）。
func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		return "-" + string(buf)
	}
	return string(buf)
}
