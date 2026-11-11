// 小区基础信息：层级 / 房屋 / 小区 三级的"信息卡片"数据。
//
// 语义：
//   - 层级卡片：名称、范围（下一级名称）、房屋总数（含所有子层级）、住户人数（含子层级房屋）、地址、关联设备（含子层级）
//   - 房屋卡片：房屋基础信息 + 在册住户 + 登记车辆
//   - 小区卡片：小区信息 + 楼栋/房屋/住户/设备规模
package service

import (
	"context"
	"gorm.io/gorm"

	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/pkg/errs"
	"web-basic/pkg/jsonx"
)

// NodeOverview 层级信息卡片。
type NodeOverview struct {
	ID            uint64   `json:"id,string"`
	Name          string   `json:"name"`
	Type          int      `json:"type"`
	TypeName      string   `json:"typeName"`
	ParentID      uint64   `json:"parentId,string"`
	ParentName    string   `json:"parentName"`
	Address       string   `json:"address"`
	ChildNames    []string `json:"childNames"`
	HouseTotal    int64    `json:"houseTotal"`
	ResidentCount int64    `json:"residentCount"`
	Devices       []string `json:"devices"`
	DeviceTotal   int64    `json:"deviceTotal"`
	PathNames     []string `json:"pathNames"`
}

// HouseOverview 房屋信息卡片（含住户与车辆）。
type HouseOverview struct {
	House     HouseVO         `json:"house"`
	Address   string          `json:"address"`
	Residents []HouseResident `json:"residents"`
	Vehicles  []HouseVehicle  `json:"vehicles"`
}

// HouseResident 房屋在册住户。
type HouseResident struct {
	ResidentID uint64 `json:"residentId,string"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Role       int    `json:"role"`
	RoleName   string `json:"roleName"`
	Relation   string `json:"relation"`
	IsMain     bool   `json:"isMain"`
	StartDate  string `json:"startDate"`
	EndDate    string `json:"endDate"`
}

// HouseVehicle 房屋登记车辆。
type HouseVehicle struct {
	VehicleID       uint64 `json:"vehicleId,string"`
	PlateNo         string `json:"plateNo"`
	PlateColor      int    `json:"plateColor"`
	VehicleType     int    `json:"vehicleType"`
	VehicleTypeName string `json:"vehicleTypeName"`
	ParkingSpaceNo  string `json:"parkingSpaceNo"`
	OwnerType       int    `json:"ownerType"`
}

// CommunityOverview 小区信息卡片。
type SpaceCommunityCard struct {
	ID           uint64  `json:"id,string"`
	Name         string  `json:"name"`
	Code         string  `json:"code"`
	Address      string  `json:"address"`
	ManagerName  string  `json:"managerName"`
	ServicePhone string  `json:"servicePhone"`
	Buildings    int64   `json:"buildings"`
	Houses       int64   `json:"houses"`
	Residents    int64   `json:"residents"`
	Devices      int64   `json:"devices"`
	BuildArea    float64 `json:"buildArea"`
	HouseArea    float64 `json:"houseArea"`
}

// nodeSubtreeIDs 返回节点自身 + 所有后代节点 ID。
func (s *Service) nodeSubtreeIDs(ctx context.Context, node *entity.StructureNode) []uint64 {
	ids := []uint64{node.ID}
	var rows []entity.StructureNode
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).
		Select("id").Where("community_id = ? AND path LIKE ?", node.CommunityID, node.Path+",%").
		Find(&rows).Error
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids
}

// NodeCard 层级信息卡片。
func (s *Service) NodeCard(ctx context.Context, nodeID uint64, p *principal.Info) (*NodeOverview, error) {
	n, err := s.nodeScoped(ctx, p, nodeID)
	if err != nil {
		return nil, err
	}
	out := &NodeOverview{
		ID:         n.ID,
		Name:       n.Name,
		Type:       n.NodeType,
		TypeName:   nodeTypeName(n.NodeType),
		ParentID:   n.ParentID,
		Address:    n.FullAddress,
		ChildNames: []string{},
		Devices:    []string{},
		PathNames:  []string{},
	}
	if out.Address == "" {
		out.Address = n.Address
	}
	if n.ParentID != 0 {
		var parent entity.StructureNode
		if err := s.Repo.DB.WithContext(ctx).Where("id = ?", n.ParentID).First(&parent).Error; err == nil {
			out.ParentName = parent.Name
		}
	}
	// 路径名称
	ids := parsePathIDs(n.Path)
	if len(ids) > 0 {
		var chain []entity.StructureNode
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).Where("id IN ?", ids).Find(&chain).Error
		byID := map[uint64]string{}
		for _, c := range chain {
			byID[c.ID] = c.Name
		}
		for _, id := range ids {
			if name, ok := byID[id]; ok {
				out.PathNames = append(out.PathNames, name)
			}
		}
	}
	// 下一级名称
	var children []entity.StructureNode
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.StructureNode{}).
		Select("id, name").Where("community_id = ? AND parent_id = ?", n.CommunityID, n.ID).
		Order("sort ASC, id ASC").Find(&children).Error
	for _, c := range children {
		out.ChildNames = append(out.ChildNames, c.Name)
	}
	subIDs := s.nodeSubtreeIDs(ctx, n)
	// 房屋总数（含子层级）+ 住户人数
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.House{}).
		Where("community_id = ? AND node_id IN ?", n.CommunityID, subIDs).Count(&out.HouseTotal).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	_ = s.Repo.DB.WithContext(ctx).Raw(`
		SELECT COUNT(DISTINCT rh.resident_id)
		FROM pm_resident_house rh
		JOIN pm_house h ON h.id = rh.house_id AND h.deleted = 0
		WHERE rh.deleted = 0 AND rh.status = 1 AND h.community_id = ? AND h.node_id IN ?`,
		n.CommunityID, subIDs).Scan(&out.ResidentCount).Error
	// 关联设备（本层级及以下）
	var devices []entity.Device
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Device{}).
		Where("community_id = ? AND deleted = 0 AND (bind_node_id IN ? OR (bind_level > 1 AND bind_id IN ?))", n.CommunityID, subIDs, subIDs).
		Order("id ASC").Limit(200).Find(&devices).Error
	for _, d := range devices {
		out.Devices = append(out.Devices, d.Name)
	}
	out.DeviceTotal = int64(len(devices))
	return out, nil
}

// HouseCard 房屋信息卡片（住户 + 车辆）。
func (s *Service) HouseCard(ctx context.Context, houseID uint64, p *principal.Info) (*HouseOverview, error) {
	h, err := s.findHouse(ctx, p, houseID)
	if err != nil {
		return nil, err
	}
	list := s.fillHouseNames(ctx, []entity.House{*h})
	out := &HouseOverview{Residents: []HouseResident{}, Vehicles: []HouseVehicle{}}
	if len(list) > 0 {
		out.House = list[0]
	}
	out.Address = h.FullName
	// 在册住户
	var rels []entity.ResidentHouse
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).
		Where("house_id = ? AND status = 1", h.ID).Find(&rels).Error
	if len(rels) > 0 {
		rids := make([]uint64, 0, len(rels))
		for _, r := range rels {
			rids = append(rids, r.ResidentID)
		}
		var residents []entity.Resident
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.Resident{}).Where("id IN ?", rids).Find(&residents).Error
		byID := make(map[uint64]entity.Resident, len(residents))
		for _, r := range residents {
			byID[r.ID] = r
		}
		for _, rel := range rels {
			r := byID[rel.ResidentID]
			item := HouseResident{
				ResidentID: rel.ResidentID,
				Name:       r.Name,
				Phone:      r.Phone,
				Role:       rel.RelRole,
				RoleName:   residentRoleName(rel.RelRole),
				Relation:   rel.Relation,
				IsMain:     rel.IsMain,
			}
			if rel.StartDate != nil {
				item.StartDate = rel.StartDate.Format("2006-01-02")
			}
			if rel.EndDate != nil {
				item.EndDate = rel.EndDate.Format("2006-01-02")
			}
			out.Residents = append(out.Residents, item)
		}
	}
	// 车辆
	var vehicles []entity.Vehicle
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Vehicle{}).Where("house_id = ?", h.ID).Order("id ASC").Find(&vehicles).Error
	for _, v := range vehicles {
		out.Vehicles = append(out.Vehicles, HouseVehicle{
			VehicleID:       v.ID,
			PlateNo:         v.PlateNo,
			PlateColor:      v.PlateColor,
			VehicleType:     v.VehicleType,
			VehicleTypeName: vehicleTypeName(v.VehicleType),
			ParkingSpaceNo:  v.ParkingSpaceNo,
			OwnerType:       v.OwnerType,
		})
	}
	return out, nil
}

// CommunityCard 小区信息卡片。
func (s *Service) CommunityCard(ctx context.Context, communityID uint64, p *principal.Info) (*SpaceCommunityCard, error) {
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	com, err := s.findCommunity(ctx, p, communityID)
	if err != nil {
		return nil, err
	}
	sum, err := s.SpaceSummary(ctx, communityID, p)
	if err != nil {
		return nil, err
	}
	out := &SpaceCommunityCard{
		ID:           com.ID,
		Name:         com.Name,
		Code:         com.Code,
		Address:      communityBaseAddress(com),
		ManagerName:  com.ManagerName,
		ServicePhone: com.ServicePhone,
		Buildings:    sum.Buildings,
		Houses:       sum.Houses,
		BuildArea:    sum.TotalBuildArea,
		HouseArea:    sum.TotalHouseArea,
	}
	_ = s.Repo.DB.WithContext(ctx).Raw(`
		SELECT COUNT(DISTINCT rh.resident_id)
		FROM pm_resident_house rh
		JOIN pm_house h ON h.id = rh.house_id AND h.deleted = 0
		WHERE rh.deleted = 0 AND rh.status = 1 AND h.community_id = ?`, communityID).
		Scan(&out.Residents).Error
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Device{}).
		Where("community_id = ? AND deleted = 0", communityID).Count(&out.Devices).Error
	return out, nil
}

func residentRoleName(role int) string {
	switch role {
	case 1:
		return "业主"
	case 2:
		return "家属"
	case 3:
		return "租户"
	default:
		return "其他"
	}
}

func vehicleTypeName(t int) string {
	switch t {
	case 1:
		return "小型车"
	case 2:
		return "大型车"
	case 3:
		return "新能源"
	default:
		return "其他"
	}
}

// NodeChildrenWithHouses 层级下的"子层级 + 直接挂载的房屋"，用于树懒加载。
type TreeNodeChild struct {
	RowType string   `json:"rowType"` // node / house
	Node    *NodeVO  `json:"node,omitempty"`
	House   *HouseVO `json:"house,omitempty"`
}

// NodeChildren 返回某层级的子节点与直接挂载房屋；nodeID=0 表示小区根级。
func (s *Service) NodeChildren(ctx context.Context, communityID, nodeID uint64, p *principal.Info) ([]TreeNodeChild, error) {
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	out := []TreeNodeChild{}
	nodes, err := s.NodeTree(ctx, communityID, p)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		if n.ParentID == nodeID {
			out = append(out, TreeNodeChild{RowType: "node", Node: n})
		}
	}
	q := &HouseQuery{CommunityID: jsonx.Uint64(communityID), IncludeSub: BoolPtr(false)}
	q.Page = 1
	q.PageSize = 500
	if nodeID == 0 {
		q.RootOnly = true
	} else {
		q.NodeID = jsonx.Uint64(nodeID)
	}
	houses, _, err := s.ListHouses(ctx, q, p)
	if err != nil {
		return nil, err
	}
	for i := range houses {
		out = append(out, TreeNodeChild{RowType: "house", House: &houses[i]})
	}
	return out, nil
}

// BoolPtr 便捷布尔指针。
func BoolPtr(v bool) *bool { return &v }

// ClearCommunitySpace 清空小区下所有"层级 + 房屋"（逻辑删除），用于导入出错后的整体重置。
//
// 为避免误删业务数据，存在以下数据时会拒绝执行：
// 在册住户关系、未结清账单、车辆、单位商铺、访客登记。
func (s *Service) ClearCommunitySpace(ctx context.Context, communityID uint64, p *principal.Info) (map[string]int64, error) {
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
	// 先统计房屋 ID 范围（含直接挂小区的）
	var houseIDs []uint64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.House{}).
		Where("community_id = ?", communityID).Pluck("id", &houseIDs).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if len(houseIDs) > 0 {
		var residents int64
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.ResidentHouse{}).
			Where("house_id IN ? AND status = 1", houseIDs).Count(&residents).Error
		if residents > 0 {
			return nil, errs.Invalid("该小区已有在册住户，不能清空层级与房屋，请先解绑住户")
		}
		var bills int64
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.Bill{}).
			Where("house_id IN ? AND status IN (1,2)", houseIDs).Count(&bills).Error
		if bills > 0 {
			return nil, errs.Invalid("该小区还有未结清账单，不能清空层级与房屋")
		}
		var vehicles int64
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.Vehicle{}).
			Where("house_id IN ?", houseIDs).Count(&vehicles).Error
		if vehicles > 0 {
			return nil, errs.Invalid("该小区还有车辆登记，不能清空层级与房屋")
		}
		var merchants int64
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.Merchant{}).
			Where("house_id IN ?", houseIDs).Count(&merchants).Error
		if merchants > 0 {
			return nil, errs.Invalid("该小区还有单位商铺，不能清空层级与房屋")
		}
		var visitors int64
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.Visitor{}).
			Where("visit_house_id IN ?", houseIDs).Count(&visitors).Error
		if visitors > 0 {
			return nil, errs.Invalid("该小区还有访客登记，不能清空层级与房屋")
		}
	}
	stats := map[string]int64{}
	err = s.Repo.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 设备与层级解绑
		_ = tx.Model(&entity.Device{}).Where("community_id = ? AND bind_node_id > 0", communityID).
			Updates(map[string]any{"bind_node_id": 0, "bind_name": ""}).Error
		// 逻辑删除房屋
		h := tx.Where("community_id = ?", communityID).Delete(&entity.House{})
		if h.Error != nil {
			return h.Error
		}
		stats["houses"] = h.RowsAffected
		// 逻辑删除层级
		n := tx.Where("community_id = ?", communityID).Delete(&entity.StructureNode{})
		if n.Error != nil {
			return n.Error
		}
		stats["nodes"] = n.RowsAffected
		return nil
	})
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Community{}).Where("id = ?", communityID).
		Updates(map[string]any{"building_count": 0, "house_count": 0}).Error
	return stats, nil
}

// CascaderNode 房屋级联选择节点（层级 + 房屋叶子）。
type CascaderNode struct {
	Value    string          `json:"value"`
	Label    string          `json:"label"`
	IsHouse  bool            `json:"isHouse"`
	Children []*CascaderNode `json:"children,omitempty"`
}

// HouseCascaderTree 返回"小区层级 + 房屋"的级联树，供单位/车辆/住户/访客等页面选房。
// usageType>0 时只返回该用途的房屋（如单位管理只看商业房屋）。
func (s *Service) HouseCascaderTree(ctx context.Context, communityID uint64, usageType int, p *principal.Info) ([]*CascaderNode, error) {
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	var nodes []entity.StructureNode
	if err := s.Repo.DB.WithContext(ctx).Where("community_id = ?", communityID).
		Order("level_no ASC, sort ASC, id ASC").Find(&nodes).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	hq := s.Repo.DB.WithContext(ctx).Model(&entity.House{}).Where("community_id = ?", communityID)
	if usageType > 0 {
		hq = hq.Where("usage_type = ?", usageType)
	}
	var houses []entity.House
	if err := hq.Order("node_id ASC, house_no ASC").Limit(20000).Find(&houses).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	nodeByID := make(map[uint64]*CascaderNode, len(nodes))
	childrenOf := make(map[uint64][]*CascaderNode, len(nodes))
	for _, n := range nodes {
		item := &CascaderNode{Value: "n-" + uintToStr(n.ID), Label: n.Name}
		nodeByID[n.ID] = item
		childrenOf[n.ParentID] = append(childrenOf[n.ParentID], item)
	}
	rootHouses := []*CascaderNode{}
	for _, h := range houses {
		label := h.HouseNo
		if label == "" {
			label = h.Code
		}
		leaf := &CascaderNode{Value: "h-" + uintToStr(h.ID), Label: label, IsHouse: true}
		if h.NodeID == 0 || nodeByID[h.NodeID] == nil {
			rootHouses = append(rootHouses, leaf)
			continue
		}
		parent := nodeByID[h.NodeID]
		parent.Children = append(parent.Children, leaf)
	}
	// 组装层级：子节点挂在父节点下
	var build func(parentID uint64) []*CascaderNode
	build = func(parentID uint64) []*CascaderNode {
		list := childrenOf[parentID]
		for _, item := range list {
			nid := parseNumericSuffix(item.Value)
			item.Children = append(item.Children, build(nid)...)
		}
		return list
	}
	roots := build(0)
	roots = append(roots, rootHouses...)
	return roots, nil
}

// uintToStr / parseNumericSuffix 级联 value 的轻量编解码。
func uintToStr(v uint64) string {
	if v == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func parseNumericSuffix(v string) uint64 {
	var out uint64
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c < '0' || c > '9' {
			continue
		}
		out = out*10 + uint64(c-'0')
	}
	return out
}
