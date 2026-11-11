// 楼宇结构（自定义层级 + 房屋）。
//
// 数据模型：pm_structure_node 是一棵以小区为根的树，层级数量与名称由物业自行维护
// （1栋 → 1单元 → 2层 这样任意组合，没有区域/单元也能用）；pm_house.node_id 指向
// 房屋挂载的节点（通常是叶子）。旧的 pm_region/pm_building/pm_unit/pm_floor 只作为
// 历史数据来源，迁移后不再写入。
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

// ============================ 层级节点 ============================

// NodeReq 层级节点入参。
type NodeReq struct {
	CommunityID   jsonx.Uint64 `json:"communityId" binding:"required"`
	ParentID      jsonx.Uint64 `json:"parentId"`
	Code          string       `json:"code" binding:"max=64"`
	Name          string       `json:"name" binding:"required,max=64"`
	NodeType      int          `json:"nodeType"`
	Category      int          `json:"category"`
	TotalFloors   int          `json:"totalFloors"`
	FloorsAbove   int          `json:"floorsAbove"`
	FloorsUnder   int          `json:"floorsUnder"`
	UnitCount     int          `json:"unitCount"`
	ElevatorCount int          `json:"elevatorCount"`
	BuildArea     float64      `json:"buildArea"`
	BuildYear     int          `json:"buildYear"`
	StructureType string       `json:"structureType" binding:"max=32"`
	DeliveryDate  string       `json:"deliveryDate"`
	Address       string       `json:"address" binding:"max=255"`
	FloorNo       int          `json:"floorNo"`
	Sort          int          `json:"sort"`
	Status        int          `json:"status"`
	Remark        string       `json:"remark" binding:"max=500"`
}

// NodeVO 层级节点出参（带级别名称）。
type NodeVO struct {
	entity.StructureNode
	ParentName string `json:"parentName"`
	LevelName  string `json:"levelName"`
	HouseCount int    `json:"houseCount"`
	ChildCount int    `json:"childCount"`
	FullName   string `json:"fullName"`
	TypeName   string `json:"typeName"`
}

var nodeTypeNames = map[int]string{1: "区域", 2: "楼栋", 3: "单元", 4: "楼层"}

// nodeTypeName 层级类型名称（只允许区域/楼栋/单元/楼层）。
func nodeTypeName(t int) string {
	if v, ok := nodeTypeNames[t]; ok {
		return v
	}
	return "层级"
}

// validNodeType 层级类型必须来自字典 space_node_type（区域/楼栋/单元/楼层）。
func validNodeType(t int) bool {
	_, ok := nodeTypeNames[t]
	return ok
}

// communityBaseAddress 小区基础地址（省市区 + 详细地址）。
func communityBaseAddress(c *entity.Community) string {
	if c == nil {
		return ""
	}
	addr := strings.TrimSpace(c.Address)
	if addr == "" {
		addr = strings.TrimSpace(c.Province + c.City + c.District)
	}
	return addr
}

// NodeTree 返回小区层级树（节点自身，不再固定四级）。
func (s *Service) NodeTree(ctx context.Context, communityID uint64, p *principal.Info) ([]*NodeVO, error) {
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	var list []entity.StructureNode
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).
		Where("community_id = ?", communityID).Order("level_no ASC, sort ASC, id ASC").Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	// 统计：每节点直接房屋数 与 子节点数
	var houseRows []struct {
		NodeID uint64
		Count  int
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.House{}).
		Select("node_id, COUNT(*) AS count").Where("community_id = ? AND node_id <> 0", communityID).
		Group("node_id").Scan(&houseRows).Error
	houseCount := make(map[uint64]int, len(houseRows))
	for _, r := range houseRows {
		houseCount[r.NodeID] = r.Count
	}
	childCount := make(map[uint64]int, len(list))
	nameByID := make(map[uint64]string, len(list))
	for _, n := range list {
		nameByID[n.ID] = n.Name
		if n.ParentID != 0 {
			childCount[n.ParentID]++
		}
	}
	out := make([]*NodeVO, 0, len(list))
	for _, n := range list {
		out = append(out, &NodeVO{
			StructureNode: n,
			ParentName:    nameByID[n.ParentID],
			LevelName:     nodeTypeNames[n.NodeType],
			HouseCount:    houseCount[n.ID],
			ChildCount:    childCount[n.ID],
			FullName:      s.nodeFullName(nameByID, n),
			TypeName:      nodeTypeName(n.NodeType),
		})
	}
	return out, nil
}

// nodeFullName 节点全称（祖先名称 + 自身），例如 1栋1单元2层。
func (s *Service) nodeFullName(nameByID map[uint64]string, n entity.StructureNode) string {
	parts := []string{}
	for _, seg := range strings.Split(n.Path, ",") {
		var idv uint64
		_, _ = fmt.Sscanf(strings.TrimSpace(seg), "%d", &idv)
		if name, ok := nameByID[idv]; ok {
			parts = append(parts, name)
		}
	}
	if len(parts) == 0 {
		return n.Name
	}
	return strings.Join(parts, "")
}

// CreateNode 新增层级节点（只关联上级，默认挂在小区下）。
func (s *Service) CreateNode(ctx context.Context, req *NodeReq, p *principal.Info) (uint64, error) {
	com, err := s.findCommunity(ctx, p, req.CommunityID.Uint64())
	if err != nil {
		return 0, err
	}
	var parent *entity.StructureNode
	if req.ParentID.Uint64() != 0 {
		parent = &entity.StructureNode{}
		if err := s.Repo.DB.WithContext(ctx).Where("id = ? AND community_id = ?", req.ParentID.Uint64(), com.ID).
			First(parent).Error; err != nil {
			return 0, errs.Invalid("上级层级不存在或不属于当前小区")
		}
	}
	if !validNodeType(req.NodeType) {
		return 0, errs.Invalid("请选择层级类型（区域/楼栋/单元/楼层）")
	}
	// 同一上级下名称必须唯一
	var dup int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).
		Where("community_id = ? AND parent_id = ? AND name = ?", com.ID, req.ParentID.Uint64(), strings.TrimSpace(req.Name)).
		Count(&dup).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	if dup > 0 {
		return 0, errs.Invalid("同一上级下已存在同名层级")
	}
	level := 1
	path := ""
	if parent != nil {
		level = parent.LevelNo + 1
		path = fmt.Sprintf("%s,%d", parent.Path, 0) // 占位，稍后写入自身 ID
	}
	now := time.Now()
	nodeID := id.Next()
	code := strings.TrimSpace(req.Code)
	if code == "" {
		code = fmt.Sprintf("N%d", nodeID)
	}
	if parent != nil {
		path = fmt.Sprintf("%s,%d", parent.Path, nodeID)
	} else {
		path = fmt.Sprintf("0,%d", nodeID)
	}
	delivery, err := parseDate(req.DeliveryDate)
	if err != nil {
		return 0, err
	}
	// 地址 = 上级地址 + 名称；小区下一级 = 小区地址 + 名称
	// 小区下一级地址 = 小区地址 + 小区名称
	baseAddr := communityBaseAddress(com) + com.Name
	if parent != nil {
		baseAddr = parent.FullAddress
		if baseAddr == "" {
			baseAddr = communityBaseAddress(com)
		}
	}
	n := &entity.StructureNode{
		Base:          entity.Base{ID: nodeID, CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:      com.TenantID,
		CommunityID:   com.ID,
		ParentID:      req.ParentID.Uint64(),
		Code:          code,
		Name:          strings.TrimSpace(req.Name),
		NodeType:      pmDefault(req.NodeType, 5),
		Category:      pmDefault(req.Category, 9),
		LevelNo:       level,
		Path:          path,
		TotalFloors:   req.TotalFloors,
		FloorsAbove:   req.FloorsAbove,
		FloorsUnder:   req.FloorsUnder,
		UnitCount:     req.UnitCount,
		ElevatorCount: req.ElevatorCount,
		BuildArea:     req.BuildArea,
		BuildYear:     req.BuildYear,
		StructureType: req.StructureType,
		DeliveryDate:  delivery,
		Address:       req.Address,
		FullAddress:   baseAddr + strings.TrimSpace(req.Name),
		FloorNo:       req.FloorNo,
		Sort:          req.Sort,
		Status:        pmDefault(req.Status, 1),
	}
	if err := s.Repo.DB.WithContext(ctx).Create(n).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	return n.ID, nil
}

// UpdateNode 修改层级节点（可调整上级，子节点路径会级联更新）。
func (s *Service) UpdateNode(ctx context.Context, nodeID uint64, req *NodeReq, p *principal.Info) error {
	n, err := s.nodeScoped(ctx, p, nodeID)
	if err != nil {
		return err
	}
	if !validNodeType(req.NodeType) {
		return errs.Invalid("请选择层级类型（区域/楼栋/单元/楼层）")
	}
	parentID := req.ParentID.Uint64()
	if parentID != 0 {
		if parentID == n.ID {
			return errs.Invalid("不能把层级挂到自己下面")
		}
		var parent entity.StructureNode
		if err := s.Repo.DB.WithContext(ctx).Where("id = ? AND community_id = ?", parentID, n.CommunityID).
			First(&parent).Error; err != nil {
			return errs.Invalid("上级层级不存在或不属于当前小区")
		}
		if strings.Contains(","+parent.Path+",", fmt.Sprintf(",%d,", n.ID)) {
			return errs.Invalid("不能把层级挂到自己的子层级下")
		}
	}
	// 同级重名校验（排除自身）
	var dup int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).
		Where("community_id = ? AND parent_id = ? AND name = ? AND id <> ?", n.CommunityID, parentID, strings.TrimSpace(req.Name), n.ID).
		Count(&dup).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if dup > 0 {
		return errs.Invalid("同一上级下已存在同名层级")
	}
	var com entity.Community
	_ = s.Repo.DB.WithContext(ctx).Where("id = ?", n.CommunityID).First(&com).Error
	baseAddr := communityBaseAddress(&com) + com.Name
	oldPath := n.Path
	newLevel := 1
	newPath := fmt.Sprintf("0,%d", n.ID)
	if parentID != 0 {
		var parent entity.StructureNode
		_ = s.Repo.DB.WithContext(ctx).Where("id = ?", parentID).First(&parent).Error
		newLevel = parent.LevelNo + 1
		newPath = fmt.Sprintf("%s,%d", parent.Path, n.ID)
		if parent.FullAddress != "" {
			baseAddr = parent.FullAddress
		}
	}
	newFullAddress := baseAddr + strings.TrimSpace(req.Name)
	delivery, err := parseDate(req.DeliveryDate)
	if err != nil {
		return err
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).Where("id = ?", n.ID).Updates(map[string]any{
		"parent_id":      parentID,
		"code":           defaultIfEmpty(strings.TrimSpace(req.Code), n.Code),
		"name":           strings.TrimSpace(req.Name),
		"node_type":      pmDefault(req.NodeType, n.NodeType),
		"category":       pmDefault(req.Category, n.Category),
		"level_no":       newLevel,
		"path":           newPath,
		"total_floors":   req.TotalFloors,
		"floors_above":   req.FloorsAbove,
		"floors_under":   req.FloorsUnder,
		"unit_count":     req.UnitCount,
		"elevator_count": req.ElevatorCount,
		"build_area":     req.BuildArea,
		"build_year":     req.BuildYear,
		"structure_type": req.StructureType,
		"delivery_date":  delivery,
		"address":        req.Address,
		"full_address":   newFullAddress,
		"floor_no":       req.FloorNo,
		"sort":           req.Sort,
		"status":         pmDefault(req.Status, 1),
		"remark":         req.Remark,
		"update_by":      p.UserID,
		"update_time":    time.Now(),
	}).Error; err != nil {
		return wrapTxErr(err)
	}
	// 子节点路径级联更新（数据量小，逐条更正；只更新 path 前缀）
	if oldPath != newPath {
		_ = s.Repo.DB.WithContext(ctx).Exec(
			"UPDATE pm_structure_node SET path = CONCAT(?, SUBSTRING(path, ?)) WHERE community_id = ? AND path LIKE ?",
			newPath, len(oldPath)+1, n.CommunityID, oldPath+",%").Error
	}
	// 自身或父级地址变化时，级联刷新所有后代地址
	if oldPath != newPath || req.Name != "" {
		s.cascadeNodeAddress(ctx, n.CommunityID, n.ID, newFullAddress)
	}
	return nil
}

// cascadeNodeAddress 递归刷新子节点的 full_address（父地址 + 名称）。
func (s *Service) cascadeNodeAddress(ctx context.Context, communityID, rootID uint64, rootAddress string) {
	var children []entity.StructureNode
	_ = s.Repo.DB.WithContext(ctx).Where("community_id = ? AND parent_id = ?", communityID, rootID).Find(&children).Error
	for _, c := range children {
		addr := rootAddress + c.Name
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).Where("id = ?", c.ID).UpdateColumn("full_address", addr).Error
		s.cascadeNodeAddress(ctx, communityID, c.ID, addr)
	}
}

// DeleteNode 删除层级节点（有子节点或房屋时拒绝）。
func (s *Service) DeleteNode(ctx context.Context, nodeID uint64, p *principal.Info) error {
	n, err := s.nodeScoped(ctx, p, nodeID)
	if err != nil {
		return err
	}
	var children, houses int64
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).Where("parent_id = ?", n.ID).Count(&children).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.House{}).Where("node_id = ?", n.ID).Count(&houses).Error
	if children > 0 {
		return errs.Invalid("该层级下还有子层级，请先删除或移动子层级")
	}
	if houses > 0 {
		return errs.Invalid("该层级下还有房屋，请先处理房屋")
	}
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.StructureNode{}, n.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// nodeScoped 带作用域读取层级节点。
func (s *Service) nodeScoped(ctx context.Context, p *principal.Info, nodeID uint64) (*entity.StructureNode, error) {
	var n entity.StructureNode
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).Where("id = ?", nodeID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return nil, errs.ErrNotFound
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	if err := tx.First(&n).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &n, nil
}

// NodeOptions 层级下拉（房屋/设备选择用，返回带全称）。
func (s *Service) NodeOptions(ctx context.Context, communityID uint64, leafOnly bool, p *principal.Info) ([]OptionVO, error) {
	nodes, err := s.NodeTree(ctx, communityID, p)
	if err != nil {
		return nil, err
	}
	hasChild := make(map[uint64]bool, len(nodes))
	for _, n := range nodes {
		if n.ParentID != 0 {
			hasChild[n.ParentID] = true
		}
	}
	out := make([]OptionVO, 0, len(nodes))
	for _, n := range nodes {
		if leafOnly && hasChild[n.ID] {
			continue
		}
		out = append(out, OptionVO{ID: n.ID, Name: n.FullName, Code: n.Code, ParentID: n.ParentID})
	}
	return out, nil
}

// ============================ 房屋 ============================

// HouseQuery 房屋查询。
type HouseQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	NodeID      jsonx.Uint64 `form:"nodeId"`
	IncludeSub  *bool        `form:"includeSub"`
	RootOnly    bool         `form:"rootOnly"`
	RegionID    jsonx.Uint64 `form:"regionId"`
	BuildingID  jsonx.Uint64 `form:"buildingId"`
	UnitID      jsonx.Uint64 `form:"unitId"`
	FloorID     jsonx.Uint64 `form:"floorId"`
	Keyword     string       `form:"keyword"`
	UsageType   *int         `form:"usageType"`
	HouseStatus *int         `form:"houseStatus"`
}

// HouseVO 房屋出参（带层级名称）。
type HouseVO struct {
	entity.House
	BuildingName  string `json:"buildingName"`
	UnitName      string `json:"unitName"`
	HouseTypeName string `json:"houseTypeName"`
	NodeName      string `json:"nodeName"`
	NodePath      string `json:"nodePath"`
}

// ListHouses 房屋分页列表（支持按节点过滤，默认含子层级）。
func (s *Service) ListHouses(ctx context.Context, q *HouseQuery, p *principal.Info) ([]HouseVO, int64, error) {
	q.Normalize()
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
	}
	tx := s.scoped(ctx, &entity.House{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.RootOnly {
		tx = tx.Where("node_id = 0")
	}
	if q.NodeID.Uint64() != 0 {
		includeSub := true
		if q.IncludeSub != nil {
			includeSub = *q.IncludeSub
		}
		if includeSub {
			var node entity.StructureNode
			if err := s.Repo.DB.WithContext(ctx).Where("id = ?", q.NodeID.Uint64()).First(&node).Error; err == nil {
				var ids []uint64
				_ = s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).
					Where("community_id = ? AND (id = ? OR path LIKE ?)", node.CommunityID, node.ID, node.Path+",%").
					Pluck("id", &ids).Error
				if len(ids) == 0 {
					ids = []uint64{node.ID}
				}
				tx = tx.Where("node_id IN ?", ids)
			} else {
				tx = tx.Where("node_id = ?", q.NodeID.Uint64())
			}
		} else {
			tx = tx.Where("node_id = ?", q.NodeID.Uint64())
		}
	}
	// 兼容旧参数（区域/楼栋/单元/楼层现在都是节点 ID）
	if q.RegionID.Uint64() != 0 {
		tx = tx.Where("region_id = ?", q.RegionID.Uint64())
	}
	if q.BuildingID.Uint64() != 0 {
		tx = tx.Where("building_id = ?", q.BuildingID.Uint64())
	}
	if q.UnitID.Uint64() != 0 {
		tx = tx.Where("unit_id = ?", q.UnitID.Uint64())
	}
	if q.FloorID.Uint64() != 0 {
		tx = tx.Where("floor_id = ?", q.FloorID.Uint64())
	}
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(code LIKE ? OR house_no LIKE ? OR full_name LIKE ?)", kw+"%", kw+"%", kw+"%")
	}
	if q.UsageType != nil {
		tx = tx.Where("usage_type = ?", *q.UsageType)
	}
	if q.HouseStatus != nil {
		tx = tx.Where("house_status = ?", *q.HouseStatus)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.House, 0, q.PageSize)
	if err := tx.Order("node_id ASC, floor_no ASC, house_no ASC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return s.fillHouseNames(ctx, list), total, nil
}

// fillHouseNames 批量补充房屋的节点/类型名称（避免 N+1）。
func (s *Service) fillHouseNames(ctx context.Context, list []entity.House) []HouseVO {
	out := make([]HouseVO, 0, len(list))
	if len(list) == 0 {
		return out
	}
	nodeIDs := make([]uint64, 0, len(list))
	typeIDs := make([]uint64, 0, len(list))
	for _, h := range list {
		nodeIDs = append(nodeIDs, h.NodeID)
		if h.HouseTypeID != 0 {
			typeIDs = append(typeIDs, h.HouseTypeID)
		}
	}
	var nodes []entity.StructureNode
	if len(nodeIDs) > 0 {
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).Where("id IN ?", nodeIDs).Find(&nodes).Error
	}
	nameByID := make(map[uint64]string, len(nodes))
	nodeByID := make(map[uint64]entity.StructureNode, len(nodes))
	for _, n := range nodes {
		nameByID[n.ID] = n.Name
		nodeByID[n.ID] = n
	}
	// 补上级名称（楼栋/单元用于展示）
	parentIDs := make([]uint64, 0)
	for _, n := range nodes {
		if n.ParentID != 0 {
			parentIDs = append(parentIDs, n.ParentID)
		}
	}
	parentNames := s.nameMap(ctx, &entity.StructureNode{}, parentIDs)
	typeNames := s.nameMap(ctx, &entity.HouseType{}, typeIDs)
	// 楼栋/单元名称：按路径中的 node_type 回填，供旧页面/报表使用
	buildNames := map[uint64]string{}
	unitNames := map[uint64]string{}
	allPathIDs := make([]uint64, 0, len(nodes)*3)
	for _, n := range nodes {
		allPathIDs = append(allPathIDs, parsePathIDs(n.Path)...)
	}
	if len(allPathIDs) > 0 {
		var chain []entity.StructureNode
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).Where("id IN ?", allPathIDs).Find(&chain).Error
		for _, c := range chain {
			switch c.NodeType {
			case 2:
				buildNames[c.ID] = c.Name
			case 3:
				unitNames[c.ID] = c.Name
			}
		}
	}
	pathNameByID := make(map[uint64]string, len(nodes)+len(allPathIDs))
	for _, n := range nodes {
		pathNameByID[n.ID] = n.Name
	}
	for _, h := range list {
		vo := HouseVO{House: h}
		if n, ok := nodeByID[h.NodeID]; ok {
			vo.NodeName = n.Name
			vo.NodePath = s.nodeFullName(nameByID, n)
			if n.ParentID != 0 {
				vo.UnitName = parentNames[n.ParentID]
			}
			// 楼栋/单元名称按路径回填
			for _, segID := range parsePathIDs(n.Path) {
				if name, ok := buildNames[segID]; ok {
					vo.BuildingName = name
				}
				if name, ok := unitNames[segID]; ok {
					vo.UnitName = name
				}
			}
		}
		vo.HouseTypeName = typeNames[h.HouseTypeID]
		out = append(out, vo)
	}
	return out
}

// HouseReq 房屋入参（nodeId 为新模型主键；兼容旧字段 buildingId/unitId/floorId）。
type HouseReq struct {
	CommunityID    jsonx.Uint64 `json:"communityId"`
	NodeID         jsonx.Uint64 `json:"nodeId"`
	BuildingID     jsonx.Uint64 `json:"buildingId"`
	UnitID         jsonx.Uint64 `json:"unitId"`
	FloorID        jsonx.Uint64 `json:"floorId"`
	FloorNo        int          `json:"floorNo"`
	HouseTypeID    jsonx.Uint64 `json:"houseTypeId"`
	Code           string       `json:"code" binding:"required,max=64"`
	HouseNo        string       `json:"houseNo" binding:"required,max=64"`
	UsageType      int          `json:"usageType"`
	AreaBuild      float64      `json:"areaBuild"`
	AreaUse        float64      `json:"areaUse"`
	AreaShare      float64      `json:"areaShare"`
	CeilingHeight  float64      `json:"ceilingHeight"`
	PropertyRight  int          `json:"propertyRight"`
	IsStreet       bool         `json:"isStreet"`
	ShopWidth      float64      `json:"shopWidth"`
	IsSale         bool         `json:"isSale"`
	Facilities     string       `json:"facilities" binding:"max=500"`
	HandoverStatus int          `json:"handoverStatus"`
	Orientation    string       `json:"orientation" binding:"max=20"`
	Layout         string       `json:"layout" binding:"max=64"`
	Decoration     int          `json:"decoration"`
	HouseStatus    int          `json:"houseStatus"`
	DeliveryDate   string       `json:"deliveryDate"`
	Sort           int          `json:"sort"`
	Remark         string       `json:"remark" binding:"max=500"`
}

// CreateHouse 新增房屋（必须挂到某个层级节点）。
func (s *Service) CreateHouse(ctx context.Context, req *HouseReq, p *principal.Info) (uint64, error) {
	nodeID := req.NodeID.Uint64()
	if nodeID == 0 {
		nodeID = firstNonZeroU(req.FloorID.Uint64(), req.UnitID.Uint64(), req.BuildingID.Uint64())
	}
	var node *entity.StructureNode
	var com *entity.Community
	var err error
	if nodeID != 0 {
		node, err = s.nodeScoped(ctx, p, nodeID)
		if err != nil {
			return 0, err
		}
		com, err = s.findCommunity(ctx, p, node.CommunityID)
		if err != nil {
			return 0, err
		}
	} else {
		// 直接挂在小区的房屋
		if req.CommunityID.Uint64() == 0 {
			return 0, errs.Invalid("请选择小区")
		}
		if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
			return 0, err
		}
		com, err = s.findCommunity(ctx, p, req.CommunityID.Uint64())
		if err != nil {
			return 0, err
		}
	}
	h, err := s.buildHouse(ctx, node, com, req, p)
	if err != nil {
		return 0, err
	}
	if err := s.Repo.DB.WithContext(ctx).Create(h).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	if node != nil {
		s.recalcNodeCounts(ctx, node.CommunityID)
	}
	s.recalcCommunityCounts(ctx, h.CommunityID)
	return h.ID, nil
}

// buildHouse 组装房屋（从节点路径推导楼栋/单元/楼层与全称）。
func (s *Service) buildHouse(ctx context.Context, node *entity.StructureNode, com *entity.Community, req *HouseReq, p *principal.Info) (*entity.House, error) {
	// 取整条路径节点，用于填充 building_id/unit_id/floor_id/full_name
	var chain []entity.StructureNode
	ids := []uint64{}
	if node != nil {
		ids = parsePathIDs(node.Path)
	}
	if len(ids) > 0 {
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).Where("id IN ?", ids).Find(&chain).Error
	}
	byID := make(map[uint64]entity.StructureNode, len(chain))
	for _, n := range chain {
		byID[n.ID] = n
	}
	var buildingID, unitID, floorID uint64
	var names []string
	for _, segID := range ids {
		n, ok := byID[segID]
		if !ok {
			continue
		}
		names = append(names, n.Name)
		switch n.NodeType {
		case 1:
			// 区域暂不单独存储（保留在 node 上）
		case 2:
			buildingID = n.ID
		case 3:
			unitID = n.ID
		case 4:
			floorID = n.ID
		}
	}
	if buildingID == 0 {
		// 没有楼栋节点时，取路径上第一个非区域节点作为"楼栋"引用
		for _, segID := range ids {
			if n, ok := byID[segID]; ok && n.NodeType != 1 {
				buildingID = n.ID
				break
			}
		}
	}
	var tenantID, communityID, nodeID uint64
	baseAddr := communityBaseAddress(com) + com.Name
	if node != nil {
		tenantID, communityID, nodeID = node.TenantID, node.CommunityID, node.ID
		if node.FullAddress != "" {
			baseAddr = node.FullAddress
		}
	} else if com != nil {
		tenantID, communityID = com.TenantID, com.ID
	}
	if communityID == 0 {
		return nil, errs.Invalid("房屋必须属于某个小区")
	}
	var houseTypeName string
	if req.HouseTypeID.Uint64() != 0 {
		typeNames := s.nameMap(ctx, &entity.HouseType{}, []uint64{req.HouseTypeID.Uint64()})
		houseTypeName = typeNames[req.HouseTypeID.Uint64()]
	}
	delivery, err := parseDate(req.DeliveryDate)
	if err != nil {
		return nil, err
	}
	floorNo := req.FloorNo
	if floorNo == 0 && node != nil && node.NodeType == 4 {
		floorNo = node.FloorNo
	}
	now := time.Now()
	fullName := strings.Join(names, "")
	if fullName == "" && node != nil {
		fullName = node.Name
	}
	if node != nil && node.FullAddress != "" {
		fullName = node.FullAddress
	} else {
		fullName = baseAddr
	}
	fullName += strings.TrimSpace(req.HouseNo)
	return &entity.House{
		Base:           entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:       tenantID,
		CommunityID:    communityID,
		NodeID:         nodeID,
		BuildingID:     buildingID,
		UnitID:         unitID,
		FloorID:        floorID,
		FloorNo:        floorNo,
		HouseTypeID:    req.HouseTypeID.Uint64(),
		Code:           strings.TrimSpace(req.Code),
		HouseNo:        strings.TrimSpace(req.HouseNo),
		FullName:       fullName,
		UsageType:      pmDefault(req.UsageType, 1),
		AreaBuild:      req.AreaBuild,
		AreaUse:        req.AreaUse,
		AreaShare:      req.AreaShare,
		CeilingHeight:  req.CeilingHeight,
		PropertyRight:  pmDefault(req.PropertyRight, 1),
		IsStreet:       req.IsStreet,
		ShopWidth:      req.ShopWidth,
		IsSale:         req.IsSale,
		Facilities:     req.Facilities,
		HandoverStatus: pmDefault(req.HandoverStatus, 1),
		Orientation:    req.Orientation,
		Layout:         defaultIfEmpty(req.Layout, houseTypeName),
		Decoration:     req.Decoration,
		HouseStatus:    pmDefault(req.HouseStatus, 1),
		DeliveryDate:   delivery,
		Sort:           req.Sort,
	}, nil
}

// UpdateHouse 修改房屋。
func (s *Service) UpdateHouse(ctx context.Context, houseID uint64, req *HouseReq, p *principal.Info) error {
	h, err := s.findHouse(ctx, p, houseID)
	if err != nil {
		return err
	}
	nodeID := req.NodeID.Uint64()
	if nodeID == 0 {
		nodeID = h.NodeID
	}
	var node *entity.StructureNode
	var com *entity.Community
	if nodeID != 0 {
		node, err = s.nodeScoped(ctx, p, nodeID)
		if err != nil {
			return err
		}
	} else {
		com, err = s.findCommunity(ctx, p, h.CommunityID)
		if err != nil {
			return err
		}
	}
	if node != nil {
		com, err = s.findCommunity(ctx, p, node.CommunityID)
		if err != nil {
			return err
		}
	}
	built, err := s.buildHouse(ctx, node, com, req, p)
	if err != nil {
		return err
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.House{}).Where("id = ?", h.ID).Updates(map[string]any{
		"node_id":         built.NodeID,
		"building_id":     built.BuildingID,
		"unit_id":         built.UnitID,
		"floor_id":        built.FloorID,
		"floor_no":        built.FloorNo,
		"house_type_id":   built.HouseTypeID,
		"code":            built.Code,
		"house_no":        built.HouseNo,
		"full_name":       built.FullName,
		"usage_type":      built.UsageType,
		"area_build":      built.AreaBuild,
		"area_use":        built.AreaUse,
		"area_share":      built.AreaShare,
		"ceiling_height":  built.CeilingHeight,
		"property_right":  built.PropertyRight,
		"is_street":       built.IsStreet,
		"shop_width":      built.ShopWidth,
		"is_sale":         built.IsSale,
		"facilities":      built.Facilities,
		"handover_status": built.HandoverStatus,
		"orientation":     built.Orientation,
		"layout":          built.Layout,
		"decoration":      built.Decoration,
		"house_status":    built.HouseStatus,
		"delivery_date":   built.DeliveryDate,
		"sort":            built.Sort,
		"remark":          req.Remark,
		"update_by":       p.UserID,
		"update_time":     time.Now(),
	}).Error; err != nil {
		return wrapTxErr(err)
	}
	s.recalcNodeCounts(ctx, h.CommunityID)
	return nil
}

// DeleteHouse 删除房屋（有在册住户/未结账单时拒绝）。
func (s *Service) DeleteHouse(ctx context.Context, houseID uint64, p *principal.Info) error {
	h, err := s.findHouse(ctx, p, houseID)
	if err != nil {
		return err
	}
	var n int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).Where("house_id = ? AND status = 1", h.ID).Count(&n).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if n > 0 {
		return errs.Invalid("该房屋还有在册住户，请先解除住户关系")
	}
	var bills int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Bill{}).Where("house_id = ? AND status IN (1,2)", h.ID).Count(&bills).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if bills > 0 {
		return errs.Invalid("该房屋还有未结清账单，不能删除")
	}
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.House{}, h.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	s.recalcNodeCounts(ctx, h.CommunityID)
	s.recalcCommunityCounts(ctx, h.CommunityID)
	return nil
}

// BatchHouseReq 批量生成房屋。
type BatchHouseReq struct {
	NodeID         jsonx.Uint64 `json:"nodeId"`
	BuildingID     jsonx.Uint64 `json:"buildingId"`
	UnitID         jsonx.Uint64 `json:"unitId"`
	Count          int          `json:"count"`
	StartNo        int          `json:"startNo"`
	FloorFrom      int          `json:"floorFrom"`
	FloorTo        int          `json:"floorTo"`
	HousesPerFloor int          `json:"housesPerFloor"`
	UsageType      int          `json:"usageType"`
	AreaBuild      float64      `json:"areaBuild"`
	HouseTypeID    jsonx.Uint64 `json:"houseTypeId"`
	CodePrefix     string       `json:"codePrefix"`
}

// BatchCreateHouses 批量生成房屋。
//
// 两种用法：
//   - 新模型：传 nodeId + count + startNo（在指定层级下顺序生成 N 套）；
//   - 旧模型：传 buildingId/unitId + floorFrom/floorTo + housesPerFloor（按楼层×户数生成，自动建楼层节点）。
func (s *Service) BatchCreateHouses(ctx context.Context, req *BatchHouseReq, p *principal.Info) (int, error) {
	nodeID := req.NodeID.Uint64()
	if nodeID == 0 {
		nodeID = firstNonZeroU(req.UnitID.Uint64(), req.BuildingID.Uint64())
	}
	if nodeID == 0 {
		return 0, errs.Invalid("请选择房屋所属层级")
	}
	node, err := s.nodeScoped(ctx, p, nodeID)
	if err != nil {
		return 0, err
	}
	// 旧模型：按楼层区间生成
	if req.FloorTo >= req.FloorFrom && req.HousesPerFloor > 0 {
		total := 0
		for floor := req.FloorFrom; floor <= req.FloorTo; floor++ {
			fnode, err := s.ensureFloorNode(ctx, node, floor, p)
			if err != nil {
				return total, err
			}
			req2 := *req
			if req2.CodePrefix == "" {
				req2.CodePrefix = node.Code
			}
			n, err := s.batchHousesAt(ctx, fnode, &req2, floor, p)
			if err != nil {
				return total, err
			}
			total += n
		}
		s.recalcNodeCounts(ctx, node.CommunityID)
		s.recalcCommunityCounts(ctx, node.CommunityID)
		return total, nil
	}
	// 新模型：顺序生成
	count := req.Count
	if count <= 0 {
		return 0, errs.Invalid("请填写生成数量")
	}
	if count > 5000 {
		return 0, errs.Invalid("单次最多生成 5000 套")
	}
	start := req.StartNo
	if start <= 0 {
		start = 1
	}
	prefix := req.CodePrefix
	if prefix == "" {
		prefix = node.Code
	}
	now := time.Now()
	houses := make([]*entity.House, 0, count)
	for i := 0; i < count; i++ {
		no := fmt.Sprintf("%03d", start+i)
		houses = append(houses, &entity.House{
			Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now},
			TenantID:    node.TenantID,
			CommunityID: node.CommunityID,
			NodeID:      node.ID,
			BuildingID:  node.ID,
			Code:        prefix + "-" + no,
			HouseNo:     no,
			FullName:    node.Name + no,
			UsageType:   pmDefault(req.UsageType, 1),
			AreaBuild:   req.AreaBuild,
			HouseTypeID: req.HouseTypeID.Uint64(),
			HouseStatus: 1,
			Sort:        start + i,
		})
	}
	if err := s.Repo.DB.WithContext(ctx).CreateInBatches(houses, 500).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	s.recalcNodeCounts(ctx, node.CommunityID)
	s.recalcCommunityCounts(ctx, node.CommunityID)
	return len(houses), nil
}

// batchHousesAt 在楼层节点下按"每层户数"生成房屋（同时回填楼栋/单元引用，兼容旧查询）。
func (s *Service) batchHousesAt(ctx context.Context, floorNode *entity.StructureNode, req *BatchHouseReq, floor int, p *principal.Info) (int, error) {
	prefix := req.CodePrefix
	if prefix == "" {
		prefix = floorNode.Code
	}
	// 从层级路径解析楼栋/单元节点
	var buildingID, unitID uint64
	pathIDs := parsePathIDs(floorNode.Path)
	if len(pathIDs) > 0 {
		var chain []entity.StructureNode
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).Where("id IN ?", pathIDs).Find(&chain).Error
		for _, n := range chain {
			switch n.NodeType {
			case 2:
				buildingID = n.ID
			case 3:
				unitID = n.ID
			}
		}
	}
	now := time.Now()
	houses := make([]*entity.House, 0, req.HousesPerFloor)
	for i := 1; i <= req.HousesPerFloor; i++ {
		no := fmt.Sprintf("%d%02d", floor, i)
		houses = append(houses, &entity.House{
			Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now},
			TenantID:    floorNode.TenantID,
			CommunityID: floorNode.CommunityID,
			NodeID:      floorNode.ID,
			BuildingID:  buildingID,
			UnitID:      unitID,
			FloorID:     floorNode.ID,
			FloorNo:     floor,
			Code:        prefix + "-" + no,
			HouseNo:     no,
			FullName:    floorNode.Name + no,
			UsageType:   pmDefault(req.UsageType, 1),
			AreaBuild:   req.AreaBuild,
			HouseTypeID: req.HouseTypeID.Uint64(),
			HouseStatus: 1,
			Sort:        floor*100 + i,
		})
	}
	if err := s.Repo.DB.WithContext(ctx).CreateInBatches(houses, 500).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	return len(houses), nil
}

// ensureFloorNode 确保楼层节点存在（旧模型批量生成用）。
func (s *Service) ensureFloorNode(ctx context.Context, parent *entity.StructureNode, floor int, p *principal.Info) (*entity.StructureNode, error) {
	code := fmt.Sprintf("%s-F%d", parent.Code, floor)
	var node entity.StructureNode
	err := s.Repo.DB.WithContext(ctx).Where("community_id = ? AND parent_id = ? AND floor_no = ? AND node_type = 4",
		parent.CommunityID, parent.ID, floor).First(&node).Error
	if err == nil {
		return &node, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errs.ErrDB.WithErr(err)
	}
	now := time.Now()
	nodeID := id.Next()
	node = entity.StructureNode{
		Base:        entity.Base{ID: nodeID, CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now},
		TenantID:    parent.TenantID,
		CommunityID: parent.CommunityID,
		ParentID:    parent.ID,
		Code:        code,
		Name:        floorName(floor),
		NodeType:    4,
		LevelNo:     parent.LevelNo + 1,
		Path:        fmt.Sprintf("%s,%d", parent.Path, nodeID),
		FloorNo:     floor,
		Sort:        floor,
		Status:      1,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(&node).Error; err != nil {
		if e2 := s.Repo.DB.WithContext(ctx).Where("community_id = ? AND parent_id = ? AND floor_no = ? AND node_type = 4",
			parent.CommunityID, parent.ID, floor).First(&node).Error; e2 == nil {
			return &node, nil
		}
		return nil, wrapTxErr(err)
	}
	return &node, nil
}

// recalcNodeCounts 重算节点房屋数（直接挂载 + 子节点汇总）。
func (s *Service) recalcNodeCounts(ctx context.Context, communityID uint64) {
	var nodes []entity.StructureNode
	if err := s.Repo.DB.WithContext(ctx).Where("community_id = ?", communityID).Find(&nodes).Error; err != nil {
		return
	}
	var rows []struct {
		NodeID uint64
		Count  int
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.House{}).
		Select("node_id, COUNT(*) AS count").Where("community_id = ? AND node_id <> 0", communityID).
		Group("node_id").Scan(&rows).Error
	direct := make(map[uint64]int, len(rows))
	for _, r := range rows {
		direct[r.NodeID] = r.Count
	}
	// 自底向上汇总：按 level_no 倒序
	sum := make(map[uint64]int, len(nodes))
	for _, n := range nodes {
		sum[n.ID] += direct[n.ID]
	}
	byID := make(map[uint64]entity.StructureNode, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	for i := len(nodes) - 1; i >= 0; i-- {
		n := nodes[i]
		if n.ParentID != 0 {
			sum[n.ParentID] += sum[n.ID]
		}
	}
	for idv, v := range sum {
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).Where("id = ?", idv).
			UpdateColumn("house_count", v).Error
	}
}

// recalcCommunityCounts 重算小区楼栋/房屋数。
func (s *Service) recalcCommunityCounts(ctx context.Context, communityID uint64) {
	if communityID == 0 {
		return
	}
	var buildings, houses int64
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).
		Where("community_id = ? AND node_type = 2", communityID).Count(&buildings).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.House{}).Where("community_id = ?", communityID).Count(&houses).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Community{}).Where("id = ?", communityID).
		Updates(map[string]any{"building_count": buildings, "house_count": houses}).Error
}

// parsePathIDs 解析 path（"0,1,2"）为 ID 数组（去掉根 0）。
func parsePathIDs(path string) []uint64 {
	parts := strings.Split(path, ",")
	out := make([]uint64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "0" {
			continue
		}
		var v uint64
		if _, err := fmt.Sscanf(p, "%d", &v); err == nil && v != 0 {
			out = append(out, v)
		}
	}
	return out
}

// nameMap 通用 ID → name 映射（一次查询，避免 N+1）。
func (s *Service) nameMap(ctx context.Context, m any, ids []uint64) map[uint64]string {
	out := make(map[uint64]string, len(ids))
	if len(ids) == 0 {
		return out
	}
	var rows []struct {
		ID   uint64
		Name string
	}
	if err := s.Repo.DB.WithContext(ctx).Model(m).Select("id, name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		out[r.ID] = r.Name
	}
	return out
}

// nodeBelongsTo 校验层级节点属于指定小区。
func (s *Service) nodeBelongsTo(ctx context.Context, communityID, nodeID uint64) (*entity.StructureNode, error) {
	var n entity.StructureNode
	if err := s.Repo.DB.WithContext(ctx).Where("id = ? AND community_id = ?", nodeID, communityID).First(&n).Error; err != nil {
		return nil, errs.Invalid("层级不存在或不属于当前小区")
	}
	return &n, nil
}

// firstNonZeroU 返回第一个非 0 值。
func firstNonZeroU(ids ...uint64) uint64 {
	for _, v := range ids {
		if v != 0 {
			return v
		}
	}
	return 0
}

// SpaceSummary 小区基础信息概览（楼栋/单元/楼层/房屋与面积汇总）。
type SpaceSummary struct {
	Buildings      int64            `json:"buildings"`
	Units          int64            `json:"units"`
	Floors         int64            `json:"floors"`
	Houses         int64            `json:"houses"`
	TotalBuildArea float64          `json:"totalBuildArea"`
	TotalHouseArea float64          `json:"totalHouseArea"`
	ByUsage        map[string]int64 `json:"byUsage"`
}

// Summary 统计小区空间与房屋。
func (s *Service) SpaceSummary(ctx context.Context, communityID uint64, p *principal.Info) (*SpaceSummary, error) {
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	out := &SpaceSummary{ByUsage: map[string]int64{}}
	var nodeRows []struct {
		NodeType int
		Count    int64
		Area     float64
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).
		Select("node_type, COUNT(*) AS count, IFNULL(SUM(build_area),0) AS area").
		Where("community_id = ?", communityID).Group("node_type").Scan(&nodeRows).Error
	for _, r := range nodeRows {
		switch r.NodeType {
		case 2:
			out.Buildings += r.Count
		case 3:
			out.Units += r.Count
		case 4:
			out.Floors += r.Count
		}
		out.TotalBuildArea += r.Area
	}
	var houseRows []struct {
		UsageType int
		Count     int64
		Area      float64
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.House{}).
		Select("usage_type, COUNT(*) AS count, IFNULL(SUM(area_build),0) AS area").
		Where("community_id = ?", communityID).Group("usage_type").Scan(&houseRows).Error
	for _, r := range houseRows {
		out.Houses += r.Count
		out.TotalHouseArea += r.Area
		switch r.UsageType {
		case 1:
			out.ByUsage["residential"] += r.Count
		case 2:
			out.ByUsage["commercial"] += r.Count
		case 3:
			out.ByUsage["office"] += r.Count
		case 4:
			out.ByUsage["parking"] += r.Count
		default:
			out.ByUsage["other"] += r.Count
		}
	}
	out.TotalBuildArea = round2(out.TotalBuildArea)
	out.TotalHouseArea = round2(out.TotalHouseArea)
	return out, nil
}

// spaceCategoryText 空间类别 → 文本。
func spaceCategoryText(c int) string {
	switch c {
	case 1:
		return "住宅楼"
	case 2:
		return "别墅"
	case 3:
		return "商业楼"
	case 4:
		return "办公楼"
	case 5:
		return "车库/车位"
	case 6:
		return "配套设施"
	default:
		return "其他"
	}
}

// houseUsageText 房屋用途 → 文本。
func houseUsageText(u int) string {
	switch u {
	case 1:
		return "住宅"
	case 2:
		return "商业"
	case 3:
		return "办公"
	case 4:
		return "车位"
	default:
		return "其他"
	}
}

// propertyRightText 产权性质 → 文本。
func propertyRightText(v int) string {
	switch v {
	case 1:
		return "商品房"
	case 2:
		return "安置房"
	case 3:
		return "公房"
	case 4:
		return "自建"
	default:
		return "其他"
	}
}

func parseSpaceCategory(v string) int {
	switch strings.TrimSpace(v) {
	case "住宅楼", "住宅", "1":
		return 1
	case "别墅", "2":
		return 2
	case "商业楼", "商业", "3":
		return 3
	case "办公楼", "办公", "4":
		return 4

	case "配套", "配套设施", "6":
		return 6
	default:
		return 9
	}
}

func parsePropertyRight(v string) int {
	switch strings.TrimSpace(v) {
	case "安置房", "2":
		return 2
	case "公房", "3":
		return 3
	case "自建", "4":
		return 4
	case "其他", "9":
		return 9
	default:
		return 1
	}
}

func decorationText(v int) string {
	switch v {
	case 1:
		return "毛坯"
	case 2:
		return "简装"
	case 3:
		return "精装"
	default:
		return ""
	}
}

func houseStatusText(v int) string {
	switch v {
	case 2:
		return "自住"
	case 3:
		return "出租"
	case 4:
		return "装修中"
	case 5:
		return "停用"
	default:
		return "空置"
	}
}

func yesNo(v bool) string {
	if v {
		return "是"
	}
	return "否"
}

func formatDateOnly(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}
