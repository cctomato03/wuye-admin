// 物业概览：小区维度的核心指标（房屋/住户/车辆/设备/收费/门禁）。
//
// 实现要点：全部用 GROUP BY 聚合查询（命中 (tenant_id, community_id, ...) 索引），
// 不做多表 JOIN 上的 COUNT(DISTINCT)，避免在万级小区下变成慢查询。
package service

import (
	"context"
	"time"

	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/pkg/errs"
)

// CommunityOverview 小区概览。
type CommunityOverview struct {
	CommunityID   uint64  `json:"communityId,string"`
	CommunityName string  `json:"communityName"`
	HouseTotal    int64   `json:"houseTotal"`
	HouseResident int64   `json:"houseResident"`
	ResidentTotal int64   `json:"residentTotal"` // 有效住户关系数
	MerchantTotal int64   `json:"merchantTotal"`
	VehicleTotal  int64   `json:"vehicleTotal"`
	ParkingTotal  int64   `json:"parkingTotal"`
	DeviceTotal   int64   `json:"deviceTotal"`
	DeviceOnline  int64   `json:"deviceOnline"`
	VisitorToday  int64   `json:"visitorToday"`
	VisitorInside int64   `json:"visitorInside"`
	OtherPerson   int64   `json:"otherPerson"`
	BillMonth     string  `json:"billMonth"`
	BillCount     int64   `json:"billCount"`
	Receivable    float64 `json:"receivable"`
	Received      float64 `json:"received"`
	Outstanding   float64 `json:"outstanding"`
	AccessSuccess int64   `json:"accessSuccess"`
	AccessFailed  int64   `json:"accessFailed"`
	AccessPending int64   `json:"accessPending"`
}

// FeeTrendItem 月度收费趋势。
type FeeTrendItem struct {
	BillMonth  string  `json:"billMonth"`
	Receivable float64 `json:"receivable"`
	Received   float64 `json:"received"`
	Count      int64   `json:"count"`
}

// PropertyOverview 物业概览数据。
type PropertyOverview struct {
	Community      *CommunityOverview `json:"community"`
	FeeTrend       []FeeTrendItem     `json:"feeTrend"`
	CommunityCount int64              `json:"communityCount"`
	TenantCount    int64              `json:"tenantCount"`
}

// PropertyDashboard 物业概览（communityID=0 时返回用户可见的第一个小区）。
func (s *Service) PropertyDashboard(ctx context.Context, communityID uint64, p *principal.Info) (*PropertyOverview, error) {
	vo := &PropertyOverview{}
	if communityID == 0 {
		all, ids := s.communityScope(p)
		if all {
			var c entity.Community
			if err := s.Repo.DB.WithContext(ctx).Model(&entity.Community{}).
				Where("status = 1").Order("sort ASC, id ASC").First(&c).Error; err == nil {
				communityID = c.ID
			}
		} else if len(ids) > 0 {
			communityID = ids[0]
		}
	}
	// 顶部统计（平台视角）
	cq := s.Repo.DB.WithContext(ctx).Model(&entity.Community{})
	if t := s.tenantIDOf(p); t != 0 {
		cq = cq.Where("tenant_id = ?", t)
	}
	_ = cq.Count(&vo.CommunityCount).Error
	if p.IsSuper {
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.Tenant{}).Count(&vo.TenantCount).Error
	}
	if communityID == 0 {
		return vo, nil
	}
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	com, err := s.findCommunity(ctx, p, communityID)
	if err != nil {
		return nil, err
	}
	month := time.Now().Format("200601")
	ov := &CommunityOverview{CommunityID: com.ID, CommunityName: com.Name, BillMonth: month}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.House{}).Where("community_id = ?", com.ID).Count(&ov.HouseTotal).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.House{}).Where("community_id = ? AND resident_count > 0", com.ID).Count(&ov.HouseResident).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).Where("community_id = ? AND status = 1", com.ID).Count(&ov.ResidentTotal).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Merchant{}).Where("community_id = ? AND status = 1", com.ID).Count(&ov.MerchantTotal).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Vehicle{}).Where("community_id = ? AND status = 1", com.ID).Count(&ov.VehicleTotal).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.ParkingSpace{}).Where("community_id = ?", com.ID).Count(&ov.ParkingTotal).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Device{}).Where("community_id = ?", com.ID).Count(&ov.DeviceTotal).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Device{}).Where("community_id = ? AND online = 1", com.ID).Count(&ov.DeviceOnline).Error
	t := today()
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Visitor{}).
		Where("community_id = ? AND create_time >= ?", com.ID, t).Count(&ov.VisitorToday).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Visitor{}).
		Where("community_id = ? AND status = 2", com.ID).Count(&ov.VisitorInside).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.OtherPerson{}).
		Where("community_id = ? AND status = 1", com.ID).Count(&ov.OtherPerson).Error

	// 本月收费
	var feeRow struct {
		Count      int64
		Receivable float64
		Received   float64
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Bill{}).
		Select("COUNT(*) AS count, IFNULL(SUM(amount - discount_amount + late_fee),0) AS receivable, IFNULL(SUM(paid_amount),0) AS received").
		Where("community_id = ? AND bill_month = ? AND status != ?", com.ID, month, BillStatusCancelled).
		Scan(&feeRow).Error
	ov.BillCount = feeRow.Count
	ov.Receivable = round2(feeRow.Receivable)
	ov.Received = round2(feeRow.Received)
	ov.Outstanding = round2(ov.Receivable - ov.Received)

	// 门禁授权概况
	var accessRows []struct {
		Status int
		Count  int64
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.AccessGrant{}).
		Select("status, COUNT(*) AS count").Where("community_id = ?", com.ID).
		Group("status").Scan(&accessRows).Error
	for _, r := range accessRows {
		switch r.Status {
		case GrantStatusSuccess:
			ov.AccessSuccess += r.Count
		case GrantStatusFailed:
			ov.AccessFailed += r.Count
		case GrantStatusPending, GrantStatusIssuing:
			ov.AccessPending += r.Count
		}
	}
	vo.Community = ov

	// 近 6 个月收费趋势（按账期聚合，走 idx_comm_month_status）
	var trend []struct {
		BillMonth  string
		Count      int64
		Receivable float64
		Received   float64
	}
	since := time.Now().AddDate(0, -5, 0).Format("200601")
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Bill{}).
		Select("bill_month, COUNT(*) AS count, IFNULL(SUM(amount - discount_amount + late_fee),0) AS receivable, IFNULL(SUM(paid_amount),0) AS received").
		Where("community_id = ? AND bill_month >= ? AND status != ?", com.ID, since, BillStatusCancelled).
		Group("bill_month").Order("bill_month ASC").Scan(&trend).Error
	for _, r := range trend {
		vo.FeeTrend = append(vo.FeeTrend, FeeTrendItem{BillMonth: r.BillMonth, Count: r.Count, Receivable: round2(r.Receivable), Received: round2(r.Received)})
	}
	if vo.FeeTrend == nil {
		vo.FeeTrend = []FeeTrendItem{}
	}
	return vo, nil
}

// CommunityStatsRow 平台视角的小区运营行。
type CommunityStatsRow struct {
	CommunityID   uint64  `json:"communityId,string"`
	CommunityName string  `json:"communityName"`
	TenantName    string  `json:"tenantName"`
	HouseTotal    int64   `json:"houseTotal"`
	ResidentTotal int64   `json:"residentTotal"`
	BillMonth     string  `json:"billMonth"`
	Receivable    float64 `json:"receivable"`
	Received      float64 `json:"received"`
}

// ListCommunityStats 平台/物业公司维度的小区运营概览（分页）。
func (s *Service) ListCommunityStats(ctx context.Context, p *principal.Info, page, pageSize int) ([]CommunityStatsRow, int64, error) {
	pq := entity.PageQuery{Page: page, PageSize: pageSize}
	pq.Normalize()
	tx := s.communityTableScope(ctx, p)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	var communities []entity.Community
	if err := tx.Order("id ASC").Offset(pq.Offset()).Limit(pq.PageSize).Find(&communities).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	tenantNames, _ := s.tenantNameMap(ctx)
	month := time.Now().Format("200601")
	out := make([]CommunityStatsRow, 0, len(communities))
	for _, c := range communities {
		row := CommunityStatsRow{CommunityID: c.ID, CommunityName: c.Name, TenantName: tenantNames[c.TenantID], BillMonth: month}
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.House{}).Where("community_id = ?", c.ID).Count(&row.HouseTotal).Error
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).Where("community_id = ? AND status = 1", c.ID).Count(&row.ResidentTotal).Error
		var fee struct {
			Receivable float64
			Received   float64
		}
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.Bill{}).
			Select("IFNULL(SUM(amount - discount_amount + late_fee),0) AS receivable, IFNULL(SUM(paid_amount),0) AS received").
			Where("community_id = ? AND bill_month = ? AND status != ?", c.ID, month, BillStatusCancelled).
			Scan(&fee).Error
		row.Receivable = round2(fee.Receivable)
		row.Received = round2(fee.Received)
		out = append(out, row)
	}
	return out, total, nil
}
