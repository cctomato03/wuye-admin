// 收费管理：收费项目 / 收费标准 / 水电表与抄表 / 账单生成 / 缴费 / 账户余额与押金。
//
// 计费模型（pm_fee_item.calc_mode）：
//
//	1 按面积·月   应收 = 建筑面积 × 单价
//	2 按面积·天   应收 = 建筑面积 × 单价 ÷ 当月天数 × 实际计费天数
//	3 按用量      应收 = (本期读数 − 上期读数) × 倍率 × 单价（抄表生成）
//	4 固定金额    应收 = 单价（按房屋/单位一笔）
//	5 按套        应收 = 单价 × 套数（默认 1）
//
// 大数据量处理：账单生成按小区+账期切片，批量 CreateInBatches(500)，
// 生成前先查已存在的 (fee_item_id, house_id, bill_month) 组合做幂等跳过。
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

// 计费方式
const (
	CalcModeAreaMonth = 1
	CalcModeAreaDay   = 2
	CalcModeUsage     = 3
	CalcModeFixed     = 4
	CalcModeUnit      = 5
)

// 账单状态
const (
	BillStatusUnpaid    = 1
	BillStatusPartial   = 2
	BillStatusPaid      = 3
	BillStatusReduced   = 4
	BillStatusCancelled = 5
)

// 支付方式
const (
	PayMethodBox       = 1 // 扫码盒子
	PayMethodMiniApp   = 2 // 小程序
	PayMethodCash      = 3 // 现金
	PayMethodOfficial  = 4 // 公众号
	PayMethodQRCode    = 5 // 二维码
	PayMethodBalance   = 6 // 余额划扣
	PayMethodOtherElec = 7 // 其他电子
	PayMethodDeposit   = 8 // 押金抵扣
	PayMethodBank      = 9 // 银行转账
)

// ============================ 收费项目 ============================

// FeeItemQuery 收费项目查询。
type FeeItemQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	Keyword     string       `form:"keyword"`
	ItemType    *int         `form:"itemType"`
	Status      *int         `form:"status"`
}

// ListFeeItems 收费项目列表。
func (s *Service) ListFeeItems(ctx context.Context, q *FeeItemQuery, p *principal.Info) ([]entity.FeeItem, int64, error) {
	q.Normalize()
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
	}
	tx := s.scoped(ctx, &entity.FeeItem{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(name LIKE ? OR code LIKE ?)", kw+"%", kw+"%")
	}
	if q.ItemType != nil {
		tx = tx.Where("item_type = ?", *q.ItemType)
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.FeeItem, 0, q.PageSize)
	if err := tx.Order("sort ASC, id ASC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// FeeItemReq 收费项目入参。
type FeeItemReq struct {
	CommunityID  jsonx.Uint64 `json:"communityId" binding:"required"`
	Code         string       `json:"code" binding:"required,max=64"`
	Name         string       `json:"name" binding:"required,max=100"`
	ItemType     int          `json:"itemType"`
	CalcMode     int          `json:"calcMode"`
	UnitPrice    float64      `json:"unitPrice"`
	Unit         string       `json:"unit" binding:"max=20"`
	BillingCycle int          `json:"billingCycle"`
	GenerateDay  int          `json:"generateDay"`
	HouseUsage   int          `json:"houseUsage"`
	ChargeObject int          `json:"chargeObject"`
	LateFeeRate  float64      `json:"lateFeeRate"`
	DueDays      int          `json:"dueDays"`
	Status       int          `json:"status"`
	Sort         int          `json:"sort"`
	Remark       string       `json:"remark" binding:"max=500"`
}

// CreateFeeItem 新增收费项目。
func (s *Service) CreateFeeItem(ctx context.Context, req *FeeItemReq, p *principal.Info) (uint64, error) {
	if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
		return 0, err
	}
	com, err := s.findCommunity(ctx, p, req.CommunityID.Uint64())
	if err != nil {
		return 0, err
	}
	if req.CalcMode != 0 && (req.CalcMode < 1 || req.CalcMode > 5) {
		return 0, errs.Invalid("计费方式取值应为 1~5")
	}
	now := time.Now()
	day := pmDefault(req.GenerateDay, 1)
	if day < 1 || day > 28 {
		day = 1
	}
	f := &entity.FeeItem{
		Base:         entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:     com.TenantID,
		CommunityID:  com.ID,
		Code:         strings.TrimSpace(req.Code),
		Name:         strings.TrimSpace(req.Name),
		ItemType:     pmDefault(req.ItemType, 1),
		CalcMode:     pmDefault(req.CalcMode, 1),
		UnitPrice:    req.UnitPrice,
		Unit:         req.Unit,
		BillingCycle: pmDefault(req.BillingCycle, 1),
		GenerateDay:  day,
		HouseUsage:   req.HouseUsage,
		ChargeObject: pmDefault(req.ChargeObject, 1),
		LateFeeRate:  req.LateFeeRate,
		DueDays:      pmDefault(req.DueDays, 30),
		Status:       pmDefault(req.Status, 1),
		Sort:         req.Sort,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(f).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	return f.ID, nil
}

// UpdateFeeItem 修改收费项目。
func (s *Service) UpdateFeeItem(ctx context.Context, itemID uint64, req *FeeItemReq, p *principal.Info) error {
	f, err := s.feeItemScoped(ctx, p, itemID)
	if err != nil {
		return err
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.FeeItem{}).Where("id = ?", f.ID).Updates(map[string]any{
		"code":          strings.TrimSpace(req.Code),
		"name":          strings.TrimSpace(req.Name),
		"item_type":     pmDefault(req.ItemType, 1),
		"calc_mode":     pmDefault(req.CalcMode, 1),
		"unit_price":    req.UnitPrice,
		"unit":          req.Unit,
		"billing_cycle": pmDefault(req.BillingCycle, 1),
		"generate_day":  pmDefault(req.GenerateDay, 1),
		"house_usage":   req.HouseUsage,
		"charge_object": pmDefault(req.ChargeObject, 1),
		"late_fee_rate": req.LateFeeRate,
		"due_days":      pmDefault(req.DueDays, 30),
		"status":        pmDefault(req.Status, 1),
		"sort":          req.Sort,
		"remark":        req.Remark,
		"update_by":     p.UserID,
		"update_time":   time.Now(),
	}).Error; err != nil {
		return wrapTxErr(err)
	}
	return nil
}

// DeleteFeeItem 删除收费项目（已有账单时拒绝，避免账单失去来源）。
func (s *Service) DeleteFeeItem(ctx context.Context, itemID uint64, p *principal.Info) error {
	f, err := s.feeItemScoped(ctx, p, itemID)
	if err != nil {
		return err
	}
	var n int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Bill{}).Where("fee_item_id = ?", f.ID).Count(&n).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if n > 0 {
		return errs.Invalid("该收费项目已产生账单，不能删除（可停用）")
	}
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.FeeItem{}, f.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// feeItemScoped 带作用域读取收费项目。
func (s *Service) feeItemScoped(ctx context.Context, p *principal.Info, itemID uint64) (*entity.FeeItem, error) {
	var f entity.FeeItem
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.FeeItem{}).Where("id = ?", itemID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return nil, errs.ErrNotFound
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	if err := tx.First(&f).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &f, nil
}

// ============================ 收费标准 ============================

// FeeStandardReq 收费标准入参。
type FeeStandardReq struct {
	FeeItemID     jsonx.Uint64 `json:"feeItemId" binding:"required"`
	HouseUsage    int          `json:"houseUsage"`
	HouseTypeID   jsonx.Uint64 `json:"houseTypeId"`
	BuildingID    jsonx.Uint64 `json:"buildingId"`
	UnitPrice     float64      `json:"unitPrice" binding:"required"`
	EffectiveFrom string       `json:"effectiveFrom"`
	EffectiveTo   string       `json:"effectiveTo"`
	Status        int          `json:"status"`
	Remark        string       `json:"remark" binding:"max=500"`
}

// ListFeeStandards 某收费项目的差异化单价。
func (s *Service) ListFeeStandards(ctx context.Context, itemID uint64, p *principal.Info) ([]entity.FeeStandard, error) {
	f, err := s.feeItemScoped(ctx, p, itemID)
	if err != nil {
		return nil, err
	}
	var list []entity.FeeStandard
	if err := s.Repo.DB.WithContext(ctx).Where("fee_item_id = ?", f.ID).Order("house_usage ASC, id ASC").Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// CreateFeeStandard 新增收费标准。
func (s *Service) CreateFeeStandard(ctx context.Context, req *FeeStandardReq, p *principal.Info) (uint64, error) {
	f, err := s.feeItemScoped(ctx, p, req.FeeItemID.Uint64())
	if err != nil {
		return 0, err
	}
	from, err := parseDate(req.EffectiveFrom)
	if err != nil {
		return 0, err
	}
	to, err := parseDate(req.EffectiveTo)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	std := &entity.FeeStandard{
		Base:          entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:      f.TenantID,
		CommunityID:   f.CommunityID,
		FeeItemID:     f.ID,
		HouseUsage:    req.HouseUsage,
		HouseTypeID:   req.HouseTypeID.Uint64(),
		BuildingID:    req.BuildingID.Uint64(),
		UnitPrice:     req.UnitPrice,
		EffectiveFrom: from,
		EffectiveTo:   to,
		Status:        pmDefault(req.Status, 1),
	}
	if err := s.Repo.DB.WithContext(ctx).Create(std).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	return std.ID, nil
}

// DeleteFeeStandard 删除收费标准。
func (s *Service) DeleteFeeStandard(ctx context.Context, standardID uint64, p *principal.Info) error {
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.FeeStandard{}).Where("id = ?", standardID)
	if !all {
		if len(ids) == 0 {
			return errs.ErrNotFound
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	var std entity.FeeStandard
	if err := tx.First(&std).Error; err != nil {
		return errs.ErrNotFound
	}
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.FeeStandard{}, std.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// resolveUnitPrice 解析房屋适用的单价：楼栋 > 房屋类型 > 用途 > 项目默认。
func (s *Service) resolveUnitPrice(ctx context.Context, item *entity.FeeItem, house *entity.House) float64 {
	if item.CalcMode == CalcModeUsage {
		return item.UnitPrice
	}
	var stds []entity.FeeStandard
	if err := s.Repo.DB.WithContext(ctx).Where("fee_item_id = ? AND status = 1", item.ID).Find(&stds).Error; err != nil {
		return item.UnitPrice
	}
	price := item.UnitPrice
	bestRank := 99
	for _, std := range stds {
		if std.HouseUsage != 0 && house != nil && std.HouseUsage != house.UsageType {
			continue
		}
		rank := 3
		switch {
		case std.BuildingID != 0 && house != nil && std.BuildingID == house.BuildingID:
			rank = 0
		case std.HouseTypeID != 0 && house != nil && std.HouseTypeID == house.HouseTypeID:
			rank = 1
		case std.HouseUsage != 0:
			rank = 2
		}
		if rank < bestRank {
			bestRank = rank
			price = std.UnitPrice
		}
	}
	return price
}

// ============================ 水电表与抄表 ============================

// MeterQuery 水表查询。
type MeterQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	Keyword     string       `form:"keyword"`
	MeterType   *int         `form:"meterType"`
	HouseID     jsonx.Uint64 `form:"houseId"`
}

// ListMeters 水电表列表。
func (s *Service) ListMeters(ctx context.Context, q *MeterQuery, p *principal.Info) ([]entity.Meter, int64, error) {
	q.Normalize()
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
	}
	tx := s.scoped(ctx, &entity.Meter{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(meter_no LIKE ? OR house_name LIKE ?)", kw+"%", kw+"%")
	}
	if q.MeterType != nil {
		tx = tx.Where("meter_type = ?", *q.MeterType)
	}
	if q.HouseID.Uint64() != 0 {
		tx = tx.Where("house_id = ?", q.HouseID.Uint64())
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Meter, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// MeterReq 水表入参。
type MeterReq struct {
	CommunityID jsonx.Uint64 `json:"communityId" binding:"required"`
	HouseID     jsonx.Uint64 `json:"houseId"`
	MerchantID  jsonx.Uint64 `json:"merchantId"`
	MeterType   int          `json:"meterType"`
	MeterNo     string       `json:"meterNo" binding:"required,max=64"`
	Ratio       float64      `json:"ratio"`
	InitReading float64      `json:"initReading"`
	InstallDate string       `json:"installDate"`
	Status      int          `json:"status"`
	Remark      string       `json:"remark" binding:"max=500"`
}

// CreateMeter 新增水电表。
func (s *Service) CreateMeter(ctx context.Context, req *MeterReq, p *principal.Info) (uint64, error) {
	if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
		return 0, err
	}
	com, err := s.findCommunity(ctx, p, req.CommunityID.Uint64())
	if err != nil {
		return 0, err
	}
	var houseName string
	if req.HouseID.Uint64() != 0 {
		h, err := s.houseBelongsTo(ctx, com.ID, req.HouseID.Uint64())
		if err != nil {
			return 0, err
		}
		houseName = h.FullName
	}
	install, err := parseDate(req.InstallDate)
	if err != nil {
		return 0, err
	}
	ratio := req.Ratio
	if ratio <= 0 {
		ratio = 1
	}
	now := time.Now()
	m := &entity.Meter{
		Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:    com.TenantID,
		CommunityID: com.ID,
		HouseID:     req.HouseID.Uint64(),
		HouseName:   houseName,
		MerchantID:  req.MerchantID.Uint64(),
		MeterType:   pmDefault(req.MeterType, 1),
		MeterNo:     strings.TrimSpace(req.MeterNo),
		Ratio:       ratio,
		InitReading: req.InitReading,
		LastReading: req.InitReading,
		InstallDate: install,
		Status:      pmDefault(req.Status, 1),
	}
	if err := s.Repo.DB.WithContext(ctx).Create(m).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	return m.ID, nil
}

// DeleteMeter 删除水电表。
func (s *Service) DeleteMeter(ctx context.Context, meterID uint64, p *principal.Info) error {
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Meter{}).Where("id = ?", meterID)
	if !all {
		if len(ids) == 0 {
			return errs.ErrNotFound
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	var m entity.Meter
	if err := tx.First(&m).Error; err != nil {
		return errs.ErrNotFound
	}
	var n int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.MeterReading{}).Where("meter_id = ?", m.ID).Count(&n).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if n > 0 {
		return errs.Invalid("该表已有抄表记录，不能删除")
	}
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.Meter{}, m.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// MeterReadingReq 抄表入参。
type MeterReadingReq struct {
	MeterID     jsonx.Uint64 `json:"meterId" binding:"required"`
	Period      string       `json:"period" binding:"required,len=6"`
	CurrReading float64      `json:"currReading" binding:"required"`
	UnitPrice   float64      `json:"unitPrice"`
	ReadTime    string       `json:"readTime"`
	Remark      string       `json:"remark" binding:"max=500"`
}

// CreateMeterReading 录入抄表（自动算用量与金额，并更新表底数）。
func (s *Service) CreateMeterReading(ctx context.Context, req *MeterReadingReq, p *principal.Info) (uint64, error) {
	var m entity.Meter
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Meter{}).Where("id = ?", req.MeterID.Uint64())
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return 0, errs.ErrNotFound
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	if err := tx.First(&m).Error; err != nil {
		return 0, errs.ErrNotFound
	}
	if _, _, err := monthRange(req.Period); err != nil {
		return 0, err
	}
	if req.CurrReading < m.LastReading {
		return 0, errs.Invalid(fmt.Sprintf("本期读数（%.2f）不能小于上期读数（%.2f）", req.CurrReading, m.LastReading))
	}
	var dup int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.MeterReading{}).
		Where("meter_id = ? AND period = ?", m.ID, req.Period).Count(&dup).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	if dup > 0 {
		return 0, errs.Invalid("该表在此账期已有抄表记录")
	}
	usage := (req.CurrReading - m.LastReading) * m.Ratio
	price := req.UnitPrice
	readTime, err := parseDateTime(req.ReadTime)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	if readTime == nil {
		readTime = &now
	}
	r := &entity.MeterReading{
		Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:    m.TenantID,
		CommunityID: m.CommunityID,
		MeterID:     m.ID,
		HouseID:     m.HouseID,
		HouseName:   m.HouseName,
		Period:      req.Period,
		PrevReading: m.LastReading,
		CurrReading: req.CurrReading,
		UsageAmount: usage,
		UnitPrice:   price,
		Amount:      round2(usage * price),
		ReadTime:    readTime,
		ReaderID:    p.UserID,
		ReaderName:  p.Nickname,
		Status:      1,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(r).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Meter{}).Where("id = ?", m.ID).Updates(map[string]any{
		"last_reading":   req.CurrReading,
		"last_read_time": now,
		"update_time":    now,
	}).Error
	return r.ID, nil
}

// MeterReadingQuery 抄表记录查询。
type MeterReadingQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	Period      string       `form:"period"`
	MeterID     jsonx.Uint64 `form:"meterId"`
	Status      *int         `form:"status"`
}

// ListMeterReadings 抄表记录列表。
func (s *Service) ListMeterReadings(ctx context.Context, q *MeterReadingQuery, p *principal.Info) ([]entity.MeterReading, int64, error) {
	q.Normalize()
	tx := s.scoped(ctx, &entity.MeterReading{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.Period != "" {
		tx = tx.Where("period = ?", q.Period)
	}
	if q.MeterID.Uint64() != 0 {
		tx = tx.Where("meter_id = ?", q.MeterID.Uint64())
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.MeterReading, 0, q.PageSize)
	if err := tx.Order("period DESC, id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// ============================ 账单生成 ============================

// GenerateBillReq 账单生成入参。
type GenerateBillReq struct {
	CommunityID jsonx.Uint64 `json:"communityId" binding:"required"`
	FeeItemID   jsonx.Uint64 `json:"feeItemId" binding:"required"`
	BillMonth   string       `json:"billMonth" binding:"required,len=6"`
	BuildingID  jsonx.Uint64 `json:"buildingId"`
	HouseUsage  *int         `json:"houseUsage"`
	DryRun      bool         `json:"dryRun"`
}

// GenerateBillResult 账单生成结果。
type GenerateBillResult struct {
	Created int     `json:"created"`
	Skipped int     `json:"skipped"`
	Amount  float64 `json:"amount"`
}

// GenerateBills 按收费项目生成某小区某账期的账单（幂等：已存在的跳过）。
//
// 处理流程（分批，避免大事务 + 内存爆）：
//  1. 按房屋用途筛选目标房屋（最多 5 万套，分批处理）
//  2. 逐户计算单价与应收（收费标准按 楼栋 > 房屋类型 > 用途 依次覆写）
//  3. 查该账期已有账单做幂等跳过
//  4. 批量写入（每批 500 条）
func (s *Service) GenerateBills(ctx context.Context, req *GenerateBillReq, p *principal.Info) (*GenerateBillResult, error) {
	if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
		return nil, err
	}
	item, err := s.feeItemScoped(ctx, p, req.FeeItemID.Uint64())
	if err != nil {
		return nil, err
	}
	if item.CalcMode == CalcModeUsage {
		return nil, errs.Invalid("该收费项目按用量计费，请在「抄表管理」中录入读数后生成账单")
	}
	start, end, err := monthRange(req.BillMonth)
	if err != nil {
		return nil, err
	}
	result := &GenerateBillResult{}

	if item.ChargeObject == 2 {
		return s.generateMerchantBills(ctx, item, req, start, end, p)
	}

	// 1) 目标房屋
	hq := s.Repo.DB.WithContext(ctx).Model(&entity.House{}).Where("community_id = ? AND deleted = 0 AND house_status != 5", item.CommunityID)
	if item.HouseUsage != 0 {
		if req.HouseUsage != nil {
			hq = hq.Where("usage_type = ?", *req.HouseUsage)
		} else {
			hq = hq.Where("usage_type = ?", item.HouseUsage)
		}
	} else if req.HouseUsage != nil {
		hq = hq.Where("usage_type = ?", *req.HouseUsage)
	}
	if req.BuildingID.Uint64() != 0 {
		hq = hq.Where("building_id = ?", req.BuildingID.Uint64())
	}
	var houses []entity.House
	if err := hq.Order("id ASC").Find(&houses).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if len(houses) == 0 {
		return result, nil
	}
	// 2) 已有账单（幂等）
	var existRows []string
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Bill{}).
		Select("CONCAT(fee_item_id, '-', house_id)").
		Where("community_id = ? AND fee_item_id = ? AND bill_month = ? AND status != ?",
			item.CommunityID, item.ID, req.BillMonth, BillStatusCancelled).
		Scan(&existRows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	exist := make(map[string]struct{}, len(existRows))
	for _, k := range existRows {
		exist[k] = struct{}{}
	}
	daysInMonth := end.Day()
	now := time.Now()
	// 账单冗余业主信息：便于按住户查询欠费、发催缴短信，也方便余额/押金划扣定位账户
	houseIDs := make([]uint64, 0, len(houses))
	for i := range houses {
		houseIDs = append(houseIDs, houses[i].ID)
	}
	owners := s.houseOwnerMap(ctx, houseIDs)
	bills := make([]*entity.Bill, 0, 512)
	dueDate := start.AddDate(0, 0, item.DueDays)
	for i := range houses {
		h := houses[i]
		key := fmt.Sprintf("%d-%d", item.ID, h.ID)
		if _, ok := exist[key]; ok {
			result.Skipped++
			continue
		}
		price := s.resolveUnitPrice(ctx, item, &h)
		var qty, amount float64
		switch item.CalcMode {
		case CalcModeAreaMonth:
			qty = h.AreaBuild
			amount = qty * price
		case CalcModeAreaDay:
			qty = h.AreaBuild
			amount = qty * price / float64(daysInMonth)
		case CalcModeFixed, CalcModeUnit:
			qty = 1
			amount = price
		default:
			qty = h.AreaBuild
			amount = qty * price
		}
		if amount <= 0 {
			result.Skipped++
			continue
		}
		bill := &entity.Bill{
			Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now},
			TenantID:    item.TenantID,
			CommunityID: item.CommunityID,
			BillNo:      newBillNo(req.BillMonth),
			FeeItemID:   item.ID,
			FeeItemName: item.Name,
			ItemType:    item.ItemType,
			HouseID:     h.ID,
			HouseName:   h.FullName,
			BuildingID:  h.BuildingID,
			HouseUsage:  h.UsageType,
			BillMonth:   req.BillMonth,
			PeriodStart: &start,
			PeriodEnd:   &end,
			Quantity:    round2(qty),
			UnitPrice:   price,
			Amount:      round2(amount),
			Status:      BillStatusUnpaid,
			DueDate:     &dueDate,
			Source:      1,
		}
		if owner, ok := owners[h.ID]; ok {
			bill.ResidentID = owner.id
			bill.ResidentName = owner.name
		}
		bills = append(bills, bill)
	}
	if req.DryRun {
		result.Created = len(bills)
		for _, b := range bills {
			result.Amount += b.Amount
		}
		return result, nil
	}
	if len(bills) > 0 {
		if err := s.Repo.DB.WithContext(ctx).CreateInBatches(bills, 500).Error; err != nil {
			return nil, wrapTxErr(err)
		}
		result.Created = len(bills)
		for _, b := range bills {
			result.Amount += b.Amount
		}
	}
	return result, nil
}

// generateMerchantBills 按单位（商铺/办公）生成账单。
func (s *Service) generateMerchantBills(ctx context.Context, item *entity.FeeItem, req *GenerateBillReq, start, end time.Time, p *principal.Info) (*GenerateBillResult, error) {
	result := &GenerateBillResult{}
	var merchants []entity.Merchant
	if err := s.Repo.DB.WithContext(ctx).Where("community_id = ? AND status = 1", item.CommunityID).
		Order("id ASC").Find(&merchants).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	var existRows []string
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Bill{}).
		Select("CONCAT(fee_item_id, '-', merchant_id)").
		Where("community_id = ? AND fee_item_id = ? AND bill_month = ? AND status != ?",
			item.CommunityID, item.ID, req.BillMonth, BillStatusCancelled).
		Scan(&existRows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	exist := make(map[string]struct{}, len(existRows))
	for _, k := range existRows {
		exist[k] = struct{}{}
	}
	now := time.Now()
	dueDate := start.AddDate(0, 0, item.DueDays)
	bills := make([]*entity.Bill, 0, len(merchants))
	for _, m := range merchants {
		key := fmt.Sprintf("%d-%d", item.ID, m.ID)
		if _, ok := exist[key]; ok {
			result.Skipped++
			continue
		}
		price := item.UnitPrice
		var qty, amount float64
		switch item.CalcMode {
		case CalcModeAreaMonth:
			qty = m.Area
			amount = qty * price
		case CalcModeAreaDay:
			qty = m.Area
			amount = qty * price / float64(end.Day())
		default:
			qty = 1
			amount = price
		}
		if amount <= 0 {
			result.Skipped++
			continue
		}
		bills = append(bills, &entity.Bill{
			Base:         entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now},
			TenantID:     item.TenantID,
			CommunityID:  item.CommunityID,
			BillNo:       newBillNo(req.BillMonth),
			FeeItemID:    item.ID,
			FeeItemName:  item.Name,
			ItemType:     item.ItemType,
			MerchantID:   m.ID,
			MerchantName: m.Name,
			HouseID:      m.HouseID,
			HouseName:    m.HouseName,
			BuildingID:   m.BuildingID,
			HouseUsage:   2,
			BillMonth:    req.BillMonth,
			PeriodStart:  &start,
			PeriodEnd:    &end,
			Quantity:     round2(qty),
			UnitPrice:    price,
			Amount:       round2(amount),
			Status:       BillStatusUnpaid,
			DueDate:      &dueDate,
			Source:       1,
		})
	}
	if req.DryRun {
		result.Created = len(bills)
		for _, b := range bills {
			result.Amount += b.Amount
		}
		return result, nil
	}
	if len(bills) > 0 {
		if err := s.Repo.DB.WithContext(ctx).CreateInBatches(bills, 500).Error; err != nil {
			return nil, wrapTxErr(err)
		}
	}
	result.Created = len(bills)
	for _, b := range bills {
		result.Amount += b.Amount
	}
	return result, nil
}

// GenerateMeterBillsReq 按抄表生成账单。
type GenerateMeterBillsReq struct {
	CommunityID jsonx.Uint64 `json:"communityId" binding:"required"`
	FeeItemID   jsonx.Uint64 `json:"feeItemId" binding:"required"`
	Period      string       `json:"period" binding:"required,len=6"`
}

// GenerateMeterBills 把未出账的抄表记录生成账单。
func (s *Service) GenerateMeterBills(ctx context.Context, req *GenerateMeterBillsReq, p *principal.Info) (*GenerateBillResult, error) {
	if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
		return nil, err
	}
	item, err := s.feeItemScoped(ctx, p, req.FeeItemID.Uint64())
	if err != nil {
		return nil, err
	}
	start, end, err := monthRange(req.Period)
	if err != nil {
		return nil, err
	}
	var readings []entity.MeterReading
	if err := s.Repo.DB.WithContext(ctx).Where("community_id = ? AND period = ? AND status = 1", item.CommunityID, req.Period).
		Order("id ASC").Find(&readings).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	result := &GenerateBillResult{}
	now := time.Now()
	dueDate := start.AddDate(0, 0, item.DueDays)
	readingHouseIDs := make([]uint64, 0, len(readings))
	for i := range readings {
		if readings[i].HouseID != 0 {
			readingHouseIDs = append(readingHouseIDs, readings[i].HouseID)
		}
	}
	owners := s.houseOwnerMap(ctx, readingHouseIDs)
	bills := make([]*entity.Bill, 0, len(readings))
	for i := range readings {
		r := readings[i]
		price := r.UnitPrice
		if price <= 0 {
			price = item.UnitPrice
		}
		amount := round2(r.UsageAmount * price)
		if amount <= 0 {
			result.Skipped++
			continue
		}
		b := &entity.Bill{
			Base:           entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now},
			TenantID:       item.TenantID,
			CommunityID:    item.CommunityID,
			BillNo:         newBillNo(req.Period),
			FeeItemID:      item.ID,
			FeeItemName:    item.Name,
			ItemType:       item.ItemType,
			HouseID:        r.HouseID,
			HouseName:      r.HouseName,
			HouseUsage:     1,
			BillMonth:      req.Period,
			PeriodStart:    &start,
			PeriodEnd:      &end,
			Quantity:       r.UsageAmount,
			UnitPrice:      price,
			Amount:         amount,
			Status:         BillStatusUnpaid,
			DueDate:        &dueDate,
			Source:         2,
			MeterReadingID: r.ID,
		}
		if owner, ok := owners[r.HouseID]; ok {
			b.ResidentID = owner.id
			b.ResidentName = owner.name
		}
		bills = append(bills, b)
	}
	if len(bills) > 0 {
		if err := s.Repo.DB.WithContext(ctx).CreateInBatches(bills, 500).Error; err != nil {
			return nil, wrapTxErr(err)
		}
		// 抄表记录标记已出账
		readingIDs := make([]uint64, 0, len(bills))
		for _, b := range bills {
			readingIDs = append(readingIDs, b.MeterReadingID)
			result.Amount += b.Amount
		}
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.MeterReading{}).Where("id IN ?", readingIDs).
			Updates(map[string]any{"status": 2}).Error
	}
	result.Created = len(bills)
	return result, nil
}

// ============================ 账单 ============================

// BillQuery 账单查询。
type BillQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	BillMonth   string       `form:"billMonth"`
	FeeItemID   jsonx.Uint64 `form:"feeItemId"`
	HouseID     jsonx.Uint64 `form:"houseId"`
	BuildingID  jsonx.Uint64 `form:"buildingId"`
	Keyword     string       `form:"keyword"`
	Status      *int         `form:"status"`
	ItemType    *int         `form:"itemType"`
	OnlyUnpaid  *bool        `form:"onlyUnpaid"`
}

// ListBills 账单分页列表。
func (s *Service) ListBills(ctx context.Context, q *BillQuery, p *principal.Info) ([]entity.Bill, int64, error) {
	q.Normalize()
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
	}
	tx := s.scoped(ctx, &entity.Bill{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.BillMonth != "" {
		tx = tx.Where("bill_month = ?", q.BillMonth)
	}
	if q.FeeItemID.Uint64() != 0 {
		tx = tx.Where("fee_item_id = ?", q.FeeItemID.Uint64())
	}
	if q.HouseID.Uint64() != 0 {
		tx = tx.Where("house_id = ?", q.HouseID.Uint64())
	}
	if q.BuildingID.Uint64() != 0 {
		tx = tx.Where("building_id = ?", q.BuildingID.Uint64())
	}
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(house_name LIKE ? OR bill_no LIKE ? OR resident_name LIKE ? OR merchant_name LIKE ?)", kw+"%", kw+"%", kw+"%", kw+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	if q.ItemType != nil {
		tx = tx.Where("item_type = ?", *q.ItemType)
	}
	if q.OnlyUnpaid != nil && *q.OnlyUnpaid {
		tx = tx.Where("status IN ?", []int{BillStatusUnpaid, BillStatusPartial})
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Bill, 0, q.PageSize)
	if err := tx.Order("bill_month DESC, id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// billScoped 带作用域读取账单。
func (s *Service) billScoped(ctx context.Context, p *principal.Info, billID uint64) (*entity.Bill, error) {
	var b entity.Bill
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Bill{}).Where("id = ?", billID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return nil, errs.ErrNotFound
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	if err := tx.First(&b).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &b, nil
}

// BillDetailVO 账单详情（含缴费流水）。
type BillDetailVO struct {
	entity.Bill
	Payments []entity.Payment `json:"payments"`
}

// GetBill 账单详情。
func (s *Service) GetBill(ctx context.Context, billID uint64, p *principal.Info) (*BillDetailVO, error) {
	b, err := s.billScoped(ctx, p, billID)
	if err != nil {
		return nil, err
	}
	vo := &BillDetailVO{Bill: *b}
	_ = s.Repo.DB.WithContext(ctx).Where("bill_id = ?", b.ID).Order("id DESC").Find(&vo.Payments).Error
	return vo, nil
}

// PayBillReq 收费入参。
type PayBillReq struct {
	PayMethod int     `json:"payMethod" binding:"required,oneof=1 2 3 4 5 6 7 8 9"`
	Amount    float64 `json:"amount"`
	TradeNo   string  `json:"tradeNo" binding:"max=100"`
	Remark    string  `json:"remark" binding:"max=500"`
}

// PayBillResult 收费结果。
type PayBillResult struct {
	PaymentID uint64  `json:"paymentId,string"`
	PayNo     string  `json:"payNo"`
	Paid      float64 `json:"paid"`
	Remaining float64 `json:"remaining"`
	Status    int     `json:"status"`
}

// PayBill 缴费（支持部分缴、余额划扣、押金抵扣）。
//
// 事务保证：账单已收金额、账单状态、缴费流水、账户余额与流水 全部一致。
func (s *Service) PayBill(ctx context.Context, billID uint64, req *PayBillReq, p *principal.Info) (*PayBillResult, error) {
	b, err := s.billScoped(ctx, p, billID)
	if err != nil {
		return nil, err
	}
	if b.Status == BillStatusPaid {
		return nil, errs.Invalid("该账单已结清")
	}
	if b.Status == BillStatusCancelled || b.Status == BillStatusReduced {
		return nil, errs.Invalid("该账单已作废或已减免，不能收费")
	}
	due := round2(b.Amount - b.DiscountAmount + b.LateFee - b.PaidAmount)
	if due <= 0 {
		return nil, errs.Invalid("该账单无需缴费")
	}
	amount := req.Amount
	if amount <= 0 || amount > due+0.001 {
		amount = due
	}
	amount = round2(amount)
	now := time.Now()
	payNo := "P" + now.Format("20060102150405") + id.NewString()[15:]
	res := &PayBillResult{PayNo: payNo}

	err = s.Repo.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 余额划扣 / 押金抵扣：先校验账户余额
		var account *entity.Account
		if req.PayMethod == PayMethodBalance || req.PayMethod == PayMethodDeposit {
			// 账单可能没有冗余住户（例如历史账单），按房屋当前业主兜底定位账户
			payerID := b.ResidentID
			if payerID == 0 && b.HouseID != 0 {
				if owner, ok := s.houseOwnerMap(ctx, []uint64{b.HouseID})[b.HouseID]; ok {
					payerID = owner.id
				}
			}
			var acc entity.Account
			q := tx.Where("community_id = ? AND owner_type = 1 AND resident_id = ?", b.CommunityID, payerID)
			if payerID == 0 || b.MerchantID != 0 {
				q = tx.Where("community_id = ? AND owner_type = 2 AND merchant_id = ?", b.CommunityID, b.MerchantID)
			}
			if err := q.First(&acc).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errs.Invalid("该住户/单位尚未开通账户，无法使用余额或押金抵扣")
				}
				return err
			}
			available := acc.Balance
			if req.PayMethod == PayMethodDeposit {
				available = acc.Deposit
			}
			if available < amount {
				return errs.Invalid(fmt.Sprintf("可用余额不足（当前 %.2f 元）", available))
			}
			account = &acc
		}
		payment := &entity.Payment{
			Base:         entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
			TenantID:     b.TenantID,
			CommunityID:  b.CommunityID,
			PayNo:        payNo,
			BillID:       b.ID,
			BillNo:       b.BillNo,
			HouseID:      b.HouseID,
			HouseName:    b.HouseName,
			ResidentID:   b.ResidentID,
			ResidentName: b.ResidentName,
			PayMethod:    req.PayMethod,
			Amount:       amount,
			Status:       1,
			PayTime:      &now,
			OperatorID:   p.UserID,
			OperatorName: p.Nickname,
			TradeNo:      req.TradeNo,
			Channel:      channelOf(req.PayMethod),
		}
		if err := tx.Create(payment).Error; err != nil {
			return err
		}
		res.PaymentID = payment.ID
		res.Paid = amount
		paidTotal := round2(b.PaidAmount + amount)
		status := BillStatusPartial
		remaining := round2(b.Amount - b.DiscountAmount + b.LateFee - paidTotal)
		if remaining <= 0.001 {
			status = BillStatusPaid
			remaining = 0
		}
		fields := map[string]any{
			"paid_amount": paidTotal,
			"status":      status,
			"update_by":   p.UserID,
			"update_time": now,
		}
		if status == BillStatusPaid {
			fields["pay_time"] = now
		}
		if err := tx.Model(&entity.Bill{}).Where("id = ?", b.ID).Updates(fields).Error; err != nil {
			return err
		}
		// 账户扣款
		if account != nil {
			target := 1
			after := round2(account.Balance - amount)
			updates := map[string]any{"balance": after, "update_time": now}
			if req.PayMethod == PayMethodDeposit {
				target = 2
				after = round2(account.Deposit - amount)
				updates = map[string]any{"deposit": after, "update_time": now}
			}
			if err := tx.Model(&entity.Account{}).Where("id = ?", account.ID).Updates(updates).Error; err != nil {
				return err
			}
			log := &entity.AccountLog{
				Base:         entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now},
				TenantID:     account.TenantID,
				CommunityID:  account.CommunityID,
				AccountID:    account.ID,
				ChangeType:   2,
				Amount:       -amount,
				BalanceAfter: after,
				TargetType:   target,
				BizType:      "bill",
				BizID:        b.ID,
			}
			if err := tx.Create(log).Error; err != nil {
				return err
			}
		}
		res.Remaining = remaining
		res.Status = status
		return nil
	})
	if err != nil {
		return nil, wrapTxErr(err)
	}
	return res, nil
}

// CancelBill 作废账单。
func (s *Service) CancelBill(ctx context.Context, billID uint64, p *principal.Info) error {
	b, err := s.billScoped(ctx, p, billID)
	if err != nil {
		return err
	}
	if b.PaidAmount > 0 {
		return errs.Invalid("该账单已有收款，请先退款后再作废")
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Bill{}).Where("id = ?", b.ID).Updates(map[string]any{
		"status": BillStatusCancelled, "update_by": p.UserID, "update_time": time.Now(),
	}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// BillSummary 收费汇总。
type BillSummary struct {
	BillMonth   string  `json:"billMonth"`
	Total       int64   `json:"total"`
	Unpaid      int64   `json:"unpaid"`
	Paid        int64   `json:"paid"`
	Receivable  float64 `json:"receivable"`
	Received    float64 `json:"received"`
	Outstanding float64 `json:"outstanding"`
}

// BillSummaryOf 某小区某账期的收费汇总（一次聚合查询）。
func (s *Service) BillSummaryOf(ctx context.Context, communityID uint64, billMonth string, p *principal.Info) (*BillSummary, error) {
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	var row struct {
		Total      int64
		Receivable float64
		Received   float64
	}
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Bill{}).
		Select("COUNT(*) AS total, IFNULL(SUM(amount - discount_amount + late_fee),0) AS receivable, IFNULL(SUM(paid_amount),0) AS received").
		Where("community_id = ? AND status != ?", communityID, BillStatusCancelled)
	if billMonth != "" {
		tx = tx.Where("bill_month = ?", billMonth)
	}
	if err := tx.Scan(&row).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	sum := &BillSummary{Total: row.Total, Receivable: round2(row.Receivable), Received: round2(row.Received), BillMonth: billMonth}
	sum.Outstanding = round2(sum.Receivable - sum.Received)
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Bill{}).
		Where("community_id = ? AND status IN ?", communityID, []int{BillStatusUnpaid, BillStatusPartial}).
		Count(&sum.Unpaid).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Bill{}).
		Where("community_id = ? AND status = ?", communityID, BillStatusPaid).Count(&sum.Paid).Error
	return sum, nil
}

// ============================ 缴费流水 ============================

// PaymentQuery 缴费记录查询。
type PaymentQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	Keyword     string       `form:"keyword"`
	PayMethod   *int         `form:"payMethod"`
	Status      *int         `form:"status"`
	StartDate   string       `form:"startDate"`
	EndDate     string       `form:"endDate"`
}

// ListPayments 缴费流水列表。
func (s *Service) ListPayments(ctx context.Context, q *PaymentQuery, p *principal.Info) ([]entity.Payment, int64, error) {
	q.Normalize()
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
	}
	tx := s.scoped(ctx, &entity.Payment{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(pay_no LIKE ? OR bill_no LIKE ? OR house_name LIKE ? OR resident_name LIKE ?)", kw+"%", kw+"%", kw+"%", kw+"%")
	}
	if q.PayMethod != nil {
		tx = tx.Where("pay_method = ?", *q.PayMethod)
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	if q.StartDate != "" {
		if d, err := parseDate(q.StartDate); err == nil && d != nil {
			tx = tx.Where("pay_time >= ?", *d)
		}
	}
	if q.EndDate != "" {
		if d, err := parseDate(q.EndDate); err == nil && d != nil {
			tx = tx.Where("pay_time <= ?", endOfDay(*d))
		}
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Payment, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// ============================ 账户 ============================

// RechargeReq 充值/押金缴纳入参。
type RechargeReq struct {
	CommunityID jsonx.Uint64 `json:"communityId" binding:"required"`
	OwnerType   int          `json:"ownerType"`
	ResidentID  jsonx.Uint64 `json:"residentId"`
	MerchantID  jsonx.Uint64 `json:"merchantId"`
	Amount      float64      `json:"amount" binding:"required"`
	TargetType  int          `json:"targetType"` // 1 余额 2 押金
	PayMethod   int          `json:"payMethod"`
	Remark      string       `json:"remark" binding:"max=500"`
}

// RechargeAccount 账户充值（余额）或缴纳押金。
func (s *Service) RechargeAccount(ctx context.Context, req *RechargeReq, p *principal.Info) (uint64, error) {
	if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
		return 0, err
	}
	if req.Amount <= 0 {
		return 0, errs.Invalid("金额必须大于 0")
	}
	com, err := s.findCommunity(ctx, p, req.CommunityID.Uint64())
	if err != nil {
		return 0, err
	}
	ownerType := pmDefault(req.OwnerType, 1)
	target := pmDefault(req.TargetType, 1)
	now := time.Now()
	var logID uint64
	err = s.Repo.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var acc entity.Account
		q := tx.Where("community_id = ? AND owner_type = ? AND resident_id = ?", com.ID, ownerType, req.ResidentID.Uint64())
		if ownerType == 2 {
			q = tx.Where("community_id = ? AND owner_type = ? AND merchant_id = ?", com.ID, ownerType, req.MerchantID.Uint64())
		}
		if err := q.First(&acc).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			ownerName := ""
			if ownerType == 1 && req.ResidentID.Uint64() != 0 {
				var r entity.Resident
				if err := tx.Where("id = ?", req.ResidentID.Uint64()).First(&r).Error; err == nil {
					ownerName = r.Name
				}
			}
			acc = entity.Account{
				Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now},
				TenantID:    com.TenantID,
				CommunityID: com.ID,
				OwnerType:   ownerType,
				ResidentID:  req.ResidentID.Uint64(),
				MerchantID:  req.MerchantID.Uint64(),
				OwnerName:   ownerName,
				Status:      1,
			}
			if err := tx.Create(&acc).Error; err != nil {
				return err
			}
		}
		after := round2(acc.Balance + req.Amount)
		updates := map[string]any{"balance": after, "update_time": now}
		if target == 2 {
			after = round2(acc.Deposit + req.Amount)
			updates = map[string]any{"deposit": after, "update_time": now}
		}
		if err := tx.Model(&entity.Account{}).Where("id = ?", acc.ID).Updates(updates).Error; err != nil {
			return err
		}
		log := &entity.AccountLog{
			Base:         entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, Remark: req.Remark},
			TenantID:     acc.TenantID,
			CommunityID:  acc.CommunityID,
			AccountID:    acc.ID,
			ChangeType:   1,
			Amount:       req.Amount,
			BalanceAfter: after,
			TargetType:   target,
			BizType:      "recharge",
		}
		if err := tx.Create(log).Error; err != nil {
			return err
		}
		logID = log.ID
		return nil
	})
	if err != nil {
		return 0, wrapTxErr(err)
	}
	return logID, nil
}

// ListAccounts 账户列表。
func (s *Service) ListAccounts(ctx context.Context, communityID uint64, keyword string, p *principal.Info) ([]entity.Account, error) {
	if communityID != 0 {
		if err := s.MustCommunity(ctx, p, communityID); err != nil {
			return nil, err
		}
	}
	tx := s.scoped(ctx, &entity.Account{}, p, "")
	if communityID != 0 {
		tx = tx.Where("community_id = ?", communityID)
	}
	if keyword != "" {
		tx = tx.Where("owner_name LIKE ?", likePrefix(keyword))
	}
	var list []entity.Account
	if err := tx.Order("id DESC").Limit(500).Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// AccountLogQuery 账户流水查询。
type AccountLogQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	AccountID   jsonx.Uint64 `form:"accountId"`
}

// ListAccountLogs 账户流水。
func (s *Service) ListAccountLogs(ctx context.Context, q *AccountLogQuery, p *principal.Info) ([]entity.AccountLog, int64, error) {
	q.Normalize()
	tx := s.scoped(ctx, &entity.AccountLog{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.AccountID.Uint64() != 0 {
		tx = tx.Where("account_id = ?", q.AccountID.Uint64())
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.AccountLog, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// ============================ 工具 ============================

// houseOwnerMap 批量取房屋的在册业主（业主角色 + 有效关系），用于账单冗余住户信息。
// 用分片 IN 查询代替逐户查询，1000 套房屋只打 1~2 次库。
func (s *Service) houseOwnerMap(ctx context.Context, houseIDs []uint64) map[uint64]struct {
	id   uint64
	name string
} {
	out := make(map[uint64]struct {
		id   uint64
		name string
	}, len(houseIDs))
	if len(houseIDs) == 0 {
		return out
	}
	_ = chunk(houseIDs, 1000, func(batch []uint64) error {
		var rows []struct {
			HouseID uint64
			ID      uint64
			Name    string
		}
		err := s.Repo.DB.WithContext(ctx).Table("pm_resident_house rh").
			Select("rh.house_id AS house_id, r.id AS id, r.name AS name").
			Joins("JOIN pm_resident r ON r.id = rh.resident_id AND r.deleted = 0").
			Where("rh.deleted = 0 AND rh.status = 1 AND rh.rel_role = 1 AND rh.house_id IN ?", batch).
			Order("rh.is_main DESC, rh.id ASC").Scan(&rows).Error
		if err != nil {
			return err
		}
		for _, r := range rows {
			if _, ok := out[r.HouseID]; ok {
				continue
			}
			out[r.HouseID] = struct {
				id   uint64
				name string
			}{id: r.ID, name: r.Name}
		}
		return nil
	})
	return out
}

// newBillNo 生成账单号：B + 账期 + 雪花后 10 位。
func newBillNo(period string) string {
	sn := id.NewString()
	if len(sn) > 10 {
		sn = sn[len(sn)-10:]
	}
	return "B" + period + sn
}

// channelOf 支付方式 → 渠道标识（预留第三方对接）。
func channelOf(method int) string {
	switch method {
	case PayMethodBox:
		return "pos_box"
	case PayMethodMiniApp:
		return "miniapp"
	case PayMethodCash:
		return "cash"
	case PayMethodOfficial:
		return "wechat_mp"
	case PayMethodQRCode:
		return "qrcode"
	case PayMethodBalance:
		return "balance"
	case PayMethodOtherElec:
		return "other_elec"
	case PayMethodDeposit:
		return "deposit"
	case PayMethodBank:
		return "bank"
	default:
		return "unknown"
	}
}

// round2 金额保留两位小数。
func round2(v float64) float64 {
	neg := v < 0
	if neg {
		v = -v
	}
	r := float64(int64(v*100+0.5)) / 100
	if neg {
		return -r
	}
	return r
}

// FeeBillJob 定时任务：按收费项目配置的生成日，为所有启用的物业费项目出账。
func (s *Service) FeeBillJob(ctx context.Context, params map[string]any) (string, error) {
	period := time.Now().Format("200601")
	if v, ok := params["period"].(string); ok && len(v) == 6 {
		period = v
	}
	var items []entity.FeeItem
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.FeeItem{}).
		Where("status = 1 AND calc_mode != ? AND (billing_cycle = 1 OR billing_cycle = 0)", CalcModeUsage).
		Limit(5000).Find(&items).Error; err != nil {
		return "", errs.ErrDB.WithErr(err)
	}
	operator := &principal.Info{UserID: 0, Username: "system", Nickname: "系统任务"}
	totalCreated, totalAmount := 0, 0.0
	for _, item := range items {
		res, err := s.GenerateBills(ctx, &GenerateBillReq{
			CommunityID: jsonx.Uint64(item.CommunityID),
			FeeItemID:   jsonx.Uint64(item.ID),
			BillMonth:   period,
		}, operator)
		if err != nil {
			continue
		}
		totalCreated += res.Created
		totalAmount += res.Amount
	}
	return fmt.Sprintf("账期 %s 生成账单 %d 条，合计 %.2f 元", period, totalCreated, totalAmount), nil
}
