// 小区基础信息（层级 + 房屋）的 Excel 模板、导入与导出。
//
// 一张表同时描述"层级"和"房屋"：层级路径用 / 分隔（如 A区/1栋/1单元/1层），
// 节点不存在自动创建；"空间类别/总层数"写到路径中的楼栋节点上。
// 房屋字段覆盖：用途、类型、面积（建筑/套内/公摊）、户型、朝向、装修、状态、产权、临街、层高、交付日期。
package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"web-basic/internal/excel"
	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/pkg/errs"
	"web-basic/pkg/jsonx"
)

// spaceImportHeaders 导入表头（顺序即列序）。
var spaceImportHeaders = []string{
	"层级路径", "空间类别", "总层数", "房屋编码", "房号", "房屋用途", "房屋类型",
	"建筑面积", "套内面积", "公摊面积", "户型", "朝向", "装修", "房屋状态",
	"产权性质", "是否临街", "层高", "交付日期", "备注",
}

// spaceExportHeaders 导出表头。
var spaceExportHeaders = []string{
	"层级路径", "空间类别", "房屋编码", "房号", "房屋用途", "房屋类型",
	"建筑面积", "套内面积", "公摊面积", "户型", "朝向", "装修", "房屋状态",
	"产权性质", "是否临街", "层高", "交付日期", "备注",
}

// SpaceImportTemplate 下载导入模板（住宅 / 别墅 / 商铺三种示例）。
func (s *Service) SpaceImportTemplate() ([]byte, error) {
	rows := [][]any{
		{"1栋/1单元/1层", "住宅楼", 18, "1-1-101", "101", "住宅", "三室两厅", 98.5, 82.3, 16.2, "3室2厅", "南", "精装", "空置", "商品房", "否", 2.9, "", "示例：普通住宅"},
		{"2栋/1单元/2层", "住宅楼", 18, "1-2-201", "201", "住宅", "两室一厅", 78.2, 65.1, 13.1, "2室1厅", "东南", "简装", "空置", "商品房", "否", 2.9, "", ""},
		{"别墅区/1号别墅", "别墅", 3, "V-001", "1号", "住宅", "独栋别墅", 320, 280, 0, "5室3厅", "南", "精装", "自住", "商品房", "否", 3.3, "", "示例：一栋一套"},
		{"商业楼/1层", "商业楼", 4, "S1-101", "101", "商业", "商铺", 120, 110, 0, "", "临街", "毛坯", "出租", "商品房", "是", 4.5, "", "示例：商铺"},
		{"办公楼/1层", "办公楼", 12, "O1-101", "101", "办公", "写字间", 150, 135, 15, "", "南", "简装", "出租", "商品房", "否", 3.6, "", "示例：办公楼"},
	}
	data, err := excel.Build("小区基础信息导入", spaceImportHeaders, rows)
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	return data, nil
}

// StructureImportResult 导入结果。
type StructureImportResult struct {
	Total     int      `json:"total"`
	Success   int      `json:"success"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors"`
	Buildings int      `json:"buildings"`
	Houses    int      `json:"houses"`
}

// ImportStructures 导入小区基础信息（层级不存在自动创建）。
func (s *Service) ImportStructures(ctx context.Context, communityID uint64, r io.Reader, p *principal.Info) (*StructureImportResult, error) {
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	if _, err := s.findCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	rows, err := excel.Read(r, 20000)
	if err != nil {
		return nil, errs.Invalid("文件解析失败：请确认上传的是 .xlsx 格式的 Excel 文件")
	}
	if len(rows) <= 1 {
		return nil, errs.Invalid("文件没有数据行")
	}
	res := &StructureImportResult{Errors: []string{}}
	cell := func(row []string, i int) string {
		if i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	typeNames := map[string]uint64{}
	failed := func(line int, msg string) {
		res.Failed++
		if len(res.Errors) < 200 {
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：%s", line, msg))
		}
	}
	for i, row := range rows[1:] {
		lineNo := i + 2
		res.Total++
		path := cell(row, 0)
		houseCode := cell(row, 3)
		if path == "" || houseCode == "" {
			failed(lineNo, "层级路径与房屋编码为必填")
			continue
		}
		category := parseSpaceCategory(cell(row, 1))
		floors := toInt(cell(row, 2), 0)
		node, err := s.ensureNodePathWith(ctx, communityID, path, category, floors, p)
		if err != nil {
			failed(lineNo, fmtErr(err))
			continue
		}
		var houseTypeID uint64
		if tn := cell(row, 6); tn != "" {
			if v, ok := typeNames[tn]; ok {
				houseTypeID = v
			} else {
				var exist entity.HouseType
				if err := s.Repo.DB.WithContext(ctx).Where("tenant_id = ? AND name = ? AND community_id IN ?", p.TenantID, tn, []uint64{0, communityID}).First(&exist).Error; err == nil {
					houseTypeID = exist.ID
				} else if newID, err := s.CreateHouseType(ctx, &HouseTypeReq{CommunityID: jsonx.Uint64(communityID), Name: tn, UsageType: parseUsageType(cell(row, 5))}, p); err == nil {
					houseTypeID = newID
				}
				typeNames[tn] = houseTypeID
			}
		}
		if _, err := s.CreateHouse(ctx, &HouseReq{
			CommunityID:   jsonx.Uint64(communityID),
			NodeID:        jsonx.Uint64(node.ID),
			HouseTypeID:   jsonx.Uint64(houseTypeID),
			Code:          houseCode,
			HouseNo:       defaultIfEmpty(cell(row, 4), houseCode),
			UsageType:     parseUsageType(cell(row, 5)),
			AreaBuild:     toFloat(cell(row, 7)),
			AreaUse:       toFloat(cell(row, 8)),
			AreaShare:     toFloat(cell(row, 9)),
			Layout:        cell(row, 10),
			Orientation:   cell(row, 11),
			Decoration:    parseDecoration(cell(row, 12)),
			HouseStatus:   parseHouseStatus(cell(row, 13)),
			PropertyRight: parsePropertyRight(cell(row, 14)),
			IsStreet:      isYes(cell(row, 15)),
			CeilingHeight: toFloat(cell(row, 16)),
			DeliveryDate:  cell(row, 17),
			Remark:        cell(row, 18),
		}, p); err != nil {
			failed(lineNo, fmtErr(err))
			continue
		}
		res.Success++
		res.Houses++
	}
	return res, nil
}

// ensureNodePathWith 按路径复用/创建节点；类别与层数写到"楼栋级"节点上。
func (s *Service) ensureNodePathWith(ctx context.Context, communityID uint64, path string, category, floors int, p *principal.Info) (*entity.StructureNode, error) {
	segments := []string{}
	for _, seg := range strings.Split(path, "/") {
		if strings.TrimSpace(seg) != "" {
			segments = append(segments, strings.TrimSpace(seg))
		}
	}
	if len(segments) == 0 {
		return nil, errs.Invalid("层级路径不合法")
	}
	// 类型按"从末级往前推"：最后=楼层，倒数第二=单元，倒数第三=楼栋，更上层=区域；
	// 只有 1~2 段时（别墅/独立商铺），最后一段就是楼栋。
	typeByIndex := make([]int, len(segments))
	if len(segments) <= 2 {
		for i := range typeByIndex {
			typeByIndex[i] = 1
		}
		typeByIndex[len(segments)-1] = 2
	} else {
		for i := range typeByIndex {
			typeByIndex[i] = 1
		}
		n := len(segments)
		typeByIndex[n-1] = 4
		typeByIndex[n-2] = 3
		typeByIndex[n-3] = 2
	}
	buildingIdx := -1
	for i, t := range typeByIndex {
		if t == 2 {
			buildingIdx = i
			break
		}
	}
	var parent *entity.StructureNode
	for idx, seg := range segments {
		var node entity.StructureNode
		q := s.Repo.DB.WithContext(ctx).Where("community_id = ? AND name = ?", communityID, seg)
		if parent == nil {
			q = q.Where("parent_id = 0")
		} else {
			q = q.Where("parent_id = ?", parent.ID)
		}
		if err := q.First(&node).Error; err == nil {
			parent = &node
			continue
		}
		parentID := jsonx.Uint64(0)
		if parent != nil {
			parentID = jsonx.Uint64(parent.ID)
		}
		req := &NodeReq{
			CommunityID: jsonx.Uint64(communityID),
			ParentID:    parentID,
			Name:        seg,
			NodeType:    typeByIndex[idx],
			Category:    9,
		}
		if idx == buildingIdx {
			req.Category = category
			req.TotalFloors = floors
			req.FloorsAbove = floors
		}
		newID, err := s.CreateNode(ctx, req, p)
		if err != nil {
			return nil, err
		}
		if err := s.Repo.DB.WithContext(ctx).Where("id = ?", newID).First(&node).Error; err != nil {
			return nil, errs.ErrDB.WithErr(err)
		}
		parent = &node
	}
	if parent == nil {
		return nil, errs.Invalid("层级路径不合法")
	}
	return parent, nil
}

// SpaceExport 导出小区基础信息明细（层级 + 房屋）。
func (s *Service) SpaceExport(ctx context.Context, communityID, nodeID uint64, p *principal.Info) ([]byte, string, error) {
	q := &HouseQuery{CommunityID: jsonx.Uint64(communityID), NodeID: jsonx.Uint64(nodeID)}
	q.Page = 1
	q.PageSize = 200
	var all []HouseVO
	for page := 1; page <= 250; page++ {
		q.Page = page
		list, total, err := s.ListHouses(ctx, q, p)
		if err != nil {
			return nil, "", err
		}
		all = append(all, list...)
		if int64(len(all)) >= total || len(list) == 0 {
			break
		}
	}
	// 节点类别：一次查出来，导出时标注楼栋类型
	catByNode := map[uint64]int{}
	if len(all) > 0 {
		nodeIDs := make([]uint64, 0, len(all))
		for _, h := range all {
			nodeIDs = append(nodeIDs, h.NodeID)
		}
		var nodes []entity.StructureNode
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).Where("id IN ?", nodeIDs).Find(&nodes).Error
		for _, n := range nodes {
			catByNode[n.ID] = n.Category
		}
	}
	rows := make([][]any, 0, len(all))
	for _, h := range all {
		rows = append(rows, []any{
			h.NodePath, spaceCategoryText(catByNode[h.NodeID]), h.Code, h.HouseNo,
			houseUsageText(h.UsageType), h.HouseTypeName,
			h.AreaBuild, h.AreaUse, h.AreaShare, h.Layout, h.Orientation,
			decorationText(h.Decoration), houseStatusText(h.HouseStatus),
			propertyRightText(h.PropertyRight), yesNo(h.IsStreet), h.CeilingHeight,
			formatDateOnly(h.DeliveryDate), h.Remark,
		})
	}
	data, err := excel.Build("小区基础信息", spaceExportHeaders, rows)
	if err != nil {
		return nil, "", errs.ErrInternal.WithErr(err)
	}
	return data, "小区基础信息_" + time.Now().Format("20060102_150405") + ".xlsx", nil
}

// isYes 解析"是/否/1/0/true"。
func isYes(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "是" || v == "1" || v == "true" || v == "y"
}
