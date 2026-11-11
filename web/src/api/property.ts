/**
 * 物业管理模块接口（/api/v1/property/**）
 *
 * 约定与 web-basic 一致：
 *  - 成功提示写在接口层（successMessage），页面不再重复弹窗；
 *  - 分页统一 page/pageSize ↔ useTable 的 current/size 转换；
 *  - 所有出参直接是后端领域结构（Property.*）。
 *
 * @module api/property
 */
import request from '@/utils/http'
import { toPageQuery, toTablePage } from './business'
import { currentOrgTenantId } from '@/utils/org-tenant'

/** 当前登录主体信息（含 isSuper / 授权小区），用于页面级权限判断 */
export function fetchGetUserInfo() {
  return request.get<{
    user?: { isSuper: boolean; tenantId: string; communityIds?: string[] }
    isSuper?: boolean
    tenantId?: string
    communityIds?: string[]
  }>({
    url: '/api/v1/auth/userinfo'
  })
}

/* ------------------------------ 小区 ------------------------------ */

export function fetchGetCommunityList(params: Record<string, any> = {}) {
  return request.get<Property.Page<Property.Community>>({
    url: '/api/v1/property/communities',
    params: toPageQuery(params)
  })
}

/** 小区分页（useTable 结构） */

export async function fetchCommunityPage(params: Record<string, any>) {
  const res = await fetchGetCommunityList(params)
  return toTablePage(res)
}

export function fetchGetCommunityOptions() {
  return request.get<Property.Option[]>({ url: '/api/v1/property/communities/options' })
}

export function fetchGetCommunity(id: string) {
  return request.get<Property.Community>({ url: `/api/v1/property/communities/${id}` })
}

export function fetchCreateCommunity(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/communities',
    data,
    successMessage: '小区创建成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateCommunity(id: string, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/property/communities/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteCommunity(id: string) {
  return request.del<null>({
    url: `/api/v1/property/communities/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export function fetchChangeCommunityStatus(id: string, status: number) {
  return request.put<null>({
    url: `/api/v1/property/communities/${id}/status`,
    data: { status },
    successMessage: '状态已更新',
    showSuccessMessage: true
  })
}

export function fetchGetDistrictTree() {
  return request.get<Array<{ id: string; name: string; children?: any[] }>>({
    url: '/api/v1/property/districts/tree'
  })
}

/** 行政区划逐级查询（级联懒加载）：parentId 为空时取省级 */

export function fetchDistrictChildren(params: { parentId?: string; level?: number; keyword?: string } = {}) {
  return request.get<Property.DistrictOption[]>({
    url: '/api/v1/property/districts/children',
    params
  })
}

/** 按编码还原行政区划路径（省/市/区县/街道/社区） */

export function fetchDistrictPath(code: string) {
  return request.get<Array<{ id: string; code: string; name: string; level: number }>>({
    url: '/api/v1/property/districts/path',
    params: { code }
  })
}

export function fetchGetTenantOptions() {
  return request.get<Property.Option[]>({ url: '/api/v1/property/tenants/options' })
}

/* ------------------------------ 物业公司 ------------------------------ */

export async function fetchCompanyPage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.Company>>({
    url: '/api/v1/property/companies',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchCreateCompany(data: Record<string, any>) {
  return request.post<{
    tenantId: string
    companyId: string
    adminUsername: string
    adminPassword: string
  }>({
    url: '/api/v1/property/companies',
    data,
    successMessage: '物业公司创建成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateCompany(id: string, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/property/companies/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteCompany(id: string) {
  return request.del<null>({
    url: `/api/v1/property/companies/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

/* ------------------------------ 概览 ------------------------------ */

export function fetchGetPropertyDashboard(communityId?: string) {
  return request.get<Property.Dashboard>({
    url: '/api/v1/property/dashboard',
    params: communityId ? { communityId } : {}
  })
}

export async function fetchCommunityStatsPage(params: Record<string, any> = {}) {
  const res = await request.get<Property.Page<any>>({
    url: '/api/v1/property/dashboard/communities',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

/* ------------------------------ 员工 ------------------------------ */

export async function fetchEmployeePage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.Employee>>({
    url: '/api/v1/property/employees',
    params: { ...toPageQuery(params), tenantId: currentOrgTenantId() || undefined }
  })
  return toTablePage(res)
}

export function fetchCreateEmployee(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/employees',
    data,
    successMessage: '员工创建成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateEmployee(id: string, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/property/employees/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteEmployee(id: string) {
  return request.del<null>({
    url: `/api/v1/property/employees/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export function fetchGrantEmployeeCommunities(id: string, data: { communityIds: string[]; defaultId?: string }) {
  return request.put<null>({
    url: `/api/v1/property/employees/${id}/communities`,
    data,
    successMessage: '小区授权已更新',
    showSuccessMessage: true
  })
}

export function fetchGetEmployeeDeptTree() {
  return request.get<Property.DeptNode[]>({
    url: '/api/v1/property/employees/dept-tree',
    params: { tenantId: currentOrgTenantId() || undefined }
  })
}

export function fetchGetEmployeeRoles() {
  return request.get<Property.Option[]>({
    url: '/api/v1/property/employees/roles',
    params: { tenantId: currentOrgTenantId() || undefined }
  })
}

/* ------------------------------ 楼宇结构 ------------------------------ */

/** 自定义层级树（层级管理/房屋管理页） */

export function fetchNodeTree(communityId: string) {
  return request.get<Property.StructureNodeItem[]>({
    url: '/api/v1/property/nodes/tree',
    params: { communityId }
  })
}

/** 层级下拉（房屋/设备选择） */

export function fetchNodeOptions(communityId: string, leafOnly = false) {
  return request.get<Property.Option[]>({
    url: '/api/v1/property/nodes/options',
    params: { communityId, leafOnly }
  })
}

export function fetchCreateNode(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/nodes',
    data,
    successMessage: '层级已新增',
    showSuccessMessage: true
  })
}

export function fetchUpdateNode(id: string, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/property/nodes/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteNode(id: string) {
  return request.del<null>({
    url: `/api/v1/property/nodes/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export async function fetchHousePage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.House>>({
    url: '/api/v1/property/structures/houses',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchCreateHouse(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/structures/houses',
    data,
    successMessage: '房屋创建成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateHouse(id: string, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/property/structures/houses/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteHouse(id: string) {
  return request.del<null>({
    url: `/api/v1/property/structures/houses/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export function fetchBatchCreateHouses(data: Record<string, any>) {
  return request.post<{ created: number }>({
    url: '/api/v1/property/structures/houses/batch',
    data,
    successMessage: '批量生成完成',
    showSuccessMessage: true
  })
}

export function fetchGetHouseTypes(communityId?: string) {
  return request.get<Property.HouseType[]>({
    url: '/api/v1/property/structures/house-types',
    params: communityId ? { communityId } : {}
  })
}

export function fetchCreateHouseType(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/structures/house-types',
    data,
    successMessage: '房屋类型创建成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateHouseType(id: string, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/property/structures/house-types/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteHouseType(id: string) {
  return request.del<null>({
    url: `/api/v1/property/structures/house-types/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

/* ------------------------------ 住户 ------------------------------ */

export async function fetchResidentPage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.ResidentRow>>({
    url: '/api/v1/property/residents',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchGetResident(id: string) {
  return request.get<Property.Resident>({ url: `/api/v1/property/residents/${id}` })
}

export function fetchLookupResident(params: { idType?: number; idNo: string }) {
  return request.get<Property.ResidentLookup>({
    url: '/api/v1/property/residents/lookup',
    params,
    showErrorMessage: false
  })
}

export function fetchCreateResident(data: Record<string, any>) {
  return request.post<{ residentId: string; relationId: string; reused: boolean }>({
    url: '/api/v1/property/residents',
    data,
    successMessage: '住户登记成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateResident(id: string, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/property/residents/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteResident(id: string) {
  return request.del<null>({
    url: `/api/v1/property/residents/${id}`,
    successMessage: '已删除住户并解除房屋关系',
    showSuccessMessage: true
  })
}

export function fetchBindResidentHouse(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/residents/bind',
    data,
    successMessage: '房屋绑定成功',
    showSuccessMessage: true
  })
}

export function fetchUnbindResidentHouse(relId: string) {
  return request.put<null>({
    url: `/api/v1/property/residents/relations/${relId}/unbind`,
    successMessage: '已解除关系',
    showSuccessMessage: true
  })
}

export function fetchRenewResidentHouse(relId: string, data: { endDate: string; relRole?: number }) {
  return request.put<null>({
    url: `/api/v1/property/residents/relations/${relId}/renew`,
    data,
    successMessage: '续期成功',
    showSuccessMessage: true
  })
}

export function fetchRestoreResidentHouse(relId: string) {
  return request.put<null>({
    url: `/api/v1/property/residents/relations/${relId}/restore`,
    successMessage: '已还原',
    showSuccessMessage: true
  })
}

export function fetchGetResidentHistory(id: string) {
  return request.get<Property.ResidentLog[]>({ url: `/api/v1/property/residents/${id}/history` })
}

export function fetchGetHouseResidents(params: { houseId: string; status?: number }) {
  return request.get<any[]>({ url: '/api/v1/property/residents/houses', params })
}

export function fetchAddResidentFace(id: string, data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: `/api/v1/property/residents/${id}/faces`,
    data,
    successMessage: '人脸已登记',
    showSuccessMessage: true
  })
}

export function fetchDeleteResidentFace(id: string) {
  return request.del<null>({
    url: `/api/v1/property/residents/faces/${id}`,
    successMessage: '已删除',
    showSuccessMessage: true
  })
}

/* ------------------------------ 访客 ------------------------------ */

export async function fetchVisitorPage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.Visitor>>({
    url: '/api/v1/property/visitors',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchGetVisitor(id: string) {
  return request.get<Property.Visitor>({ url: `/api/v1/property/visitors/${id}` })
}

export function fetchCreateVisitor(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/visitors',
    data,
    successMessage: '访客登记成功',
    showSuccessMessage: true
  })
}

export function fetchChangeVisitorStatus(id: string, action: 'enter' | 'leave' | 'cancel') {
  return request.put<null>({
    url: `/api/v1/property/visitors/${id}/status`,
    data: { action },
    successMessage: '状态已更新',
    showSuccessMessage: true
  })
}

export function fetchDeleteVisitor(id: string) {
  return request.del<null>({
    url: `/api/v1/property/visitors/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

/* ------------------------------ 其他人员 ------------------------------ */

export async function fetchOtherPersonPage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.OtherPerson>>({
    url: '/api/v1/property/other-people',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchCreateOtherPerson(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/other-people',
    data,
    successMessage: '人员登记成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateOtherPerson(id: string, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/property/other-people/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteOtherPerson(id: string) {
  return request.del<null>({
    url: `/api/v1/property/other-people/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export function fetchGetPersonCategories(communityId?: string) {
  return request.get<Property.PersonCategory[]>({
    url: '/api/v1/property/person-categories',
    params: communityId ? { communityId } : {}
  })
}

export function fetchCreatePersonCategory(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/person-categories',
    data,
    successMessage: '类别已新增',
    showSuccessMessage: true
  })
}

export function fetchUpdatePersonCategory(id: string, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/property/person-categories/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeletePersonCategory(id: string) {
  return request.del<null>({
    url: `/api/v1/property/person-categories/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

/* ------------------------------ 单位 ------------------------------ */

export async function fetchMerchantPage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.Merchant>>({
    url: '/api/v1/property/merchants',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchCreateMerchant(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/merchants',
    data,
    successMessage: '单位登记成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateMerchant(id: string, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/property/merchants/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteMerchant(id: string) {
  return request.del<null>({
    url: `/api/v1/property/merchants/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

/* ------------------------------ 车辆 ------------------------------ */

export async function fetchVehiclePage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.Vehicle>>({
    url: '/api/v1/property/vehicles',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchCreateVehicle(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/vehicles',
    data,
    successMessage: '车辆登记成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateVehicle(id: string, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/property/vehicles/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteVehicle(id: string) {
  return request.del<null>({
    url: `/api/v1/property/vehicles/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export async function fetchParkingSpacePage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.ParkingSpace>>({
    url: '/api/v1/property/parking-spaces',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchCreateParkingSpace(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/parking-spaces',
    data,
    successMessage: '车位新增成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateParkingSpace(id: string, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/property/parking-spaces/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteParkingSpace(id: string) {
  return request.del<null>({
    url: `/api/v1/property/parking-spaces/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export async function fetchVehicleLogPage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.VehicleLog>>({
    url: '/api/v1/property/vehicle-logs',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

/* ------------------------------ 设备 ------------------------------ */

export async function fetchDevicePage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.Device>>({
    url: '/api/v1/property/devices',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchCreateDevice(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/devices',
    data,
    successMessage: '设备新增成功',
    showSuccessMessage: true
  })
}

export function fetchUpdateDevice(id: string, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/property/devices/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteDevice(id: string) {
  return request.del<null>({
    url: `/api/v1/property/devices/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export function fetchGetDeviceOptions(communityId: string) {
  return request.get<Property.Option[]>({
    url: '/api/v1/property/devices/options',
    params: { communityId }
  })
}

export function fetchGetDeviceSummary(communityId: string) {
  return request.get<Property.DeviceStats>({
    url: '/api/v1/property/devices/summary',
    params: { communityId }
  })
}

/* ------------------------------ 门禁授权 ------------------------------ */

export async function fetchAccessGrantPage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.AccessGrant>>({
    url: '/api/v1/property/access/grants',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchGetAccessStats(communityId: string) {
  return request.get<Property.AccessStats>({
    url: '/api/v1/property/access/stats',
    params: { communityId }
  })
}

export function fetchIssuePersonAccess(data: { communityId: string; personType: number; personId: string }) {
  return request.post<{ grants: number }>({
    url: '/api/v1/property/access/issue',
    data,
    successMessage: '已提交下发',
    showSuccessMessage: true
  })
}

export function fetchRetryAccessGrants(data: { ids?: string[]; communityId?: string }) {
  return request.post<{ success: number; failed: number }>({
    url: '/api/v1/property/access/retry',
    data,
    successMessage: '重试完成',
    showSuccessMessage: true
  })
}

export function fetchRevokeAccessGrants(ids: string[]) {
  return request.post<{ revoked: number }>({
    url: '/api/v1/property/access/revoke',
    data: { ids },
    successMessage: '已解除授权',
    showSuccessMessage: true
  })
}

export function fetchGetAccessCards(params: { communityId?: string; personType?: number }) {
  return request.get<Property.AccessCard[]>({ url: '/api/v1/property/access/cards', params })
}

export function fetchCreateAccessCard(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/access/cards',
    data,
    successMessage: '门禁卡已登记',
    showSuccessMessage: true
  })
}

export function fetchDeleteAccessCard(id: string) {
  return request.del<null>({
    url: `/api/v1/property/access/cards/${id}`,
    successMessage: '门禁卡已注销',
    showSuccessMessage: true
  })
}

/* ------------------------------ 收费 ------------------------------ */

export async function fetchFeeItemPage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.FeeItem>>({
    url: '/api/v1/property/fee/items',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchGetFeeItems(communityId?: string) {
  return request.get<Property.Page<Property.FeeItem>>({
    url: '/api/v1/property/fee/items',
    params: { page: 1, pageSize: 200, communityId }
  })
}

export function fetchCreateFeeItem(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/fee/items',
    data,
    successMessage: '收费项目已新增',
    showSuccessMessage: true
  })
}

export function fetchUpdateFeeItem(id: string, data: Record<string, any>) {
  return request.put<null>({
    url: `/api/v1/property/fee/items/${id}`,
    data,
    successMessage: '保存成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteFeeItem(id: string) {
  return request.del<null>({
    url: `/api/v1/property/fee/items/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export function fetchGetFeeStandards(itemId: string) {
  return request.get<Property.FeeStandard[]>({ url: `/api/v1/property/fee/items/${itemId}/standards` })
}

export function fetchCreateFeeStandard(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/fee/standards',
    data,
    successMessage: '收费标准已新增',
    showSuccessMessage: true
  })
}

export function fetchDeleteFeeStandard(id: string) {
  return request.del<null>({
    url: `/api/v1/property/fee/standards/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export async function fetchMeterPage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.Meter>>({
    url: '/api/v1/property/fee/meters',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchCreateMeter(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/fee/meters',
    data,
    successMessage: '水表新增成功',
    showSuccessMessage: true
  })
}

export function fetchDeleteMeter(id: string) {
  return request.del<null>({
    url: `/api/v1/property/fee/meters/${id}`,
    successMessage: '删除成功',
    showSuccessMessage: true
  })
}

export async function fetchMeterReadingPage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.MeterReading>>({
    url: '/api/v1/property/fee/readings',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchCreateMeterReading(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/fee/readings',
    data,
    successMessage: '抄表已记录',
    showSuccessMessage: true
  })
}

export function fetchGenerateBills(data: Record<string, any>) {
  return request.post<Property.GenerateResult>({
    url: '/api/v1/property/fee/bills/generate',
    data,
    successMessage: '账单生成完成',
    showSuccessMessage: true
  })
}

export function fetchGenerateMeterBills(data: Record<string, any>) {
  return request.post<Property.GenerateResult>({
    url: '/api/v1/property/fee/bills/generate-meter',
    data,
    successMessage: '水电账单生成完成',
    showSuccessMessage: true
  })
}

export async function fetchBillPage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.Bill>>({
    url: '/api/v1/property/fee/bills',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchGetBill(id: string) {
  return request.get<Property.Bill>({ url: `/api/v1/property/fee/bills/${id}` })
}

export function fetchPayBill(id: string, data: { payMethod: number; amount?: number; tradeNo?: string; remark?: string }) {
  return request.post<Property.PayResult>({
    url: `/api/v1/property/fee/bills/${id}/pay`,
    data,
    successMessage: '收费成功',
    showSuccessMessage: true
  })
}

export function fetchCancelBill(id: string) {
  return request.put<null>({
    url: `/api/v1/property/fee/bills/${id}/cancel`,
    successMessage: '账单已作废',
    showSuccessMessage: true
  })
}

export function fetchGetBillSummary(params: { communityId: string; billMonth?: string }) {
  return request.get<Property.BillSummary>({ url: '/api/v1/property/fee/summary', params })
}

export async function fetchPaymentPage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.Payment>>({
    url: '/api/v1/property/fee/payments',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchGetAccounts(params: { communityId?: string; keyword?: string }) {
  return request.get<Property.Account[]>({ url: '/api/v1/property/fee/accounts', params })
}

export function fetchRechargeAccount(data: Record<string, any>) {
  return request.post<{ id: string }>({
    url: '/api/v1/property/fee/accounts/recharge',
    data,
    successMessage: '充值成功',
    showSuccessMessage: true
  })
}

export async function fetchAccountLogPage(params: Record<string, any>) {
  const res = await request.get<Property.Page<Property.AccountLog>>({
    url: '/api/v1/property/fee/accounts/logs',
    params: toPageQuery(params)
  })
  return toTablePage(res)
}

export function fetchNodeChildren(communityId: string, parentId: string) {
  return request.get<Property.TreeNodeChild[]>({
    url: '/api/v1/property/nodes/children',
    params: { communityId, parentId }
  })
}

/** 层级信息卡片 */

export function fetchNodeCard(nodeId: string) {
  return request.get<Property.NodeCard>({ url: `/api/v1/property/nodes/${nodeId}/card` })
}

/** 房屋信息卡片（住户 + 车辆） */

export function fetchHouseCard(houseId: string) {
  return request.get<Property.HouseCard>({ url: `/api/v1/property/structures/houses/${houseId}/card` })
}

/** 小区信息卡片 */

export function fetchCommunityCard(communityId: string) {
  return request.get<Property.CommunityCard>({
    url: '/api/v1/property/spaces/community-card',
    params: { communityId }
  })
}

/** 房屋级联树（小区层级 + 房屋），供业务页面选房 */
export function fetchHouseCascader(communityId: string, usageType?: number) {
  return request.get<Property.CascaderNode[]>({
    url: '/api/v1/property/houses/cascader',
    params: { communityId, usageType }
  })
}

/** 清空小区层级与房屋（导入出错后的整体重置） */
export function fetchClearSpace(communityId: string) {
  return request.post<{ houses: number; nodes: number }>({
    // POST 的 params 会被封装层当成 body，这里显式放到 query 上
    url: `/api/v1/property/spaces/clear?communityId=${communityId}`,
    successMessage: '已清空层级与房屋',
    showSuccessMessage: true
  })
}

/** 小区基础信息概览（楼栋/单元/楼层/房屋/面积） */

export function fetchSpaceSummary(communityId: string) {
  return request.get<Property.SpaceSummary>({
    url: '/api/v1/property/spaces/summary',
    params: { communityId }
  })
}

/** 层级下拉（房屋/设备选择） */
