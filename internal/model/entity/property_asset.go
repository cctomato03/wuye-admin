package entity

import "time"

// 资产/设备/门禁域实体（对应 migrations/000022_property_vehicle_device.up.sql）。

// Company 物业公司档案 pm_company（1:1 关联 sys_tenant）
type Company struct {
	Base
	TenantID      uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	Name          string     `gorm:"column:name" json:"name"`
	ShortName     string     `gorm:"column:short_name" json:"shortName"`
	CreditCode    string     `gorm:"column:credit_code" json:"creditCode"`
	LicenseNo     string     `gorm:"column:license_no" json:"licenseNo"`
	Level         string     `gorm:"column:level" json:"level"`
	LegalPerson   string     `gorm:"column:legal_person" json:"legalPerson"`
	ContactName   string     `gorm:"column:contact_name" json:"contactName"`
	ContactPhone  string     `gorm:"column:contact_phone" json:"contactPhone"`
	Email         string     `gorm:"column:email" json:"email"`
	Province      string     `gorm:"column:province" json:"province"`
	City          string     `gorm:"column:city" json:"city"`
	District      string     `gorm:"column:district" json:"district"`
	Address       string     `gorm:"column:address" json:"address"`
	ServiceScope  string     `gorm:"column:service_scope" json:"serviceScope"`
	ContractNo    string     `gorm:"column:contract_no" json:"contractNo"`
	ContractStart *time.Time `gorm:"column:contract_start" json:"contractStart"`
	ContractEnd   *time.Time `gorm:"column:contract_end" json:"contractEnd"`
	Status        int        `gorm:"column:status" json:"status"`
}

func (Company) TableName() string { return "pm_company" }

// Employee 物业员工档案 pm_employee
type Employee struct {
	Base
	TenantID    uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	UserID      uint64     `gorm:"column:user_id" json:"userId,string"`
	EmployeeNo  string     `gorm:"column:employee_no" json:"employeeNo"`
	Name        string     `gorm:"column:name" json:"name"`
	DeptID      uint64     `gorm:"column:dept_id" json:"deptId,string"`
	CommunityID uint64     `gorm:"column:community_id" json:"communityId,string"`
	Position    string     `gorm:"column:position" json:"position"`
	Phone       string     `gorm:"column:phone" json:"phone"`
	IDNo        string     `gorm:"column:id_no" json:"idNo"`
	Gender      int        `gorm:"column:gender" json:"gender"`
	EmpType     int        `gorm:"column:emp_type" json:"empType"`
	EntryDate   *time.Time `gorm:"column:entry_date" json:"entryDate"`
	LeaveDate   *time.Time `gorm:"column:leave_date" json:"leaveDate"`
	FaceURL     string     `gorm:"column:face_url" json:"faceUrl"`
	Status      int        `gorm:"column:status" json:"status"`
}

func (Employee) TableName() string { return "pm_employee" }

// ParkingSpace 车位 pm_parking_space
type ParkingSpace struct {
	Base
	TenantID        uint64  `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID     uint64  `gorm:"column:community_id" json:"communityId,string"`
	RegionID        uint64  `gorm:"column:region_id" json:"regionId,string"`
	Code            string  `gorm:"column:code" json:"code"`
	SpaceNo         string  `gorm:"column:space_no" json:"spaceNo"`
	SpaceType       int     `gorm:"column:space_type" json:"spaceType"`
	Area            float64 `gorm:"column:area" json:"area"`
	IsSale          bool    `gorm:"column:is_sale" json:"isSale"`
	FeeStandard     float64 `gorm:"column:fee_standard" json:"feeStandard"`
	Status          int     `gorm:"column:status" json:"status"`
	OwnerResidentID uint64  `gorm:"column:owner_resident_id" json:"ownerResidentId,string"`
	OwnerName       string  `gorm:"column:owner_name" json:"ownerName"`
}

func (ParkingSpace) TableName() string { return "pm_parking_space" }

// Vehicle 车辆 pm_vehicle
type Vehicle struct {
	Base
	TenantID       uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID    uint64     `gorm:"column:community_id" json:"communityId,string"`
	PlateNo        string     `gorm:"column:plate_no" json:"plateNo"`
	PlateColor     int        `gorm:"column:plate_color" json:"plateColor"`
	VehicleType    int        `gorm:"column:vehicle_type" json:"vehicleType"`
	OwnerType      int        `gorm:"column:owner_type" json:"ownerType"`
	ResidentID     uint64     `gorm:"column:resident_id" json:"residentId,string"`
	HouseID        uint64     `gorm:"column:house_id" json:"houseId,string"`
	HouseName      string     `gorm:"column:house_name" json:"houseName"`
	EmployeeID     uint64     `gorm:"column:employee_id" json:"employeeId,string"`
	MerchantID     uint64     `gorm:"column:merchant_id" json:"merchantId,string"`
	ParkingSpaceID uint64     `gorm:"column:parking_space_id" json:"parkingSpaceId,string"`
	ParkingSpaceNo string     `gorm:"column:parking_space_no" json:"parkingSpaceNo"`
	Brand          string     `gorm:"column:brand" json:"brand"`
	Model          string     `gorm:"column:model" json:"model"`
	Color          string     `gorm:"column:color" json:"color"`
	EnergyType     int        `gorm:"column:energy_type" json:"energyType"`
	ValidFrom      *time.Time `gorm:"column:valid_from" json:"validFrom"`
	ValidTo        *time.Time `gorm:"column:valid_to" json:"validTo"`
	Status         int        `gorm:"column:status" json:"status"`
}

func (Vehicle) TableName() string { return "pm_vehicle" }

// VehicleAccessLog 车辆进出记录 pm_vehicle_access_log
type VehicleAccessLog struct {
	Base
	TenantID     uint64    `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID  uint64    `gorm:"column:community_id" json:"communityId,string"`
	VehicleID    uint64    `gorm:"column:vehicle_id" json:"vehicleId,string"`
	PlateNo      string    `gorm:"column:plate_no" json:"plateNo"`
	VehicleType  int       `gorm:"column:vehicle_type" json:"vehicleType"`
	Direction    int       `gorm:"column:direction" json:"direction"`
	PassTime     time.Time `gorm:"column:pass_time" json:"passTime"`
	GateDeviceID uint64    `gorm:"column:gate_device_id" json:"gateDeviceId,string"`
	GateName     string    `gorm:"column:gate_name" json:"gateName"`
	ImageURL     string    `gorm:"column:image_url" json:"imageUrl"`
	Amount       float64   `gorm:"column:amount" json:"amount"`
}

func (VehicleAccessLog) TableName() string { return "pm_vehicle_access_log" }

// Device 设备 pm_device
type Device struct {
	Base
	TenantID      uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID   uint64     `gorm:"column:community_id" json:"communityId,string"`
	Code          string     `gorm:"column:code" json:"code"`
	Name          string     `gorm:"column:name" json:"name"`
	DeviceType    int        `gorm:"column:device_type" json:"deviceType"`
	Brand         string     `gorm:"column:brand" json:"brand"`
	Model         string     `gorm:"column:model" json:"model"`
	SerialNo      string     `gorm:"column:serial_no" json:"serialNo"`
	IP            string     `gorm:"column:ip" json:"ip"`
	MAC           string     `gorm:"column:mac" json:"mac"`
	Location      string     `gorm:"column:location" json:"location"`
	BindLevel     int        `gorm:"column:bind_level" json:"bindLevel"`
	BindID        uint64     `gorm:"column:bind_id" json:"bindId,string"`
	BindName      string     `gorm:"column:bind_name" json:"bindName"`
	BindNodeID    uint64     `gorm:"column:bind_node_id" json:"bindNodeId,string"`
	AccessEnabled bool       `gorm:"column:access_enabled" json:"accessEnabled"`
	InstallTime   *time.Time `gorm:"column:install_time" json:"installTime"`
	Online        int        `gorm:"column:online" json:"online"`
	LastHeartbeat *time.Time `gorm:"column:last_heartbeat" json:"lastHeartbeat"`
	Status        int        `gorm:"column:status" json:"status"`
}

func (Device) TableName() string { return "pm_device" }

// AccessCard 门禁卡 pm_access_card
type AccessCard struct {
	Base
	TenantID    uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID uint64     `gorm:"column:community_id" json:"communityId,string"`
	CardNo      string     `gorm:"column:card_no" json:"cardNo"`
	PersonType  int        `gorm:"column:person_type" json:"personType"`
	PersonID    uint64     `gorm:"column:person_id" json:"personId,string"`
	PersonName  string     `gorm:"column:person_name" json:"personName"`
	Status      int        `gorm:"column:status" json:"status"`
	IssueTime   *time.Time `gorm:"column:issue_time" json:"issueTime"`
}

func (AccessCard) TableName() string { return "pm_access_card" }

// AccessGrant 门禁授权明细 pm_access_grant
type AccessGrant struct {
	Base
	TenantID      uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID   uint64     `gorm:"column:community_id" json:"communityId,string"`
	BatchNo       string     `gorm:"column:batch_no" json:"batchNo"`
	PersonType    int        `gorm:"column:person_type" json:"personType"`
	PersonID      uint64     `gorm:"column:person_id" json:"personId,string"`
	PersonName    string     `gorm:"column:person_name" json:"personName"`
	HouseID       uint64     `gorm:"column:house_id" json:"houseId,string"`
	HouseName     string     `gorm:"column:house_name" json:"houseName"`
	GrantType     int        `gorm:"column:grant_type" json:"grantType"`
	DeviceID      uint64     `gorm:"column:device_id" json:"deviceId,string"`
	DeviceName    string     `gorm:"column:device_name" json:"deviceName"`
	CardNo        string     `gorm:"column:card_no" json:"cardNo"`
	Phone         string     `gorm:"column:phone" json:"phone"`
	FaceURL       string     `gorm:"column:face_url" json:"faceUrl"`
	Status        int        `gorm:"column:status" json:"status"`
	RetryCount    int        `gorm:"column:retry_count" json:"retryCount"`
	ExpireTime    *time.Time `gorm:"column:expire_time" json:"expireTime"`
	LastIssueTime *time.Time `gorm:"column:last_issue_time" json:"lastIssueTime"`
	LastMsg       string     `gorm:"column:last_msg" json:"lastMsg"`
	RevokedTime   *time.Time `gorm:"column:revoked_time" json:"revokedTime"`
}

func (AccessGrant) TableName() string { return "pm_access_grant" }

// AccessTask 门禁下发批次 pm_access_task
type AccessTask struct {
	Base
	TenantID    uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID uint64     `gorm:"column:community_id" json:"communityId,string"`
	BatchNo     string     `gorm:"column:batch_no" json:"batchNo"`
	PersonType  int        `gorm:"column:person_type" json:"personType"`
	PersonID    uint64     `gorm:"column:person_id" json:"personId,string"`
	PersonName  string     `gorm:"column:person_name" json:"personName"`
	Action      int        `gorm:"column:action" json:"action"`
	Total       int        `gorm:"column:total" json:"total"`
	Success     int        `gorm:"column:success" json:"success"`
	Failed      int        `gorm:"column:failed" json:"failed"`
	Status      int        `gorm:"column:status" json:"status"`
	Message     string     `gorm:"column:message" json:"message"`
	StartTime   *time.Time `gorm:"column:start_time" json:"startTime"`
	EndTime     *time.Time `gorm:"column:end_time" json:"endTime"`
}

func (AccessTask) TableName() string { return "pm_access_task" }
