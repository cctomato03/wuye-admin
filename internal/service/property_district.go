// 行政区划数据同步：省 / 市 / 区县 / 街道 / 社区 五级。
//
// 数据文件 data/districts.json.gz 由 scripts/build-district-data.mjs 生成（国家统计局数据，
// 编码为上下级前缀关系），随二进制内嵌，部署环境无需外网。
//
// 何时同步：
//   - 服务启动后若 pm_district 为空（首次部署）→ 后台异步导入，接口在此期间返回空/部分数据；
//   - 需要刷新数据时调用 POST /api/v1/property/districts/sync?force=true。
package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"web-basic/internal/logger"
	"web-basic/internal/model/entity"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
)

//go:embed data/districts.json.gz
var districtDataFS embed.FS

// districtSeedRow 数据集中的一行（c=编码 n=名称 l=层级）。
type districtSeedRow struct {
	Code  string `json:"c"`
	Name  string `json:"n"`
	Level int    `json:"l"`
}

// districtLevelNames 层级名称。
var districtLevelNames = map[int]string{1: "省", 2: "市", 3: "区县", 4: "街道", 5: "社区"}

// EnsureDistricts 首次启动时确保行政区划数据已导入（异步，不阻塞服务启动）。
func (s *Service) EnsureDistricts(ctx context.Context) {
	go func() {
		var n int64
		if err := s.Repo.DB.WithContext(context.WithoutCancel(ctx)).Model(&entity.District{}).Count(&n).Error; err != nil {
			logger.S().Warnw("check district count failed", "err", err)
			return
		}
		if n > 1000 {
			return // 已有全量数据
		}
		if _, err := s.SyncDistricts(context.WithoutCancel(ctx), true); err != nil {
			logger.S().Warnw("sync districts failed", "err", err)
		}
	}()
}

// SyncDistricts 导入/刷新行政区划；force=true 时先清空再导入。
// 用 MySQL 咨询锁保证同一时刻只有一个导入任务（启动自动导入与手工同步不会互相踩）。
func (s *Service) SyncDistricts(ctx context.Context, force bool) (int, error) {
	var got int
	if err := s.Repo.DB.WithContext(ctx).Raw("SELECT GET_LOCK('pm_district_sync', 5)").Scan(&got).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	if got != 1 {
		return 0, errs.Invalid("行政区划正在导入中，请稍后再试")
	}
	defer func() {
		_ = s.Repo.DB.WithContext(context.WithoutCancel(ctx)).Exec("SELECT RELEASE_LOCK('pm_district_sync')").Error
	}()
	return s.syncDistrictsLocked(ctx, force)
}

func (s *Service) syncDistrictsLocked(ctx context.Context, force bool) (int, error) {
	rows, err := loadDistrictSeed()
	if err != nil {
		return 0, errs.ErrInternal.WithErr(err)
	}
	if !force {
		var n int64
		if err := s.Repo.DB.WithContext(ctx).Model(&entity.District{}).Count(&n).Error; err != nil {
			return 0, errs.ErrDB.WithErr(err)
		}
		if n > 1000 {
			return 0, errs.Invalid("行政区划数据已存在，如需重新导入请使用强制同步")
		}
	}
	if force {
		if err := s.Repo.DB.WithContext(ctx).Exec("DELETE FROM pm_district").Error; err != nil {
			return 0, errs.ErrDB.WithErr(err)
		}
	}

	// 按层级排序后插入：父级一定先于子级，便于用 code→id 映射父级 ID
	// 数据集中 l 字段是编码长度（2/4/6/9/12），这里换算成 1~5 级。
	byLevel := make([][]districtSeedRow, 6)
	for _, r := range rows {
		if lv := levelOfCode(r.Code); lv >= 1 && lv <= 5 {
			byLevel[lv] = append(byLevel[lv], r)
		}
	}
	codeToID := make(map[string]uint64, len(rows))
	fullName := make(map[string]string, len(rows))
	now := time.Now()
	total := 0
	for level := 1; level <= 5; level++ {
		list := byLevel[level]
		batch := make([]*entity.District, 0, 2000)
		for i := range list {
			r := list[i]
			var parentID uint64
			var full string
			if level > 1 {
				parentCode := parentCodeOf(r.Code)
				parentID = codeToID[parentCode]
				full = strings.TrimSpace(fullName[parentCode] + r.Name)
			} else {
				full = r.Name
			}
			row := &entity.District{
				BaseLite: entity.BaseLite{ID: id.Next(), CreateTime: now},
				ParentID: parentID,
				Code:     r.Code,
				Name:     r.Name,
				Level:    level,
				FullName: full,
				Sort:     0,
				Status:   1,
			}
			codeToID[r.Code] = row.ID
			fullName[r.Code] = full
			batch = append(batch, row)
		}
		if len(batch) > 0 {
			if err := s.Repo.DB.WithContext(ctx).CreateInBatches(batch, 2000).Error; err != nil {
				return total, errs.ErrDB.WithErr(err)
			}
			total += len(batch)
		}
		logger.S().Infow("district sync progress", "level", level, "name", districtLevelNames[level], "inserted", len(batch), "total", total)
	}
	logger.S().Infow("district sync done", "total", total)
	return total, nil
}

// levelOfCode 由行政区划编码长度判断层级：2=省 4=市 6=区县 9=街道 12=社区。
func levelOfCode(code string) int {
	switch len(code) {
	case 2:
		return 1
	case 4:
		return 2
	case 6:
		return 3
	case 9:
		return 4
	case 12:
		return 5
	default:
		return 0
	}
}

// parentCodeOf 由子级编码推出父级编码：2/4/6/9/12 位 → 2/2/4/6/9 位前缀。
func parentCodeOf(code string) string {
	switch len(code) {
	case 4:
		return code[:2]
	case 6:
		return code[:4]
	case 9:
		return code[:6]
	case 12:
		return code[:9]
	default:
		return ""
	}
}

// loadDistrictSeed 读取内嵌数据集（gzip JSON）。
func loadDistrictSeed() ([]districtSeedRow, error) {
	f, err := districtDataFS.Open("data/districts.json.gz")
	if err != nil {
		return nil, fmt.Errorf("行政区划数据文件缺失，请先执行 node scripts/build-district-data.mjs：%w", err)
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	raw, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	var rows []districtSeedRow
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// ---------- 查询 ----------

// DistrictOption 行政区划下拉/级联节点。
type DistrictOption struct {
	ID          uint64 `json:"id,string"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
	HasChildren bool   `json:"hasChildren"`
}

// ListDistricts 按父级查询下一级行政区划（级联选择器懒加载）。
// parentID=0 返回省级；keyword 非空时按名称前缀搜索当前层级。
func (s *Service) ListDistricts(ctx context.Context, parentID uint64, level int, keyword string) ([]DistrictOption, error) {
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.District{}).Where("status = 1")
	if parentID != 0 {
		tx = tx.Where("parent_id = ?", parentID)
	} else if level > 0 {
		tx = tx.Where("level = ?", level)
	} else {
		tx = tx.Where("level = 1")
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		tx = tx.Where("name LIKE ?", kw+"%")
	}
	var list []entity.District
	if err := tx.Order("code ASC").Limit(5000).Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	out := make([]DistrictOption, 0, len(list))
	if len(list) == 0 {
		return out, nil
	}
	// 批量判断是否有下一级（只查一层，命中 idx_parent）
	ids := make([]uint64, 0, len(list))
	for _, d := range list {
		if d.Level < 5 {
			ids = append(ids, d.ID)
		}
	}
	hasChild := make(map[uint64]struct{}, len(ids))
	if len(ids) > 0 {
		for start := 0; start < len(ids); start += 1000 {
			end := start + 1000
			if end > len(ids) {
				end = len(ids)
			}
			var rows []struct{ ParentID uint64 }
			if err := s.Repo.DB.WithContext(ctx).Model(&entity.District{}).
				Select("DISTINCT parent_id").Where("parent_id IN ?", ids[start:end]).Scan(&rows).Error; err == nil {
				for _, r := range rows {
					hasChild[r.ParentID] = struct{}{}
				}
			}
		}
	}
	for _, d := range list {
		_, ok := hasChild[d.ID]
		out = append(out, DistrictOption{ID: d.ID, Code: d.Code, Name: d.Name, Level: d.Level, HasChildren: ok})
	}
	return out, nil
}

// DistrictPathItem 行政区划路径节点（回显用）。
type DistrictPathItem struct {
	ID    uint64 `json:"id,string"`
	Code  string `json:"code"`
	Name  string `json:"name"`
	Level int    `json:"level"`
}

// DistrictPath 按编码从省到社区还原路径（社区编码12位可推全部上级）。
func (s *Service) DistrictPath(ctx context.Context, code string) ([]DistrictPathItem, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, nil
	}
	codes := []string{}
	for _, n := range []int{2, 4, 6, 9, 12} {
		if len(code) >= n {
			codes = append(codes, code[:n])
		}
	}
	var list []entity.District
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.District{}).Where("code IN ?", codes).
		Order("level ASC").Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	out := make([]DistrictPathItem, 0, len(list))
	for _, d := range list {
		out = append(out, DistrictPathItem{ID: d.ID, Code: d.Code, Name: d.Name, Level: d.Level})
	}
	return out, nil
}
