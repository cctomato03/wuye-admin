package entity

import "time"

// 鐗╀笟绠＄悊鍩虹鍩熷疄浣擄紙瀵瑰簲 migrations/000020_property_foundation.up.sql锛夈€?
// District 琛屾斂鍖哄垝 pm_district
type District struct {
	BaseLite
	ParentID uint64 `gorm:"column:parent_id" json:"parentId,string"`
	Code     string `gorm:"column:code" json:"code"`
	Name     string `gorm:"column:name" json:"name"`
	Level    int    `gorm:"column:level" json:"level"`
	FullName string `gorm:"column:full_name" json:"fullName"`
	Sort     int    `gorm:"column:sort" json:"sort"`
	Status   int    `gorm:"column:status" json:"status"`
}

func (District) TableName() string { return "pm_district" }

// Community 灏忓尯 pm_community
type Community struct {
	Base
	TenantID        uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	Code            string     `gorm:"column:code" json:"code"`
	Name            string     `gorm:"column:name" json:"name"`
	ShortName       string     `gorm:"column:short_name" json:"shortName"`
	CommunityType   int        `gorm:"column:community_type" json:"communityType"`
	Province        string     `gorm:"column:province" json:"province"`
	City            string     `gorm:"column:city" json:"city"`
	District        string     `gorm:"column:district" json:"district"`
	Town            string     `gorm:"column:town" json:"town"`
	Village         string     `gorm:"column:village" json:"village"`
	DistrictCode    string     `gorm:"column:district_code" json:"districtCode"`
	StreetCode      string     `gorm:"column:street_code" json:"streetCode"`
	VillageCode     string     `gorm:"column:village_code" json:"villageCode"`
	Address         string     `gorm:"column:address" json:"address"`
	Longitude       float64    `gorm:"column:longitude" json:"longitude"`
	Latitude        float64    `gorm:"column:latitude" json:"latitude"`
	TotalArea       float64    `gorm:"column:total_area" json:"totalArea"`
	GreenArea       float64    `gorm:"column:green_area" json:"greenArea"`
	BuildingCount   int        `gorm:"column:building_count" json:"buildingCount"`
	HouseCount      int        `gorm:"column:house_count" json:"houseCount"`
	ResidentCount   int        `gorm:"column:resident_count" json:"residentCount"`
	Developer       string     `gorm:"column:developer" json:"developer"`
	PropertyCompany string     `gorm:"column:property_company" json:"propertyCompany"`
	ManagerName     string     `gorm:"column:manager_name" json:"managerName"`
	ManagerPhone    string     `gorm:"column:manager_phone" json:"managerPhone"`
	ServicePhone    string     `gorm:"column:service_phone" json:"servicePhone"`
	HandoverDate    *time.Time `gorm:"column:handover_date" json:"handoverDate"`
	Status          int        `gorm:"column:status" json:"status"`
	Sort            int        `gorm:"column:sort" json:"sort"`
}

func (Community) TableName() string { return "pm_community" }

// UserCommunity 鐢ㄦ埛-灏忓尯鎺堟潈 pm_user_community
type UserCommunity struct {
	Base
	TenantID    uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
	UserID      uint64 `gorm:"column:user_id" json:"userId,string"`
	CommunityID uint64 `gorm:"column:community_id" json:"communityId,string"`
	IsDefault   bool   `gorm:"column:is_default" json:"isDefault"`
}

func (UserCommunity) TableName() string { return "pm_user_community" }

// HouseType 鎴垮眿绫诲瀷 pm_house_type
type HouseType struct {
	Base
	TenantID    uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID uint64 `gorm:"column:community_id" json:"communityId,string"`
	Code        string `gorm:"column:code" json:"code"`
	Name        string `gorm:"column:name" json:"name"`
	Layout      string `gorm:"column:layout" json:"layout"`
	UsageType   int    `gorm:"column:usage_type" json:"usageType"`
	Sort        int    `gorm:"column:sort" json:"sort"`
	Status      int    `gorm:"column:status" json:"status"`
}

func (HouseType) TableName() string { return "pm_house_type" }

// House 鎴垮眿 pm_house
type House struct {
	Base
	TenantID       uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID    uint64     `gorm:"column:community_id" json:"communityId,string"`
	RegionID       uint64     `gorm:"column:region_id" json:"regionId,string"`
	BuildingID     uint64     `gorm:"column:building_id" json:"buildingId,string"`
	UnitID         uint64     `gorm:"column:unit_id" json:"unitId,string"`
	FloorID        uint64     `gorm:"column:floor_id" json:"floorId,string"`
	NodeID         uint64     `gorm:"column:node_id" json:"nodeId,string"`
	FloorNo        int        `gorm:"column:floor_no" json:"floorNo"`
	HouseTypeID    uint64     `gorm:"column:house_type_id" json:"houseTypeId,string"`
	Code           string     `gorm:"column:code" json:"code"`
	HouseNo        string     `gorm:"column:house_no" json:"houseNo"`
	FullName       string     `gorm:"column:full_name" json:"fullName"`
	UsageType      int        `gorm:"column:usage_type" json:"usageType"`
	AreaBuild      float64    `gorm:"column:area_build" json:"areaBuild"`
	AreaUse        float64    `gorm:"column:area_use" json:"areaUse"`
	AreaShare      float64    `gorm:"column:area_share" json:"areaShare"`
	CeilingHeight  float64    `gorm:"column:ceiling_height" json:"ceilingHeight"`
	PropertyRight  int        `gorm:"column:property_right" json:"propertyRight"`
	IsStreet       bool       `gorm:"column:is_street" json:"isStreet"`
	ShopWidth      float64    `gorm:"column:shop_width" json:"shopWidth"`
	IsSale         bool       `gorm:"column:is_sale" json:"isSale"`
	Facilities     string     `gorm:"column:facilities" json:"facilities"`
	HandoverStatus int        `gorm:"column:handover_status" json:"handoverStatus"`
	Orientation    string     `gorm:"column:orientation" json:"orientation"`
	Layout         string     `gorm:"column:layout" json:"layout"`
	Decoration     int        `gorm:"column:decoration" json:"decoration"`
	HouseStatus    int        `gorm:"column:house_status" json:"houseStatus"`
	OwnerCount     int        `gorm:"column:owner_count" json:"ownerCount"`
	ResidentCount  int        `gorm:"column:resident_count" json:"residentCount"`
	DeliveryDate   *time.Time `gorm:"column:delivery_date" json:"deliveryDate"`
	Sort           int        `gorm:"column:sort" json:"sort"`
}

func (House) TableName() string { return "pm_house" }

// StructureNode 小区层级节点（自定义层级：小区为根，任意深度）pm_structure_node
type StructureNode struct {
	Base
	TenantID      uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID   uint64     `gorm:"column:community_id" json:"communityId,string"`
	ParentID      uint64     `gorm:"column:parent_id" json:"parentId,string"`
	Code          string     `gorm:"column:code" json:"code"`
	Name          string     `gorm:"column:name" json:"name"`
	NodeType      int        `gorm:"column:node_type" json:"nodeType"`
	Category      int        `gorm:"column:category" json:"category"`
	LevelNo       int        `gorm:"column:level_no" json:"levelNo"`
	Path          string     `gorm:"column:path" json:"path"`
	BuildingType  int        `gorm:"column:building_type" json:"buildingType"`
	TotalFloors   int        `gorm:"column:total_floors" json:"totalFloors"`
	FloorsAbove   int        `gorm:"column:floors_above" json:"floorsAbove"`
	FloorsUnder   int        `gorm:"column:floors_under" json:"floorsUnder"`
	UnitCount     int        `gorm:"column:unit_count" json:"unitCount"`
	ElevatorCount int        `gorm:"column:elevator_count" json:"elevatorCount"`
	BuildArea     float64    `gorm:"column:build_area" json:"buildArea"`
	BuildYear     int        `gorm:"column:build_year" json:"buildYear"`
	StructureType string     `gorm:"column:structure_type" json:"structureType"`
	DeliveryDate  *time.Time `gorm:"column:delivery_date" json:"deliveryDate"`
	Address       string     `gorm:"column:address" json:"address"`
	FullAddress   string     `gorm:"column:full_address" json:"fullAddress"`
	FloorNo       int        `gorm:"column:floor_no" json:"floorNo"`
	HouseCount    int        `gorm:"column:house_count" json:"houseCount"`
	Sort          int        `gorm:"column:sort" json:"sort"`
	Status        int        `gorm:"column:status" json:"status"`
}

func (StructureNode) TableName() string { return "pm_structure_node" }
