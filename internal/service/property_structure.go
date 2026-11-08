// 楼宇结构的"旧接口适配层"：区域/楼栋/单元/楼层 四个入口保留（历史接口与导入模板兼容），
// 但底层统一写入 pm_structure_node（自定义层级树）。新代码请直接用层级 / 房屋接口。
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

// ============================ 区域 ============================

// RegionReq 区域入参（兼容旧接口）。
type RegionReq struct {
	CommunityID jsonx.Uint64 `json:"communityId"`
	Name        string       `json:"name" binding:"required,max=64"`
	Code        string       `json:"code" binding:"max=64"`
	RegionType  int          `json:"regionType"`
	Sort        int          `json:"sort"`
	Status      int          `json:"status"`
	Remark      string       `json:"remark" binding:"max=500"`
}

// CreateRegion 新增区域（= 自定义层级树的第一级节点）。
func (s *Service) CreateRegion(ctx context.Context, req *RegionReq, p *principal.Info) (uint64, error) {
	return s.CreateNode(ctx, &NodeReq{
		CommunityID: req.CommunityID,
		ParentID:    0,
		Code:        req.Code,
		Name:        req.Name,
		NodeType:    1,
		Sort:        req.Sort,
		Status:      req.Status,
		Remark:      req.Remark,
	}, p)
}

// UpdateRegion 修改区域。
func (s *Service) UpdateRegion(ctx context.Context, regionID uint64, req *RegionReq, p *principal.Info) error {
	return s.UpdateNode(ctx, regionID, &NodeReq{
		CommunityID: req.CommunityID,
		ParentID:    0,
		Code:        req.Code,
		Name:        req.Name,
		NodeType:    1,
		Sort:        req.Sort,
		Status:      req.Status,
		Remark:      req.Remark,
	}, p)
}

// DeleteRegion 删除区域。
func (s *Service) DeleteRegion(ctx context.Context, regionID uint64, p *principal.Info) error {
	return s.DeleteNode(ctx, regionID, p)
}

// ============================ 楼栋 ============================

// BuildingReq 楼栋入参（兼容旧接口）。
type BuildingReq struct {
	CommunityID  jsonx.Uint64 `json:"communityId"`
	RegionID     jsonx.Uint64 `json:"regionId"`
	Code         string       `json:"code" binding:"required,max=64"`
	Name         string       `json:"name" binding:"required,max=64"`
	BuildingType int          `json:"buildingType"`
	TotalFloors  int          `json:"totalFloors"`
	UnderFloors  int          `json:"underFloors"`
	BuildYear    int          `json:"buildYear"`
	Address      string       `json:"address" binding:"max=255"`
	Sort         int          `json:"sort"`
	Status       int          `json:"status"`
	Remark       string       `json:"remark" binding:"max=500"`
}

// CreateBuilding 新增楼栋（挂在区域下；无区域时直接挂小区）。
func (s *Service) CreateBuilding(ctx context.Context, req *BuildingReq, p *principal.Info) (uint64, error) {
	return s.CreateNode(ctx, &NodeReq{
		CommunityID: req.CommunityID,
		ParentID:    req.RegionID,
		Code:        req.Code,
		Name:        req.Name,
		NodeType:    2,
		TotalFloors: req.TotalFloors,
		Sort:        req.Sort,
		Status:      req.Status,
		Remark:      req.Remark,
	}, p)
}

// UpdateBuilding 修改楼栋。
func (s *Service) UpdateBuilding(ctx context.Context, buildingID uint64, req *BuildingReq, p *principal.Info) error {
	return s.UpdateNode(ctx, buildingID, &NodeReq{
		CommunityID: req.CommunityID,
		ParentID:    req.RegionID,
		Code:        req.Code,
		Name:        req.Name,
		NodeType:    2,
		TotalFloors: req.TotalFloors,
		Sort:        req.Sort,
		Status:      req.Status,
		Remark:      req.Remark,
	}, p)
}

// DeleteBuilding 删除楼栋。
func (s *Service) DeleteBuilding(ctx context.Context, buildingID uint64, p *principal.Info) error {
	return s.DeleteNode(ctx, buildingID, p)
}

// ============================ 单元 ============================

// UnitReq 单元入参（兼容旧接口）。
type UnitReq struct {
	CommunityID    jsonx.Uint64 `json:"communityId"`
	BuildingID     jsonx.Uint64 `json:"buildingId" binding:"required"`
	Code           string       `json:"code" binding:"required,max=64"`
	Name           string       `json:"name" binding:"required,max=64"`
	TotalFloors    int          `json:"totalFloors"`
	HousesPerFloor int          `json:"housesPerFloor"`
	Elevator       bool         `json:"elevator"`
	Sort           int          `json:"sort"`
	Status         int          `json:"status"`
	Remark         string       `json:"remark" binding:"max=500"`
}

// CreateUnit 新增单元（挂在楼栋节点下）。
func (s *Service) CreateUnit(ctx context.Context, req *UnitReq, p *principal.Info) (uint64, error) {
	node, err := s.nodeScoped(ctx, p, req.BuildingID.Uint64())
	if err != nil {
		return 0, err
	}
	return s.CreateNode(ctx, &NodeReq{
		CommunityID: jsonx.Uint64(node.CommunityID),
		ParentID:    req.BuildingID,
		Code:        req.Code,
		Name:        req.Name,
		NodeType:    3,
		TotalFloors: req.TotalFloors,
		Sort:        req.Sort,
		Status:      req.Status,
		Remark:      req.Remark,
	}, p)
}

// UpdateUnit 修改单元。
func (s *Service) UpdateUnit(ctx context.Context, unitID uint64, req *UnitReq, p *principal.Info) error {
	node, err := s.nodeScoped(ctx, p, unitID)
	if err != nil {
		return err
	}
	parentID := req.BuildingID.Uint64()
	if parentID == 0 {
		parentID = node.ParentID
	}
	return s.UpdateNode(ctx, unitID, &NodeReq{
		CommunityID: jsonx.Uint64(node.CommunityID),
		ParentID:    jsonx.Uint64(parentID),
		Code:        req.Code,
		Name:        req.Name,
		NodeType:    3,
		TotalFloors: req.TotalFloors,
		Sort:        req.Sort,
		Status:      req.Status,
		Remark:      req.Remark,
	}, p)
}

// DeleteUnit 删除单元。
func (s *Service) DeleteUnit(ctx context.Context, unitID uint64, p *principal.Info) error {
	return s.DeleteNode(ctx, unitID, p)
}

// ============================ 结构树（旧接口） ============================

// StructureTreeNode 旧版结构树节点（区域 → 楼栋 → 单元 → 楼层）。
type StructureTreeNode struct {
	ID       uint64               `json:"id,string"`
	ParentID uint64               `json:"parentId,string"`
	Level    string               `json:"level"`
	Name     string               `json:"name"`
	Code     string               `json:"code"`
	Extra    map[string]any       `json:"extra,omitempty"`
	Children []*StructureTreeNode `json:"children,omitempty"`
}

// StructureTree 兼容旧接口：把自定义层级按 node_type 映射成 区域/楼栋/单元/楼层 展示。
// 新前端请使用 /property/nodes/tree（完全自由的层级）。
func (s *Service) StructureTree(ctx context.Context, communityID uint64, p *principal.Info) ([]*StructureTreeNode, error) {
	nodes, err := s.NodeTree(ctx, communityID, p)
	if err != nil {
		return nil, err
	}
	nodeMap := make(map[uint64]*StructureTreeNode, len(nodes))
	for _, n := range nodes {
		level := "unit"
		switch n.NodeType {
		case 1:
			level = "region"
		case 2:
			level = "building"
		case 4:
			level = "floor"
		}
		nodeMap[n.ID] = &StructureTreeNode{
			ID: n.ID, ParentID: n.ParentID, Level: level, Name: n.Name, Code: n.Code,
			Extra: map[string]any{"nodeType": n.NodeType, "houseCount": n.HouseCount, "levelNo": n.LevelNo},
		}
	}
	roots := make([]*StructureTreeNode, 0)
	for _, n := range nodes {
		tn := nodeMap[n.ID]
		if n.ParentID == 0 {
			roots = append(roots, tn)
			continue
		}
		if parent, ok := nodeMap[n.ParentID]; ok {
			parent.Children = append(parent.Children, tn)
		} else {
			roots = append(roots, tn)
		}
	}
	return roots, nil
}

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

// structureImportHeaders 导入表头。
var structureImportHeaders = []string{
	"层级路径", "房屋编码", "房屋房号", "房屋用途", "房屋类型", "建筑面积", "套内面积",
	"户型", "朝向", "装修", "房屋状态",
}

// StructureImportTemplate 下载导入模板（层级路径用 / 分层，节点不存在会自动创建）。
func (s *Service) StructureImportTemplate() ([]byte, error) {
	rows := [][]any{
		{"1栋/1单元/1层", "B01-U1-F1-101", "101", "住宅", "三室两厅", 98.5, 82.3, "3室2厅", "南", "精装", "空置"},
		{"1栋/1单元/1层", "B01-U1-F1-102", "102", "住宅", "两室一厅", 78.2, 65.1, "2室1厅", "东南", "简装", "空置"},
		{"商业1号楼/1层", "S01-F1-101", "101", "商业", "商铺", 120, 110, "", "临街", "毛坯", "空置"},
	}
	data, err := excel.Build("房屋导入", structureImportHeaders, rows)
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

// ImportStructures 导入房屋：按"层级路径"自动建节点（用 / 分隔），房屋挂到最末级节点。
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
		houseCode := cell(row, 1)
		if path == "" || houseCode == "" {
			failed(lineNo, "层级路径与房屋编码为必填")
			continue
		}
		node, err := s.ensureNodePath(ctx, communityID, path, p)
		if err != nil {
			failed(lineNo, fmtErr(err))
			continue
		}
		var houseTypeID uint64
		if tn := cell(row, 4); tn != "" {
			if v, ok := typeNames[tn]; ok {
				houseTypeID = v
			} else {
				var exist entity.HouseType
				if err := s.Repo.DB.WithContext(ctx).Where("tenant_id = ? AND name = ? AND community_id IN ?", p.TenantID, tn, []uint64{0, communityID}).First(&exist).Error; err == nil {
					houseTypeID = exist.ID
				} else {
					if newID, err := s.CreateHouseType(ctx, &HouseTypeReq{CommunityID: jsonx.Uint64(communityID), Name: tn, UsageType: parseUsageType(cell(row, 3))}, p); err == nil {
						houseTypeID = newID
					}
				}
				typeNames[tn] = houseTypeID
			}
		}
		if _, err := s.CreateHouse(ctx, &HouseReq{
			CommunityID: jsonx.Uint64(communityID),
			NodeID:      jsonx.Uint64(node.ID),
			HouseTypeID: jsonx.Uint64(houseTypeID),
			Code:        houseCode,
			HouseNo:     defaultIfEmpty(cell(row, 2), houseCode),
			UsageType:   parseUsageType(cell(row, 3)),
			AreaBuild:   toFloat(cell(row, 5)),
			AreaUse:     toFloat(cell(row, 6)),
			Layout:      cell(row, 7),
			Orientation: cell(row, 8),
			Decoration:  parseDecoration(cell(row, 9)),
			HouseStatus: parseHouseStatus(cell(row, 10)),
		}, p); err != nil {
			failed(lineNo, fmtErr(err))
			continue
		}
		res.Success++
		res.Houses++
	}
	return res, nil
}

// ensureNodePath 按 "1栋/1单元/1层" 路径逐级复用或创建节点。
func (s *Service) ensureNodePath(ctx context.Context, communityID uint64, path string, p *principal.Info) (*entity.StructureNode, error) {
	segments := strings.Split(path, "/")
	var parent *entity.StructureNode
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
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
		newID, err := s.CreateNode(ctx, &NodeReq{
			CommunityID: jsonx.Uint64(communityID),
			ParentID:    parentID,
			Name:        seg,
			NodeType:    5,
		}, p)
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
