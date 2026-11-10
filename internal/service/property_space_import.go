// 小区基础信息：房屋批量导入 / 导出。
//
// 导入模板只有三列：房屋编码、房屋地址、房屋面积。
// 房屋地址必须是「小区地址 + 小区名称 + 层级结构 + 房屋名称」，例如：
//
//	湖北省武汉市江岸区黄浦大街25号黄浦雅苑1栋4单元1楼1号
//
// 其中"湖北省武汉市江岸区黄浦大街25号"是小区地址、"黄浦雅苑"是小区名称，
// 剩余部分按 区/栋/单元/层(楼)/号 切分：1栋(楼栋) 4单元(单元) 1楼(楼层) 1号(房屋名称)。
// 系统按解析结果补齐缺失层级，再把房屋挂到最末级节点上。
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
	"web-basic/pkg/id"
	"web-basic/pkg/jsonx"
)

// spaceImportHeaders 导入模板表头（只保留业务必需的三列）。
var spaceImportHeaders = []string{"房屋编码", "房屋地址", "房屋面积"}

// spaceExportHeaders 导出表头。
var spaceExportHeaders = []string{
	"层级路径", "房屋编码", "房号", "房屋用途", "房屋类型",
	"建筑面积", "套内面积", "公摊面积", "户型", "朝向", "装修", "房屋状态",
	"产权性质", "是否临街", "层高", "交付日期", "备注",
}

// SpaceImportTemplate 下载导入模板（示例地址按当前小区的真实地址 + 名称拼接）。
func (s *Service) SpaceImportTemplate(ctx context.Context, communityID uint64, p *principal.Info) ([]byte, error) {
	base := "湖北省武汉市江岸区黄浦大街25号黄浦雅苑"
	if communityID != 0 {
		if com, err := s.findCommunity(ctx, p, communityID); err == nil && com != nil {
			base = communityBaseAddress(com) + com.Name
		}
	}
	rows := [][]any{
		{"1-4-101", base + "1栋4单元1楼1号", 89.5},
		{"1-4-102", base + "1栋4单元1楼2号", 78.2},
		{"2-1-301", base + "2栋1单元3楼1号", 120},
	}
	data, err := excel.Build("小区基础信息导入", spaceImportHeaders, rows)
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	return data, nil
}

// StructureImportResult 导入结果。
type StructureImportResult struct {
	Total   int      `json:"total"`
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors"`
	Houses  int      `json:"houses"`
}

// addressSegment 地址解析出的一段：名称 + 对应层级类型（0 表示房屋名称）。
type addressSegment struct {
	Name string
	Type int // 1区域 2楼栋 3单元 4楼层 0房屋
}

// parseHouseAddress 解析"层级结构 + 房屋名称"。
//
// 规则：从左到右扫描，遇到 区 / 栋 / 单元 / 层 / 楼 / 号 即切分一段；
//   - 区 → 区域(1)
//   - 栋 → 楼栋(2)
//   - 单元 → 单元(3)
//   - 层 / 楼 → 楼层(4)
//   - 号 → 房屋名称(0)
func parseHouseAddress(rest string) ([]addressSegment, string, error) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return nil, "", errs.Invalid("房屋地址缺少层级结构/房屋名称")
	}
	segments := []addressSegment{}
	houseNo := ""
	runes := []rune(rest)
	start := 0
	for i := 0; i < len(runes); i++ {
		// 先看双字后缀"单元"
		if i > start && runes[i] == '元' && runes[i-1] == '单' {
			name := strings.TrimSpace(string(runes[start : i+1]))
			segments = append(segments, addressSegment{Name: name, Type: 3})
			start = i + 1
			continue
		}
		switch runes[i] {
		case '区':
			name := strings.TrimSpace(string(runes[start : i+1]))
			segments = append(segments, addressSegment{Name: name, Type: 1})
			start = i + 1
		case '栋', '幢', '座':
			name := strings.TrimSpace(string(runes[start : i+1]))
			segments = append(segments, addressSegment{Name: name, Type: 2})
			start = i + 1
		case '层', '楼':
			name := strings.TrimSpace(string(runes[start : i+1]))
			segments = append(segments, addressSegment{Name: name, Type: 4})
			start = i + 1
		case '号':
			name := strings.TrimSpace(string(runes[start : i+1]))
			houseNo = name
			start = i + 1
		}
	}
	if houseNo == "" {
		return nil, "", errs.Invalid("房屋地址必须以“号”结尾表示房屋（如 1栋4单元1楼1号）")
	}
	if trailing := strings.TrimSpace(string(runes[start:])); trailing != "" {
		return nil, "", errs.Invalid("房屋地址在“号”之后还有多余内容：" + trailing)
	}
	return segments, houseNo, nil
}

// ImportStructures 按「房屋地址」导入房屋：自动补齐缺失层级，再挂载房屋。
func (s *Service) ImportStructures(ctx context.Context, communityID uint64, r io.Reader, p *principal.Info) (*StructureImportResult, error) {
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	com, err := s.findCommunity(ctx, p, communityID)
	if err != nil {
		return nil, err
	}
	if com == nil {
		return nil, errs.ErrNotFound
	}
	rows, err := excel.Read(r, 20000)
	if err != nil {
		return nil, errs.Invalid("文件解析失败：请确认上传的是 .xlsx 格式的 Excel 文件")
	}
	if len(rows) <= 1 {
		return nil, errs.Invalid("文件没有数据行")
	}
	// 小区前缀 = 小区地址 + 小区名称
	normalizeAddr := func(v string) string {
		v = strings.ReplaceAll(v, " ", "")
		v = strings.ReplaceAll(v, "\u3000", "")
		return strings.TrimSpace(v)
	}
	prefix := normalizeAddr(communityBaseAddress(com) + com.Name)
	if prefix == "" {
		return nil, errs.Invalid("请先完善小区的行政区划/地址信息，再导入房屋")
	}
	res := &StructureImportResult{Errors: []string{}}
	cell := func(row []string, i int) string {
		if i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	failed := func(line int, msg string) {
		res.Failed++
		if len(res.Errors) < 200 {
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：%s", line, msg))
		}
	}
	seen := map[string]bool{}
	for i, row := range rows[1:] {
		lineNo := i + 2
		res.Total++
		code := cell(row, 0)
		address := normalizeAddr(cell(row, 1))
		areaText := cell(row, 2)
		if code == "" {
			failed(lineNo, "房屋编码不能为空")
			continue
		}
		if address == "" {
			failed(lineNo, "房屋地址不能为空")
			continue
		}
		if seen[code] {
			failed(lineNo, "房屋编码在本次文件中重复："+code)
			continue
		}
		if !strings.HasPrefix(address, prefix) {
			failed(lineNo, fmt.Sprintf("房屋地址与小区不匹配，地址是“%s”, 应以“%s”开头", address, prefix))
			continue
		}
		rest := strings.TrimPrefix(address, prefix)
		segments, houseNo, err := parseHouseAddress(rest)
		if err != nil {
			failed(lineNo, err.Error())
			continue
		}
		// 补齐/复用层级：从小区根开始逐级查找
		var parentID uint64
		var parentAddr string
		var lastNode *entity.StructureNode
		var buildingID, unitID, floorID uint64
		var buildErr error
		for _, seg := range segments {
			node, err := s.ensureChildNode(ctx, communityID, com, parentID, seg.Name, seg.Type, p)
			if err != nil {
				buildErr = err
				break
			}
			parentID = node.ID
			parentAddr = node.FullAddress
			lastNode = node
			switch seg.Type {
			case 2:
				buildingID = node.ID
			case 3:
				unitID = node.ID
			case 4:
				floorID = node.ID
			}
		}
		if buildErr != nil {
			failed(lineNo, fmtErr(buildErr))
			continue
		}
		area := toFloat(areaText) // 空面积按 0 处理
		var nodePtr *entity.StructureNode
		if lastNode != nil {
			nodePtr = lastNode
		}
		house := &entity.House{
			Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: time.Now(), UpdateBy: p.UserID},
			TenantID:    com.TenantID,
			CommunityID: com.ID,
			NodeID:      parentID,
			BuildingID:  buildingID,
			UnitID:      unitID,
			FloorID:     floorID,
			Code:        code,
			HouseNo:     houseNo,
			FullName:    parentAddr + houseNo,
			UsageType:   1,
			AreaBuild:   area,
			AreaUse:     area,
			HouseStatus: 1,
		}
		if nodePtr == nil {
			house.FullName = prefix + houseNo
		}
		if err := s.Repo.DB.WithContext(ctx).Create(house).Error; err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				failed(lineNo, "房屋编码已存在："+code)
			} else {
				failed(lineNo, fmtErr(err))
			}
			continue
		}
		seen[code] = true
		res.Success++
		res.Houses++
	}
	// 导入后重算节点/小区房屋统计
	s.recalcNodeCounts(ctx, communityID)
	s.recalcCommunityCounts(ctx, communityID)
	return res, nil
}

// ensureChildNode 在 parentID 下按名称复用或创建指定类型的层级节点。
func (s *Service) ensureChildNode(ctx context.Context, communityID uint64, com *entity.Community, parentID uint64, name string, nodeType int, p *principal.Info) (*entity.StructureNode, error) {
	var node entity.StructureNode
	q := s.Repo.DB.WithContext(ctx).Where("community_id = ? AND parent_id = ? AND name = ?", communityID, parentID, name)
	if err := q.First(&node).Error; err == nil {
		return &node, nil
	}
	newID, err := s.CreateNode(ctx, &NodeReq{
		CommunityID: jsonx.Uint64(communityID),
		ParentID:    jsonx.Uint64(parentID),
		Name:        name,
		NodeType:    nodeType,
	}, p)
	if err != nil {
		return nil, err
	}
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", newID).First(&node).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return &node, nil
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
			h.NodePath, h.Code, h.HouseNo,
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
