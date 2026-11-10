/**
 * 物业管理系统后端领域模型类型声明
 *
 * 对应 Go 后端 /api/v1/property/**，与 `WebBasic.*`（脚手架通用模块）并列。
 * 所有雪花 ID 一律为 string（后端 `,string` 序列化），金额为 number（两位小数）。
 *
 * @module types/api/property
 */

declare namespace Property {
  /** 通用分页 */
  interface Page<T> {
    list: T[]
    total: number
    page: number
    pageSize: number
  }

  /** 通用下拉项 */
  interface Option {
    id: string
    name: string
    code?: string
    parentId?: string
    extra?: string
  }

  /** 行政区划节点（省/市/区县/街道/社区） */
  interface DistrictOption {
    id: string
    code: string
    name: string
    level: number
    hasChildren: boolean
  }

  /** 小区 */
  interface Community {
    id: string
    tenantId: string
    tenantName?: string
    code: string
    name: string
    shortName?: string
    communityType: number
    province?: string
    city?: string
    district?: string
    town?: string
    village?: string
    districtCode?: string
    streetCode?: string
    villageCode?: string
    address?: string
    longitude?: number
    latitude?: number
    totalArea?: number
    greenArea?: number
    buildingCount?: number
    houseCount?: number
    residentCount?: number
    developer?: string
    propertyCompany?: string
    managerName?: string
    managerPhone?: string
    servicePhone?: string
    handoverDate?: string
    status: number
    sort?: number
    remark?: string
    createTime?: string
  }

  /** 物业公司 */
  interface Company {
    id: string
    tenantId: string
    tenantCode?: string
    name: string
    shortName?: string
    creditCode?: string
    licenseNo?: string
    level?: string
    legalPerson?: string
    contactName?: string
    contactPhone?: string
    email?: string
    province?: string
    city?: string
    district?: string
    address?: string
    serviceScope?: string
    contractNo?: string
    contractStart?: string
    contractEnd?: string
    expireAt?: string
    communityCount?: number
    userCount?: number
    status: number
    remark?: string
  }

  /** 员工 */
  interface Employee {
    id: string
    userId: string
    employeeNo: string
    name: string
    deptId: string
    deptName?: string
    communityId: string
    communityName?: string
    position?: string
    phone?: string
    empType: number
    entryDate?: string
    status: number
    username?: string
    communityIds?: string[]
    communityNames?: string[]
    faceUrl?: string
    remark?: string
  }

  /** 组织架构节点 */
  interface DeptNode {
    id: string
    parentId: string
    name: string
    leader?: string
    phone?: string
    staffing: number
    children?: DeptNode[]
  }

  /** 结构树节点 */
  interface StructureNode {
    id: string
    parentId: string
    level: 'region' | 'building' | 'unit' | 'floor'
    name: string
    code: string
    extra?: Record<string, any>
    children?: StructureNode[]
  }

  /** 层级节点（自定义层级：小区为根） */
  interface StructureNodeItem {
    id: string
    communityId: string
    parentId: string
    code: string
    name: string
    nodeType: number
    category: number
    floorsAbove?: number
    floorsUnder?: number
    unitCount?: number
    elevatorCount?: number
    buildArea?: number
    buildYear?: number
    structureType?: string
    deliveryDate?: string
    address?: string
    totalFloors?: number
    levelNo: number
    path: string
    floorNo: number
    houseCount: number
    childCount: number
    parentName?: string
    levelName?: string
    typeName?: string
    fullName?: string
    sort?: number
    status?: number
  }

  /** 树懒加载节点 */
  interface TreeNodeChild {
    rowType: 'node' | 'house'
    node?: StructureNodeItem
    house?: House
  }

  /** 层级信息卡片 */
  interface NodeCard {
    id: string
    name: string
    type: number
    typeName: string
    parentId: string
    parentName: string
    address: string
    childNames: string[]
    houseTotal: number
    residentCount: number
    devices: string[]
    deviceTotal: number
    pathNames: string[]
  }

  /** 房屋信息卡片 */
  interface HouseCard {
    house: House
    address: string
    residents: Array<{
      residentId: string
      name: string
      phone: string
      role: number
      roleName: string
      relation: string
      isMain: boolean
      startDate: string
      endDate: string
    }>
    vehicles: Array<{
      vehicleId: string
      plateNo: string
      plateColor: number
      vehicleType: number
      vehicleTypeName: string
      parkingSpaceNo: string
      ownerType: number
    }>
  }

  /** 小区信息卡片 */
  interface CommunityCard {
    id: string
    name: string
    code: string
    address: string
    managerName: string
    servicePhone: string
    buildings: number
    houses: number
    residents: number
    devices: number
    buildArea: number
    houseArea: number
  }

  /** 小区基础信息概览 */
  interface SpaceSummary {
    buildings: number
    units: number
    floors: number
    houses: number
    totalBuildArea: number
    totalHouseArea: number
    byUsage: Record<string, number>
  }

  /** 房屋 */
  interface House {
    id: string
    communityId: string
    regionId: string
    buildingId: string
    buildingName?: string
    unitId: string
    unitName?: string
    floorId: string
    nodeId: string
    nodeName?: string
    nodePath?: string
    floorNo: number
    houseTypeId: string
    houseTypeName?: string
    code: string
    houseNo: string
    fullName: string
    usageType: number
    areaBuild: number
    areaUse: number
    areaShare: number
    ceilingHeight?: number
    propertyRight?: number
    isStreet?: boolean
    shopWidth?: number
    isSale?: boolean
    facilities?: string
    handoverStatus?: number
    orientation?: string
    layout?: string
    decoration: number
    houseStatus: number
    ownerCount: number
    residentCount: number
    deliveryDate?: string
    remark?: string
  }

  /** 房屋类型 */
  interface HouseType {
    id: string
    communityId: string
    name: string
    code?: string
    layout?: string
    usageType: number
    sort?: number
    status: number
  }

  /** 住户（房屋关系行） */
  interface ResidentRow {
    relId: string
    residentId: string
    communityId: string
    communityName: string
    houseId: string
    houseName: string
    name: string
    idNo: string
    phone: string
    gender: number
    photoUrl?: string
    relRole: number
    relation?: string
    isMain?: boolean
    startDate?: string
    endDate?: string
    relStatus: number
  }

  /** 住户详情 */
  interface Resident {
    id: string
    name: string
    idType: number
    idNo: string
    gender: number
    birthday?: string
    phone: string
    mobile2?: string
    nation?: string
    nativePlace?: string
    householdAddr?: string
    liveAddr?: string
    photoUrl?: string
    emergencyContact?: string
    emergencyPhone?: string
    status: number
    houses?: ResidentHouse[]
    faces?: ResidentFace[]
    otherCommunityCount?: number
    remark?: string
  }

  /** 住户-房屋关系 */
  interface ResidentHouse {
    id: string
    communityId: string
    communityName?: string
    residentId: string
    houseId: string
    houseName?: string
    relRole: number
    relation?: string
    isMain: boolean
    startDate?: string
    endDate?: string
    status: number
  }

  /** 住户人脸 */
  interface ResidentFace {
    id: string
    residentId: string
    communityId: string
    fileUrl: string
    isPrimary: boolean
    status: number
    issueMsg?: string
  }

  /** 住户改证回显 */
  interface ResidentLookup {
    found: boolean
    residentId: string
    name?: string
    gender?: number
    phone?: string
    photoUrl?: string
    idNo?: string
    birthday?: string
    nation?: string
    registeredCommunityCount?: number
    hasOtherCommunity?: boolean
  }

  /** 住户关系历史 */
  interface ResidentLog {
    id: string
    action: number
    beforeEnd?: string
    afterEnd?: string
    operatorName?: string
    operateTime: string
  }

  /** 访客 */
  interface Visitor {
    id: string
    communityId: string
    visitNo: string
    name: string
    phone: string
    idNo?: string
    company?: string
    visitType: number
    visitReason?: string
    visitHouseId: string
    visitHouseName?: string
    residentName?: string
    vehicleNo?: string
    peopleCount: number
    startTime?: string
    endTime?: string
    enterTime?: string
    leaveTime?: string
    status: number
    operatorName?: string
    faceUrl?: string
    accompanies?: Array<{ name: string; phone?: string; relation?: string }>
  }

  /** 其他人员 */
  interface OtherPerson {
    id: string
    communityId: string
    categoryId: string
    categoryName?: string
    name: string
    phone?: string
    idNo?: string
    companyName?: string
    workContent?: string
    validFrom?: string
    validTo?: string
    faceUrl?: string
    cardNo?: string
    status: number
    deviceIds?: string[]
  }

  /** 人员类别 */
  interface PersonCategory {
    id: string
    communityId: string
    name: string
    needFace: boolean
    needCard: boolean
    needPhone: boolean
    defaultDays: number
    status: number
  }

  /** 单位/商铺 */
  interface Merchant {
    id: string
    communityId: string
    name: string
    merchantType: number
    buildingId: string
    houseId: string
    houseName?: string
    contactName?: string
    contactPhone?: string
    industry?: string
    businessLicense?: string
    area: number
    employeeCount: number
    startDate?: string
    endDate?: string
    status: number
    remark?: string
  }

  /** 车辆 */
  interface Vehicle {
    id: string
    communityId: string
    plateNo: string
    plateColor: number
    vehicleType: number
    ownerType: number
    residentId: string
    houseId: string
    houseName?: string
    parkingSpaceId: string
    parkingSpaceNo?: string
    brand?: string
    model?: string
    color?: string
    energyType?: number
    validFrom?: string
    validTo?: string
    status: number
    remark?: string
  }

  /** 车位 */
  interface ParkingSpace {
    id: string
    communityId: string
    code: string
    spaceNo: string
    spaceType: number
    area?: number
    isSale?: boolean
    feeStandard?: number
    status: number
    ownerName?: string
    remark?: string
  }

  /** 车辆进出记录 */
  interface VehicleLog {
    id: string
    plateNo: string
    vehicleType: number
    direction: number
    passTime: string
    gateName?: string
    amount: number
  }

  /** 设备 */
  interface Device {
    id: string
    communityId: string
    code: string
    name: string
    deviceType: number
    brand?: string
    model?: string
    serialNo?: string
    ip?: string
    mac?: string
    location?: string
    bindLevel: number
    bindId: string
    bindNodeId: string
    bindName?: string
    accessEnabled: boolean
    online: number
    status: number
    installTime?: string
    remark?: string
  }

  /** 设备概览 */
  interface DeviceStats {
    total: number
    access: number
    gate: number
    camera: number
    online: number
    offline: number
    maintain: number
  }

  /** 门禁授权明细 */
  interface AccessGrant {
    id: string
    communityId: string
    batchNo: string
    personType: number
    personId: string
    personName: string
    houseName?: string
    grantType: number
    deviceId: string
    deviceName: string
    cardNo?: string
    phone?: string
    faceUrl?: string
    status: number
    retryCount: number
    expireTime?: string
    lastIssueTime?: string
    lastMsg?: string
    createTime?: string
  }

  /** 授权统计 */
  interface AccessStats {
    total: number
    pending: number
    success: number
    failed: number
    revoked: number
    expired: number
  }

  /** 门禁卡 */
  interface AccessCard {
    id: string
    communityId: string
    cardNo: string
    personType: number
    personId: string
    personName: string
    status: number
    issueTime?: string
  }

  /** 收费项目 */
  interface FeeItem {
    id: string
    communityId: string
    code: string
    name: string
    itemType: number
    calcMode: number
    unitPrice: number
    unit?: string
    billingCycle: number
    generateDay: number
    houseUsage: number
    chargeObject: number
    lateFeeRate?: number
    dueDays: number
    status: number
    remark?: string
  }

  /** 收费标准 */
  interface FeeStandard {
    id: string
    feeItemId: string
    houseUsage: number
    houseTypeId: string
    buildingId: string
    unitPrice: number
    effectiveFrom?: string
    effectiveTo?: string
    status: number
  }

  /** 水电表 */
  interface Meter {
    id: string
    communityId: string
    houseId: string
    houseName?: string
    meterType: number
    meterNo: string
    ratio: number
    initReading: number
    lastReading: number
    lastReadTime?: string
    status: number
  }

  /** 抄表记录 */
  interface MeterReading {
    id: string
    meterId: string
    houseName?: string
    period: string
    prevReading: number
    currReading: number
    usageAmount: number
    unitPrice: number
    amount: number
    readTime?: string
    readerName?: string
    status: number
  }

  /** 账单 */
  interface Bill {
    id: string
    communityId: string
    billNo: string
    feeItemId: string
    feeItemName: string
    itemType: number
    houseId: string
    houseName: string
    merchantName?: string
    residentName?: string
    billMonth: string
    quantity: number
    unitPrice: number
    amount: number
    discountAmount: number
    lateFee: number
    paidAmount: number
    status: number
    dueDate?: string
    payTime?: string
    source: number
    createTime?: string
    payments?: Payment[]
  }

  /** 缴费流水 */
  interface Payment {
    id: string
    communityId: string
    payNo: string
    billNo: string
    houseName?: string
    residentName?: string
    payMethod: number
    amount: number
    status: number
    payTime?: string
    operatorName?: string
    tradeNo?: string
    remark?: string
  }

  /** 账户 */
  interface Account {
    id: string
    communityId: string
    ownerType: number
    residentId: string
    merchantId: string
    ownerName: string
    balance: number
    deposit: number
    status: number
  }

  /** 账户流水 */
  interface AccountLog {
    id: string
    accountId: string
    changeType: number
    amount: number
    balanceAfter: number
    targetType: number
    bizType?: string
    createTime?: string
    remark?: string
  }

  /** 收费汇总 */
  interface BillSummary {
    billMonth: string
    total: number
    unpaid: number
    paid: number
    receivable: number
    received: number
    outstanding: number
  }

  /** 物业概览 */
  interface Dashboard {
    community?: {
      communityId: string
      communityName: string
      houseTotal: number
      houseResident: number
      residentTotal: number
      merchantTotal: number
      vehicleTotal: number
      parkingTotal: number
      deviceTotal: number
      deviceOnline: number
      visitorToday: number
      visitorInside: number
      otherPerson: number
      billMonth: string
      billCount: number
      receivable: number
      received: number
      outstanding: number
      accessSuccess: number
      accessFailed: number
      accessPending: number
    }
    feeTrend: Array<{ billMonth: string; receivable: number; received: number; count: number }>
    communityCount: number
    tenantCount: number
  }

  /** 账单生成结果 */
  interface GenerateResult {
    created: number
    skipped: number
    amount: number
  }

  /** 缴费结果 */
  interface PayResult {
    paymentId: string
    payNo: string
    paid: number
    remaining: number
    status: number
  }
}
