// 小区基础信息：层级 / 房屋 / 小区 三级的"信息卡片"数据。
//
// 语义：
//   - 层级卡片：名称、范围（下一级名称）、房屋总数（含所有子层级）、住户人数（含子层级房屋）、地址、关联设备（含子层级）
//   - 房屋卡片：房屋基础信息 + 在册住户 + 登记车辆
//   - 小区卡片：小区信息 + 楼栋/房屋/住户/设备规模
package service

import (
	"context"

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
