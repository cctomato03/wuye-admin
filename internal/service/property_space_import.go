// 小区基础信息：房屋批量导入 / 导出。
//
// 导入模板只有三列：房屋编码、房屋地址、房屋面积。
// 房屋地址必须是「小区地址 + 小区名称 + 层级结构 + 房屋名称」，例如：
//
//	湖北省武汉市江岸区黄浦大街25号黄浦雅苑1栋4单元1楼1号
//
// 解析后自动补齐缺失层级，再挂载房屋。
//
// 导入采用"全量校验 + 一次性写入"：只要有任何一行不合格，整批不导入，
// 并把所有问题逐行返回，避免出现"导入了一半"的脏数据。
package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"gorm.io/gorm"

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
	// Aborted=true 表示校验未通过，本次没有写入任何数据
	Aborted bool `json:"aborted"`
}

// addressSegment 地址解析出的一段：名称 + 对应层级类型（0 表示房屋名称）。
type addressSegment struct {
	Name string
	Type int // 1区域 2楼栋 3单元 4楼层 0房屋
}

// parseHouseAddress 解析"层级结构 + 房屋名称"。
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
		if i > start && runes[i] == '元' && runes[i-1] == '单' {
			segments = append(segments, addressSegment{Name: strings.TrimSpace(string(runes[start : i+1])), Type: 3})
			start = i + 1
			continue
		}
		switch runes[i] {
		case '区':
			segments = append(segments, addressSegment{Name: strings.TrimSpace(string(runes[start : i+1])), Type: 1})
			start = i + 1
		case '栋', '幢', '座':
			segments = append(segments, addressSegment{Name: strings.TrimSpace(string(runes[start : i+1])), Type: 2})
			start = i + 1
		case '层', '楼':
			segments = append(segments, addressSegment{Name: strings.TrimSpace(string(runes[start : i+1])), Type: 4})
			start = i + 1
		case '号':
			houseNo = strings.TrimSpace(string(runes[start : i+1]))
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

// planNode 校验阶段规划出的待创建层级。
type planNode struct {
	Key       string
	ParentKey string
	ParentID  uint64 // 已存在节点直接用 ID；新建节点为 0
	Name      string
	Type      int
	Node      *entity.StructureNode // 已存在或新建后的节点（新建时为预生成对象）
	IsNew     bool
}

// planHouse 校验通过、待写入的房屋。
type planHouse struct {
	Line     int
	Code     string
	HouseNo  string
	NodeKey  string
	NodeID   uint64
	NodeAddr string
	Area     float64
}

// ImportStructures 按「房屋地址」导入房屋：先全量校验，全部通过后一次性写入。
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
	addErr := func(line int, msg string) {
		res.Failed++
		if len(res.Errors) < 500 {
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：%s", line, msg))
		}
	}

	// 现有层级：key = parentID|name
	type nodeKey struct {
		ParentID uint64
		Name     string
	}
	var existNodes []entity.StructureNode
	if err := s.Repo.DB.WithContext(ctx).Where("community_id = ?", communityID).Find(&existNodes).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	nodeMap := map[nodeKey]*entity.StructureNode{}
	for i := range existNodes {
		n := &existNodes[i]
		nodeMap[nodeKey{ParentID: n.ParentID, Name: n.Name}] = n
	}
	planNodes := []*planNode{}
	// 预生成新节点对象（父节点 ID 稍后回填），用指针保证后续行复用同一对象
	ensurePlanNode := func(parentID uint64, parentKey string, name string, typ int) *entity.StructureNode {
		k := nodeKey{ParentID: parentID, Name: name}
		if n, ok := nodeMap[k]; ok {
			return n
		}
		// 尚未落库的新节点用 parentKey+name 作为临时键
		tempKey := fmt.Sprintf("%s|%s", parentKey, name)
		for _, pn := range planNodes {
			if pn.Key == tempKey {
				return pn.Node
			}
		}
		node := &entity.StructureNode{
			Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: time.Now(), UpdateBy: p.UserID},
			TenantID:    com.TenantID,
			CommunityID: communityID,
			ParentID:    parentID,
			Code:        fmt.Sprintf("IMP%d", id.Next()),
			Name:        name,
			NodeType:    typ,
			Category:    9,
			Status:      1,
		}
		planNodes = append(planNodes, &planNode{Key: tempKey, Name: name, Type: typ, Node: node, IsNew: true})
		nodeMap[k] = node
		return node
	}

	// ---------- 阶段一：全量校验 ----------
	codes := make([]string, 0, len(rows)-1)
	parsed := make([]struct {
		line     int
		code     string
		houseNo  string
		node     *entity.StructureNode
		segments []addressSegment
		area     float64
	}, 0, len(rows)-1)
	seen := map[string]int{}
	for i, row := range rows[1:] {
		lineNo := i + 2
		res.Total++
		code := cell(row, 0)
		address := normalizeAddr(cell(row, 1))
		areaText := cell(row, 2)
		if code == "" {
			addErr(lineNo, "房屋编码不能为空")
			continue
		}
		if address == "" {
			addErr(lineNo, "房屋地址不能为空")
			continue
		}
		if first, ok := seen[code]; ok {
			addErr(lineNo, fmt.Sprintf("房屋编码与第 %d 行重复：%s", first, code))
			continue
		}
		if !strings.HasPrefix(address, prefix) {
			addErr(lineNo, fmt.Sprintf("房屋地址与小区不匹配，应以“%s”开头", prefix))
			continue
		}
		segments, houseNo, err := parseHouseAddress(strings.TrimPrefix(address, prefix))
		if err != nil {
			addErr(lineNo, err.Error())
			continue
		}
		var parentID uint64
		parentKey := "0"
		var last *entity.StructureNode
		for _, seg := range segments {
			last = ensurePlanNode(parentID, parentKey, seg.Name, seg.Type)
			parentID = last.ID
			parentKey = fmt.Sprintf("%s|%s", parentKey, seg.Name)
		}
		seen[code] = lineNo
		codes = append(codes, code)
		parsed = append(parsed, struct {
			line     int
			code     string
			houseNo  string
			node     *entity.StructureNode
			segments []addressSegment
			area     float64
		}{line: lineNo, code: code, houseNo: houseNo, node: last, segments: segments, area: toFloat(areaText)})
	}
	// 与库中已有编码比对
	if len(codes) > 0 {
		var dupCodes []string
		if err := s.Repo.DB.WithContext(ctx).Model(&entity.House{}).
			Where("community_id = ? AND code IN ?", communityID, codes).
			Pluck("code", &dupCodes).Error; err != nil {
			return nil, errs.ErrDB.WithErr(err)
		}
		if len(dupCodes) > 0 {
			dupSet := map[string]bool{}
			for _, c := range dupCodes {
				dupSet[c] = true
			}
			kept := parsed[:0]
			for _, it := range parsed {
				if dupSet[it.code] {
					addErr(it.line, "房屋编码已存在："+it.code)
					continue
				}
				kept = append(kept, it)
			}
			parsed = kept
		}
	}
	if res.Failed > 0 {
		// 有任一行不合格：整批不导入
		res.Aborted = true
		res.Success = 0
		res.Houses = 0
		return res, nil
	}

	// ---------- 阶段二：一次性写入（事务） ----------
	err = s.Repo.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 创建缺失层级（父先子后：planNodes 按解析顺序天然父先）
		for _, pn := range planNodes {
			if !pn.IsNew {
				continue
			}
			n := pn.Node
			// 解析父节点地址与层级
			var parent *entity.StructureNode
			if n.ParentID != 0 {
				var pnEntity entity.StructureNode
				if err := tx.Where("id = ?", n.ParentID).First(&pnEntity).Error; err != nil {
					return err
				}
				parent = &pnEntity
			}
			level := 1
			path := fmt.Sprintf("0,%d", n.ID)
			baseAddr := communityBaseAddress(com)
			if parent != nil {
				level = parent.LevelNo + 1
				path = fmt.Sprintf("%s,%d", parent.Path, n.ID)
				if parent.FullAddress != "" {
					baseAddr = parent.FullAddress
				}
			}
			n.LevelNo = level
			n.Path = path
			n.FullAddress = baseAddr + n.Name
			if err := tx.Create(n).Error; err != nil {
				return err
			}
		}
		// 写入房屋
		for _, it := range parsed {
			var buildingID, unitID, floorID uint64
			var nodeID uint64
			var nodeAddr string
			if it.node != nil {
				nodeID = it.node.ID
				nodeAddr = it.node.FullAddress
				// 重新读取（可能是本事务内新建的）
				var fresh entity.StructureNode
				if err := tx.Where("id = ?", nodeID).First(&fresh).Error; err == nil {
					nodeAddr = fresh.FullAddress
				}
			}
			// 沿父链取楼栋/单元/楼层 ID（同一小区可能存在多个同名单元，必须按链取）
			cur := nodeID
			for depth := 0; depth < 10 && cur != 0; depth++ {
				var n entity.StructureNode
				if err := tx.Where("id = ?", cur).First(&n).Error; err != nil {
					break
				}
				switch n.NodeType {
				case 2:
					buildingID = n.ID
				case 3:
					unitID = n.ID
				case 4:
					floorID = n.ID
				}
				cur = n.ParentID
			}
			fullName := prefix + it.houseNo
			if nodeAddr != "" {
				fullName = nodeAddr + it.houseNo
			}
			house := &entity.House{
				Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: time.Now(), UpdateBy: p.UserID},
				TenantID:    com.TenantID,
				CommunityID: com.ID,
				NodeID:      nodeID,
				BuildingID:  buildingID,
				UnitID:      unitID,
				FloorID:     floorID,
				Code:        it.code,
				HouseNo:     it.houseNo,
				FullName:    fullName,
				UsageType:   1,
				AreaBuild:   it.area,
				AreaUse:     it.area,
				HouseStatus: 1,
			}
			if err := tx.Create(house).Error; err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
					return errs.Invalid("房屋编码已存在：" + it.code)
				}
				return err
			}
			res.Success++
			res.Houses++
		}
		return nil
	})
	if err != nil {
		res.Success = 0
		res.Houses = 0
		res.Aborted = true
		res.Failed = res.Total
		res.Errors = append(res.Errors, "导入失败已整体回滚："+fmtErr(err))
		return res, nil
	}
	s.recalcNodeCounts(ctx, communityID)
	s.recalcCommunityCounts(ctx, communityID)
	return res, nil
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
