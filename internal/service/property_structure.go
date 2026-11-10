// 房屋类型（户型模板）维护。
//
// 层级的增删改查在 property_node.go，房屋在 property_node.go + property_space_import.go，
// 这里只保留与"房屋类型"相关的读写，避免职责分散。
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
	"web-basic/pkg/jsonx"
)

// ============================ 房屋类型 ============================

// HouseTypeReq 房屋类型入参。
type HouseTypeReq struct {
	CommunityID jsonx.Uint64 `json:"communityId"`
	Name        string       `json:"name" binding:"required,max=64"`
	Code        string       `json:"code" binding:"max=64"`
	Layout      string       `json:"layout" binding:"max=64"`
	UsageType   int          `json:"usageType"`
	Sort        int          `json:"sort"`
	Status      int          `json:"status"`
	Remark      string       `json:"remark" binding:"max=500"`
}

// ListHouseTypes 房屋类型列表。
func (s *Service) ListHouseTypes(ctx context.Context, communityID uint64, p *principal.Info) ([]entity.HouseType, error) {
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.HouseType{})
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if communityID != 0 {
		tx = tx.Where("community_id IN ?", []uint64{0, communityID})
	}
	var list []entity.HouseType
	if err := tx.Order("sort ASC, id ASC").Limit(500).Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// CreateHouseType 新增房屋类型。
func (s *Service) CreateHouseType(ctx context.Context, req *HouseTypeReq, p *principal.Info) (uint64, error) {
	cid := req.CommunityID.Uint64()
	tenantID := p.TenantID
	if cid != 0 {
		if err := s.MustCommunity(ctx, p, cid); err != nil {
			return 0, err
		}
		if com, err := s.findCommunity(ctx, p, cid); err == nil {
			tenantID = com.TenantID
		}
	}
	now := time.Now()
	t := &entity.HouseType{
		Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:    tenantID,
		CommunityID: cid,
		Name:        strings.TrimSpace(req.Name),
		Code:        req.Code,
		Layout:      req.Layout,
		UsageType:   pmDefault(req.UsageType, 1),
		Sort:        req.Sort,
		Status:      pmDefault(req.Status, 1),
	}
	if err := s.Repo.DB.WithContext(ctx).Create(t).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	return t.ID, nil
}

// UpdateHouseType 修改房屋类型。
func (s *Service) UpdateHouseType(ctx context.Context, typeID uint64, req *HouseTypeReq, p *principal.Info) error {
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.HouseType{}).Where("id = ?", typeID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	var exists entity.HouseType
	if err := tx.First(&exists).Error; err != nil {
		return errs.ErrNotFound
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.HouseType{}).Where("id = ?", typeID).Updates(map[string]any{
		"name":        strings.TrimSpace(req.Name),
		"code":        req.Code,
		"layout":      req.Layout,
		"usage_type":  pmDefault(req.UsageType, 1),
		"sort":        req.Sort,
		"status":      pmDefault(req.Status, 1),
		"remark":      req.Remark,
		"update_by":   p.UserID,
		"update_time": time.Now(),
	}).Error; err != nil {
		return wrapTxErr(err)
	}
	return nil
}

// DeleteHouseType 删除房屋类型。
func (s *Service) DeleteHouseType(ctx context.Context, typeID uint64, p *principal.Info) error {
	var n int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.House{}).Where("house_type_id = ?", typeID).Count(&n).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if n > 0 {
		return errs.Invalid("该类型已被房屋使用，不能删除")
	}
	tx := s.Repo.DB.WithContext(ctx).Where("id = ?", typeID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if err := tx.Delete(&entity.HouseType{}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// ============================ Excel 导入 ============================

// ============================ 工具 ============================

// floorName 楼层号 → 名称（-1 → 负一层）。
func floorName(no int) string {
	if no < 0 {
		return fmt.Sprintf("负%d层", -no)
	}
	return fmt.Sprintf("%d层", no)
}

func defaultIfEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func toInt(v string, def int) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return def
	}
	n := 0
	neg := false
	for i, r := range v {
		if i == 0 && (r == '-' || r == '+') {
			neg = r == '-'
			continue
		}
		if r < '0' || r > '9' {
			return def
		}
		n = n*10 + int(r-'0')
	}
	if neg {
		return -n
	}
	return n
}

func toFloat(v string) float64 {
	v = strings.TrimSpace(strings.ReplaceAll(v, ",", ""))
	if v == "" {
		return 0
	}
	var out float64
	var frac float64 = 1
	dot := false
	for _, r := range v {
		if r == '.' {
			dot = true
			continue
		}
		if r < '0' || r > '9' {
			return 0
		}
		if dot {
			frac /= 10
			out += float64(r-'0') * frac
		} else {
			out = out*10 + float64(r-'0')
		}
	}
	return out
}

func parseUsageType(v string) int {
	switch strings.TrimSpace(v) {
	case "商业", "2":
		return 2
	case "办公", "3":
		return 3
	case "车位", "4":
		return 4
	case "其他", "5":
		return 5
	default:
		return 1
	}
}

func parseHouseStatus(v string) int {
	switch strings.TrimSpace(v) {
	case "自住", "2":
		return 2
	case "出租", "3":
		return 3
	case "装修中", "装修", "4":
		return 4
	case "停用", "5":
		return 5
	default:
		return 1
	}
}

func parseDecoration(v string) int {
	switch strings.TrimSpace(v) {
	case "毛坯", "1":
		return 1
	case "简装", "2":
		return 2
	case "精装", "3":
		return 3
	default:
		return 0
	}
}
