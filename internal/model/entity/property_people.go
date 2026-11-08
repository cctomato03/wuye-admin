package entity

import "time"

// 人员域实体（对应 migrations/000021_property_people.up.sql）。

// Resident 住户档案 pm_resident（租户级：一个证件一条）
type Resident struct {
	Base
	TenantID         uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	Name             string     `gorm:"column:name" json:"name"`
	IDType           int        `gorm:"column:id_type" json:"idType"`
	IDNo             string     `gorm:"column:id_no" json:"idNo"`
	Gender           int        `gorm:"column:gender" json:"gender"`
	Birthday         *time.Time `gorm:"column:birthday" json:"birthday"`
	Phone            string     `gorm:"column:phone" json:"phone"`
	Mobile2          string     `gorm:"column:mobile2" json:"mobile2"`
	Nation           string     `gorm:"column:nation" json:"nation"`
	NativePlace      string     `gorm:"column:native_place" json:"nativePlace"`
	HouseholdAddr    string     `gorm:"column:household_addr" json:"householdAddr"`
	LiveAddr         string     `gorm:"column:live_addr" json:"liveAddr"`
	PhotoURL         string     `gorm:"column:photo_url" json:"photoUrl"`
	EmergencyContact string     `gorm:"column:emergency_contact" json:"emergencyContact"`
	EmergencyPhone   string     `gorm:"column:emergency_phone" json:"emergencyPhone"`
	FirstCommunityID uint64     `gorm:"column:first_community_id" json:"firstCommunityId,string"`
	Status           int        `gorm:"column:status" json:"status"`
}

func (Resident) TableName() string { return "pm_resident" }

// ResidentHouse 住户-房屋关系 pm_resident_house
type ResidentHouse struct {
	Base
	TenantID    uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID uint64     `gorm:"column:community_id" json:"communityId,string"`
	ResidentID  uint64     `gorm:"column:resident_id" json:"residentId,string"`
	HouseID     uint64     `gorm:"column:house_id" json:"houseId,string"`
	RelRole     int        `gorm:"column:rel_role" json:"relRole"`
	Relation    string     `gorm:"column:relation" json:"relation"`
	IsMain      bool       `gorm:"column:is_main" json:"isMain"`
	StartDate   *time.Time `gorm:"column:start_date" json:"startDate"`
	EndDate     *time.Time `gorm:"column:end_date" json:"endDate"`
	Status      int        `gorm:"column:status" json:"status"`
	BindTime    *time.Time `gorm:"column:bind_time" json:"bindTime"`
	UnbindTime  *time.Time `gorm:"column:unbind_time" json:"unbindTime"`
}

func (ResidentHouse) TableName() string { return "pm_resident_house" }

// ResidentHouseLog 住户房屋关系变更历史 pm_resident_house_log
type ResidentHouseLog struct {
	Base
	TenantID     uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID  uint64     `gorm:"column:community_id" json:"communityId,string"`
	ResidentID   uint64     `gorm:"column:resident_id" json:"residentId,string"`
	HouseID      uint64     `gorm:"column:house_id" json:"houseId,string"`
	RelID        uint64     `gorm:"column:rel_id" json:"relId,string"`
	Action       int        `gorm:"column:action" json:"action"`
	BeforeEnd    *time.Time `gorm:"column:before_end" json:"beforeEnd"`
	AfterEnd     *time.Time `gorm:"column:after_end" json:"afterEnd"`
	BeforeRole   int        `gorm:"column:before_role" json:"beforeRole"`
	AfterRole    int        `gorm:"column:after_role" json:"afterRole"`
	OperatorID   uint64     `gorm:"column:operator_id" json:"operatorId,string"`
	OperatorName string     `gorm:"column:operator_name" json:"operatorName"`
	OperateTime  time.Time  `gorm:"column:operate_time" json:"operateTime"`
}

func (ResidentHouseLog) TableName() string { return "pm_resident_house_log" }

// ResidentFace 住户人脸 pm_resident_face
type ResidentFace struct {
	Base
	TenantID    uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID uint64 `gorm:"column:community_id" json:"communityId,string"`
	ResidentID  uint64 `gorm:"column:resident_id" json:"residentId,string"`
	FileID      uint64 `gorm:"column:file_id" json:"fileId,string"`
	FileURL     string `gorm:"column:file_url" json:"fileUrl"`
	FaceQuality int    `gorm:"column:face_quality" json:"faceQuality"`
	Status      int    `gorm:"column:status" json:"status"`
	IsPrimary   bool   `gorm:"column:is_primary" json:"isPrimary"`
	IssueMsg    string `gorm:"column:issue_msg" json:"issueMsg"`
}

func (ResidentFace) TableName() string { return "pm_resident_face" }

// Visitor 访客 pm_visitor
type Visitor struct {
	Base
	TenantID       uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID    uint64     `gorm:"column:community_id" json:"communityId,string"`
	VisitNo        string     `gorm:"column:visit_no" json:"visitNo"`
	Name           string     `gorm:"column:name" json:"name"`
	Phone          string     `gorm:"column:phone" json:"phone"`
	IDNo           string     `gorm:"column:id_no" json:"idNo"`
	Company        string     `gorm:"column:company" json:"company"`
	VisitType      int        `gorm:"column:visit_type" json:"visitType"`
	VisitReason    string     `gorm:"column:visit_reason" json:"visitReason"`
	VisitHouseID   uint64     `gorm:"column:visit_house_id" json:"visitHouseId,string"`
	VisitHouseName string     `gorm:"column:visit_house_name" json:"visitHouseName"`
	ResidentID     uint64     `gorm:"column:resident_id" json:"residentId,string"`
	ResidentName   string     `gorm:"column:resident_name" json:"residentName"`
	VehicleNo      string     `gorm:"column:vehicle_no" json:"vehicleNo"`
	PeopleCount    int        `gorm:"column:people_count" json:"peopleCount"`
	StartTime      *time.Time `gorm:"column:start_time" json:"startTime"`
	EndTime        *time.Time `gorm:"column:end_time" json:"endTime"`
	EnterTime      *time.Time `gorm:"column:enter_time" json:"enterTime"`
	LeaveTime      *time.Time `gorm:"column:leave_time" json:"leaveTime"`
	Status         int        `gorm:"column:status" json:"status"`
	OperatorID     uint64     `gorm:"column:operator_id" json:"operatorId,string"`
	OperatorName   string     `gorm:"column:operator_name" json:"operatorName"`
	FaceURL        string     `gorm:"column:face_url" json:"faceUrl"`
}

func (Visitor) TableName() string { return "pm_visitor" }

// VisitorAccompany 访客随行人员 pm_visitor_accompany
type VisitorAccompany struct {
	Base
	TenantID    uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID uint64 `gorm:"column:community_id" json:"communityId,string"`
	VisitorID   uint64 `gorm:"column:visitor_id" json:"visitorId,string"`
	Name        string `gorm:"column:name" json:"name"`
	IDNo        string `gorm:"column:id_no" json:"idNo"`
	Phone       string `gorm:"column:phone" json:"phone"`
	Relation    string `gorm:"column:relation" json:"relation"`
}

func (VisitorAccompany) TableName() string { return "pm_visitor_accompany" }

// PersonCategory 其他人员类别 pm_person_category
type PersonCategory struct {
	Base
	TenantID    uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID uint64 `gorm:"column:community_id" json:"communityId,string"`
	Name        string `gorm:"column:name" json:"name"`
	Code        string `gorm:"column:code" json:"code"`
	NeedFace    bool   `gorm:"column:need_face" json:"needFace"`
	NeedCard    bool   `gorm:"column:need_card" json:"needCard"`
	NeedPhone   bool   `gorm:"column:need_phone" json:"needPhone"`
	DefaultDays int    `gorm:"column:default_days" json:"defaultDays"`
	Sort        int    `gorm:"column:sort" json:"sort"`
	Status      int    `gorm:"column:status" json:"status"`
}

func (PersonCategory) TableName() string { return "pm_person_category" }

// OtherPerson 其他人员 pm_other_person
type OtherPerson struct {
	Base
	TenantID    uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID uint64     `gorm:"column:community_id" json:"communityId,string"`
	CategoryID  uint64     `gorm:"column:category_id" json:"categoryId,string"`
	Name        string     `gorm:"column:name" json:"name"`
	Phone       string     `gorm:"column:phone" json:"phone"`
	IDNo        string     `gorm:"column:id_no" json:"idNo"`
	CompanyName string     `gorm:"column:company_name" json:"companyName"`
	WorkContent string     `gorm:"column:work_content" json:"workContent"`
	ValidFrom   *time.Time `gorm:"column:valid_from" json:"validFrom"`
	ValidTo     *time.Time `gorm:"column:valid_to" json:"validTo"`
	FaceURL     string     `gorm:"column:face_url" json:"faceUrl"`
	CardNo      string     `gorm:"column:card_no" json:"cardNo"`
	Status      int        `gorm:"column:status" json:"status"`
}

func (OtherPerson) TableName() string { return "pm_other_person" }

// OtherPersonDevice 其他人员-门禁设备 pm_other_person_device
type OtherPersonDevice struct {
	Base
	TenantID    uint64 `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID uint64 `gorm:"column:community_id" json:"communityId,string"`
	PersonID    uint64 `gorm:"column:person_id" json:"personId,string"`
	DeviceID    uint64 `gorm:"column:device_id" json:"deviceId,string"`
}

func (OtherPersonDevice) TableName() string { return "pm_other_person_device" }

// Merchant 单位/商铺 pm_merchant
type Merchant struct {
	Base
	TenantID        uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID     uint64     `gorm:"column:community_id" json:"communityId,string"`
	Name            string     `gorm:"column:name" json:"name"`
	MerchantType    int        `gorm:"column:merchant_type" json:"merchantType"`
	BuildingID      uint64     `gorm:"column:building_id" json:"buildingId,string"`
	HouseID         uint64     `gorm:"column:house_id" json:"houseId,string"`
	HouseName       string     `gorm:"column:house_name" json:"houseName"`
	ContactName     string     `gorm:"column:contact_name" json:"contactName"`
	ContactPhone    string     `gorm:"column:contact_phone" json:"contactPhone"`
	IDNo            string     `gorm:"column:id_no" json:"idNo"`
	Industry        string     `gorm:"column:industry" json:"industry"`
	BusinessLicense string     `gorm:"column:business_license" json:"businessLicense"`
	Area            float64    `gorm:"column:area" json:"area"`
	EmployeeCount   int        `gorm:"column:employee_count" json:"employeeCount"`
	StartDate       *time.Time `gorm:"column:start_date" json:"startDate"`
	EndDate         *time.Time `gorm:"column:end_date" json:"endDate"`
	Status          int        `gorm:"column:status" json:"status"`
}

func (Merchant) TableName() string { return "pm_merchant" }
