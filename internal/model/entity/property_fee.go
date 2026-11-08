package entity

import "time"

// 收费域实体（对应 migrations/000023_property_fee.up.sql）。

// FeeItem 收费项目 pm_fee_item
type FeeItem struct {
	Base
	TenantID     uint64  `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID  uint64  `gorm:"column:community_id" json:"communityId,string"`
	Code         string  `gorm:"column:code" json:"code"`
	Name         string  `gorm:"column:name" json:"name"`
	ItemType     int     `gorm:"column:item_type" json:"itemType"`
	CalcMode     int     `gorm:"column:calc_mode" json:"calcMode"`
	UnitPrice    float64 `gorm:"column:unit_price" json:"unitPrice"`
	Unit         string  `gorm:"column:unit" json:"unit"`
	BillingCycle int     `gorm:"column:billing_cycle" json:"billingCycle"`
	GenerateDay  int     `gorm:"column:generate_day" json:"generateDay"`
	HouseUsage   int     `gorm:"column:house_usage" json:"houseUsage"`
	ChargeObject int     `gorm:"column:charge_object" json:"chargeObject"`
	LateFeeRate  float64 `gorm:"column:late_fee_rate" json:"lateFeeRate"`
	DueDays      int     `gorm:"column:due_days" json:"dueDays"`
	Status       int     `gorm:"column:status" json:"status"`
	Sort         int     `gorm:"column:sort" json:"sort"`
}

func (FeeItem) TableName() string { return "pm_fee_item" }

// FeeStandard 收费标准 pm_fee_standard
type FeeStandard struct {
	Base
	TenantID      uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID   uint64     `gorm:"column:community_id" json:"communityId,string"`
	FeeItemID     uint64     `gorm:"column:fee_item_id" json:"feeItemId,string"`
	HouseUsage    int        `gorm:"column:house_usage" json:"houseUsage"`
	HouseTypeID   uint64     `gorm:"column:house_type_id" json:"houseTypeId,string"`
	BuildingID    uint64     `gorm:"column:building_id" json:"buildingId,string"`
	UnitPrice     float64    `gorm:"column:unit_price" json:"unitPrice"`
	EffectiveFrom *time.Time `gorm:"column:effective_from" json:"effectiveFrom"`
	EffectiveTo   *time.Time `gorm:"column:effective_to" json:"effectiveTo"`
	Status        int        `gorm:"column:status" json:"status"`
}

func (FeeStandard) TableName() string { return "pm_fee_standard" }

// Meter 水电表 pm_meter
type Meter struct {
	Base
	TenantID     uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID  uint64     `gorm:"column:community_id" json:"communityId,string"`
	HouseID      uint64     `gorm:"column:house_id" json:"houseId,string"`
	HouseName    string     `gorm:"column:house_name" json:"houseName"`
	MerchantID   uint64     `gorm:"column:merchant_id" json:"merchantId,string"`
	MeterType    int        `gorm:"column:meter_type" json:"meterType"`
	MeterNo      string     `gorm:"column:meter_no" json:"meterNo"`
	Ratio        float64    `gorm:"column:ratio" json:"ratio"`
	InitReading  float64    `gorm:"column:init_reading" json:"initReading"`
	LastReading  float64    `gorm:"column:last_reading" json:"lastReading"`
	LastReadTime *time.Time `gorm:"column:last_read_time" json:"lastReadTime"`
	InstallDate  *time.Time `gorm:"column:install_date" json:"installDate"`
	Status       int        `gorm:"column:status" json:"status"`
}

func (Meter) TableName() string { return "pm_meter" }

// MeterReading 抄表记录 pm_meter_reading
type MeterReading struct {
	Base
	TenantID    uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID uint64     `gorm:"column:community_id" json:"communityId,string"`
	MeterID     uint64     `gorm:"column:meter_id" json:"meterId,string"`
	HouseID     uint64     `gorm:"column:house_id" json:"houseId,string"`
	HouseName   string     `gorm:"column:house_name" json:"houseName"`
	Period      string     `gorm:"column:period" json:"period"`
	PrevReading float64    `gorm:"column:prev_reading" json:"prevReading"`
	CurrReading float64    `gorm:"column:curr_reading" json:"currReading"`
	UsageAmount float64    `gorm:"column:usage_amount" json:"usageAmount"`
	UnitPrice   float64    `gorm:"column:unit_price" json:"unitPrice"`
	Amount      float64    `gorm:"column:amount" json:"amount"`
	ReadTime    *time.Time `gorm:"column:read_time" json:"readTime"`
	ReaderID    uint64     `gorm:"column:reader_id" json:"readerId,string"`
	ReaderName  string     `gorm:"column:reader_name" json:"readerName"`
	BillID      uint64     `gorm:"column:bill_id" json:"billId,string"`
	Status      int        `gorm:"column:status" json:"status"`
}

func (MeterReading) TableName() string { return "pm_meter_reading" }

// Bill 账单 pm_bill
type Bill struct {
	Base
	TenantID       uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID    uint64     `gorm:"column:community_id" json:"communityId,string"`
	BillNo         string     `gorm:"column:bill_no" json:"billNo"`
	FeeItemID      uint64     `gorm:"column:fee_item_id" json:"feeItemId,string"`
	FeeItemName    string     `gorm:"column:fee_item_name" json:"feeItemName"`
	ItemType       int        `gorm:"column:item_type" json:"itemType"`
	HouseID        uint64     `gorm:"column:house_id" json:"houseId,string"`
	HouseName      string     `gorm:"column:house_name" json:"houseName"`
	BuildingID     uint64     `gorm:"column:building_id" json:"buildingId,string"`
	MerchantID     uint64     `gorm:"column:merchant_id" json:"merchantId,string"`
	MerchantName   string     `gorm:"column:merchant_name" json:"merchantName"`
	ResidentID     uint64     `gorm:"column:resident_id" json:"residentId,string"`
	ResidentName   string     `gorm:"column:resident_name" json:"residentName"`
	HouseUsage     int        `gorm:"column:house_usage" json:"houseUsage"`
	BillMonth      string     `gorm:"column:bill_month" json:"billMonth"`
	PeriodStart    *time.Time `gorm:"column:period_start" json:"periodStart"`
	PeriodEnd      *time.Time `gorm:"column:period_end" json:"periodEnd"`
	Quantity       float64    `gorm:"column:quantity" json:"quantity"`
	UnitPrice      float64    `gorm:"column:unit_price" json:"unitPrice"`
	Amount         float64    `gorm:"column:amount" json:"amount"`
	DiscountAmount float64    `gorm:"column:discount_amount" json:"discountAmount"`
	LateFee        float64    `gorm:"column:late_fee" json:"lateFee"`
	PaidAmount     float64    `gorm:"column:paid_amount" json:"paidAmount"`
	Status         int        `gorm:"column:status" json:"status"`
	DueDate        *time.Time `gorm:"column:due_date" json:"dueDate"`
	PayTime        *time.Time `gorm:"column:pay_time" json:"payTime"`
	Source         int        `gorm:"column:source" json:"source"`
	MeterReadingID uint64     `gorm:"column:meter_reading_id" json:"meterReadingId,string"`
}

func (Bill) TableName() string { return "pm_bill" }

// Payment 缴费流水 pm_payment
type Payment struct {
	Base
	TenantID     uint64     `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID  uint64     `gorm:"column:community_id" json:"communityId,string"`
	PayNo        string     `gorm:"column:pay_no" json:"payNo"`
	BillID       uint64     `gorm:"column:bill_id" json:"billId,string"`
	BillNo       string     `gorm:"column:bill_no" json:"billNo"`
	HouseID      uint64     `gorm:"column:house_id" json:"houseId,string"`
	HouseName    string     `gorm:"column:house_name" json:"houseName"`
	ResidentID   uint64     `gorm:"column:resident_id" json:"residentId,string"`
	ResidentName string     `gorm:"column:resident_name" json:"residentName"`
	PayMethod    int        `gorm:"column:pay_method" json:"payMethod"`
	Amount       float64    `gorm:"column:amount" json:"amount"`
	Status       int        `gorm:"column:status" json:"status"`
	PayTime      *time.Time `gorm:"column:pay_time" json:"payTime"`
	OperatorID   uint64     `gorm:"column:operator_id" json:"operatorId,string"`
	OperatorName string     `gorm:"column:operator_name" json:"operatorName"`
	TradeNo      string     `gorm:"column:trade_no" json:"tradeNo"`
	Channel      string     `gorm:"column:channel" json:"channel"`
	ChannelResp  string     `gorm:"column:channel_resp" json:"channelResp"`
}

func (Payment) TableName() string { return "pm_payment" }

// Account 账户 pm_account
type Account struct {
	Base
	TenantID    uint64  `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID uint64  `gorm:"column:community_id" json:"communityId,string"`
	OwnerType   int     `gorm:"column:owner_type" json:"ownerType"`
	ResidentID  uint64  `gorm:"column:resident_id" json:"residentId,string"`
	MerchantID  uint64  `gorm:"column:merchant_id" json:"merchantId,string"`
	OwnerName   string  `gorm:"column:owner_name" json:"ownerName"`
	Balance     float64 `gorm:"column:balance" json:"balance"`
	Deposit     float64 `gorm:"column:deposit" json:"deposit"`
	Status      int     `gorm:"column:status" json:"status"`
}

func (Account) TableName() string { return "pm_account" }

// AccountLog 账户流水 pm_account_log
type AccountLog struct {
	Base
	TenantID     uint64  `gorm:"column:tenant_id" json:"tenantId,string"`
	CommunityID  uint64  `gorm:"column:community_id" json:"communityId,string"`
	AccountID    uint64  `gorm:"column:account_id" json:"accountId,string"`
	ChangeType   int     `gorm:"column:change_type" json:"changeType"`
	Amount       float64 `gorm:"column:amount" json:"amount"`
	BalanceAfter float64 `gorm:"column:balance_after" json:"balanceAfter"`
	TargetType   int     `gorm:"column:target_type" json:"targetType"`
	BizType      string  `gorm:"column:biz_type" json:"bizType"`
	BizID        uint64  `gorm:"column:biz_id" json:"bizId,string"`
}

func (AccountLog) TableName() string { return "pm_account_log" }
