/**
 * 物业管理模块接口级 E2E（零依赖，Node 18+）
 *
 *   node scripts/e2e-property.mjs http://127.0.0.1:8090 admin
 *
 * 覆盖：登录 → 建物业公司(租户) → 建小区 → 楼宇结构与房屋 → 设备 → 住户登记与门禁下发
 *      → 访客/其他人员 → 收费项目与账单/缴费 → 概览 → 跨租户越权防护
 * 环境变量：E2E_ADMIN_PASSWORD（默认 Admin@123456）
 */
import { execFileSync } from 'node:child_process'

const BASE = (process.argv[2] || 'http://127.0.0.1:8090').replace(/\/$/, '')
const ADMIN = process.argv[3] || 'admin'
const PASSWORD = process.env.E2E_ADMIN_PASSWORD || 'Admin@123456'

let pass = 0
let fail = 0
const failures = []

function check(name, cond, extra = '') {
  if (cond) {
    pass++
    console.log(`  ✓ ${name}`)
  } else {
    fail++
    failures.push(name + (extra ? ` (${extra})` : ''))
    console.log(`  ✗ ${name}${extra ? ' → ' + extra : ''}`)
  }
}

async function api(method, path, body, token) {
  const headers = { 'Content-Type': 'application/json' }
  if (token) headers.Authorization = `Bearer ${token}`
  const res = await fetch(BASE + path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body)
  })
  const text = await res.text()
  let json
  try {
    json = JSON.parse(text)
  } catch {
    json = { code: res.status, message: text.slice(0, 200) }
  }
  return { status: res.status, ...json }
}

async function login(username, password) {
  // 验证码开关是运行时参数：开着就要先取验证码，并从 Redis 读回答案（与 e2e-api.mjs 一致）
  let captcha = { captchaId: '', captchaCode: '' }
  try {
    const pub = await api('GET', '/api/v1/system/configs/public')
    const flag = String(pub?.data?.['sys.login.captcha'] ?? 'true').toLowerCase()
    if (flag !== 'false') {
      const cap = await api('GET', '/api/v1/auth/captcha')
      const captchaId = cap?.data?.captchaId || ''
      if (captchaId) {
        captcha.captchaId = captchaId
        captcha.captchaCode = readCaptchaFromRedis(captchaId) || ''
      }
    }
  } catch {
    /* 取不到验证码时按关闭处理，登录失败会给出明确错误 */
  }
  const r = await api('POST', '/api/v1/auth/login', { username, password, ...captcha })
  return r
}

/** 从 Redis 里读出验证码答案（容器名/前缀可用环境变量覆盖） */
function readCaptchaFromRedis(captchaId) {
  const containers = [process.env.E2E_REDIS_CONTAINER, 'wb-redis', 'web-basic-redis', 'web-basic-redis57'].filter(Boolean)
  const prefix = process.env.E2E_REDIS_PREFIX || 'web:'
  try {
    for (const container of containers) {
      try {
        const v = execFileSync('docker', ['exec', container, 'redis-cli', 'GET', `${prefix}captcha:${captchaId}`], {
          encoding: 'utf8'
        }).trim()
        if (v) return v
      } catch {
        /* 换下一个候选容器 */
      }
    }
  } catch {
    /* 无 docker 环境时返回空 */
  }
  return ''
}

const suffix = Date.now().toString().slice(-6)

async function main() {
  console.log(`\n=== 物业管理 E2E @ ${BASE} ===\n`)

  // ---------- 1. 登录 ----------
  console.log('[1] 登录与权限')
  const admin = await login(ADMIN, PASSWORD)
  check('超管登录成功', admin.code === 0, admin.message)
  const token = admin.data?.accessToken
  if (!token) throw new Error('登录失败，无法继续：' + admin.message)

  // ---------- 2. 物业公司（租户） ----------
  console.log('\n[2] 物业公司（租户）')
  const companyCode = 'pm' + suffix
  const company = await api(
    'POST',
    '/api/v1/property/companies',
    {
      tenantCode: companyCode,
      name: `E2E物业公司${suffix}`,
      shortName: 'E2E物业',
      creditCode: '91440300' + suffix,
      contactName: '张三',
      contactPhone: '13800000000',
      level: '一级',
      maxUsers: 100
    },
    token
  )
  check('创建物业公司', company.code === 0, company.message)
  const tenantId = company.data?.tenantId
  const companyId = company.data?.companyId
  check('返回租户ID', !!tenantId)
  check('返回管理员账号', !!company.data?.adminUsername)
  const companyAdmin = { username: company.data?.adminUsername, password: company.data?.adminPassword }
  // 新公司只内置一个「物业公司管理员」角色
  const companyRoles = await api('GET', `/api/v1/system/roles?page=1&pageSize=50&tenantId=${tenantId}`, undefined, token)
  check('新物业公司只内置 1 个管理员角色', companyRoles.code === 0 && companyRoles.data.total === 1, String(companyRoles.data?.total))
  check('公司管理员角色覆盖本公司全部小区', (companyRoles.data?.list || [])[0]?.communityScope === 1, String((companyRoles.data?.list || [])[0]?.communityScope))

  const companies = await api('GET', '/api/v1/property/companies?page=1&pageSize=10', undefined, token)
  check('物业公司列表可查', companies.code === 0 && companies.data.total >= 1)

  // ---------- 3. 小区 ----------
  console.log('\n[3] 小区管理')
  const communityCode = 'C' + suffix
  const comm = await api(
    'POST',
    '/api/v1/property/communities',
    {
      tenantId,
      code: communityCode,
      name: `E2E花园${suffix}`,
      communityType: 1,
      province: '广东省',
      city: '深圳市',
      district: '南山区',
      districtCode: '440305',
      address: '科技园路 1 号',
      totalArea: 120000,
      buildingCount: 0,
      servicePhone: '0755-12345678'
    },
    token
  )
  check('创建小区', comm.code === 0, comm.message)
  const communityId = comm.data?.id
  check('返回小区ID', !!communityId)

  const dupComm = await api(
    'POST',
    '/api/v1/property/communities',
    { tenantId, code: communityCode, name: '重复编码' },
    token
  )
  check('小区编码重复被拒绝', dupComm.code !== 0)

  const options = await api('GET', '/api/v1/property/communities/options', undefined, token)
  check('小区下拉返回数据', options.code === 0 && Array.isArray(options.data) && options.data.length >= 1)

  const districts = await api('GET', '/api/v1/property/districts/tree', undefined, token)
  check('行政区划树可查（省/市两级）', districts.code === 0 && districts.data.length >= 30, String(districts.data?.length))

  // 五级行政区划：省 → 市 → 区县 → 街道 → 社区
  const provinces = await api('GET', '/api/v1/property/districts/children?level=1', undefined, token)
  check('行政区划：省级 ≥ 30', provinces.code === 0 && provinces.data.length >= 30, String(provinces.data?.length))
  const citiesOf = await api('GET', `/api/v1/property/districts/children?parentId=${provinces.data[0].id}`, undefined, token)
  check('行政区划：市级可下钻', citiesOf.code === 0 && citiesOf.data.length >= 1)
  const areasOf = await api('GET', `/api/v1/property/districts/children?parentId=${citiesOf.data[0].id}`, undefined, token)
  check('行政区划：区县可下钻', areasOf.code === 0 && areasOf.data.length >= 1)
  const streetsOf = await api('GET', `/api/v1/property/districts/children?parentId=${areasOf.data[0].id}`, undefined, token)
  check('行政区划：街道可下钻', streetsOf.code === 0 && streetsOf.data.length >= 1)
  const villagesOf = await api('GET', `/api/v1/property/districts/children?parentId=${streetsOf.data[0].id}`, undefined, token)
  check('行政区划：社区可下钻', villagesOf.code === 0 && villagesOf.data.length >= 1, String(villagesOf.data?.length))
  const pathOf = await api('GET', `/api/v1/property/districts/path?code=${villagesOf.data[0].code}`, undefined, token)
  check('行政区划：按编码还原省/市/区县/街道/社区路径', pathOf.code === 0 && pathOf.data.length === 5, String(pathOf.data?.length))

  // 参数校验提示必须是人话（不能出现 Key: 'XxxReq.Field' / Field validation 之类的原文）
  const badPwd = await api(
    'POST',
    '/api/v1/property/companies',
    { tenantCode: 'errcheck' + suffix, name: '提示测试物业', adminUsername: 'err' + suffix, adminPassword: '123' },
    token
  )
  check('密码过短：返回中文可读提示', badPwd.code === 10001 && /管理员密码/.test(badPwd.message) && !/validation|Key:/.test(badPwd.message), badPwd.message)
  const badField = await api('POST', '/api/v1/property/communities', { name: '缺编码' }, token)
  check('缺必填项：提示具体字段', badField.code === 10001 && /请填写/.test(badField.message) && !/validation|Key:/.test(badField.message), badField.message)

  // ---------- 4. 基础层级与批量房屋 ----------
  console.log('\n[4] 层级（区域/楼栋/单元/楼层）与批量房屋')
  const region = await api('POST', '/api/v1/property/nodes', { communityId, parentId: 0, name: '一期', nodeType: 1 }, token)
  check('新增区域层级', region.code === 0, region.message)
  const regionId = region.data?.id

  const building = await api('POST', '/api/v1/property/nodes', { communityId, parentId: regionId, name: '1栋', nodeType: 2 }, token)
  check('新增楼栋层级', building.code === 0, building.message)
  const buildingId = building.data?.id

  const unit = await api('POST', '/api/v1/property/nodes', { communityId, parentId: buildingId, name: '1单元', nodeType: 3 }, token)
  check('新增单元层级', unit.code === 0, unit.message)
  const unitId = unit.data?.id

  const batch = await api(
    'POST',
    '/api/v1/property/structures/houses/batch',
    { communityId, nodeId: unitId, floorFrom: 1, floorTo: 3, housesPerFloor: 2, usageType: 1, areaBuild: 89.5 },
    token
  )
  check('批量生成房屋（3层×2户=6套，自动建楼层）', batch.code === 0 && batch.data.created === 6, batch.message || String(batch.data?.created))

  const houses = await api('GET', `/api/v1/property/structures/houses?communityId=${communityId}&page=1&pageSize=50`, undefined, token)
  check('房屋列表可查', houses.code === 0 && houses.data.total === 6, String(houses.data?.total))
  const firstHouse = houses.data?.list?.[0]
  check('房屋带层级名称', !!firstHouse?.buildingName)

  const tree = await api('GET', `/api/v1/property/nodes/tree?communityId=${communityId}`, undefined, token)
  check('层级树返回区域节点', tree.code === 0 && tree.data.length >= 1 && (tree.data || []).some((n) => n.typeName === '区域'))

  const commAfter = await api('GET', `/api/v1/property/communities/${communityId}`, undefined, token)
  check('小区房屋数已回填', commAfter.code === 0 && Number(commAfter.data.houseCount) === 6, String(commAfter.data?.houseCount))

  // ---------- 4.1 基础信息（层级+房屋富字段、导入、导出、统计） ----------
  console.log('\n[4.1] 基础信息（层级/房屋/导入/导出）')
  const villaNode = await api(
    'POST',
    '/api/v1/property/nodes',
    { communityId, name: '别墅区' + suffix, nodeType: 2, category: 2, floorsAbove: 3, buildArea: 320, buildYear: 2018, structureType: '框架' },
    token
  )
  check('新增别墅层级（含类别/层数/面积）', villaNode.code === 0, villaNode.message)
  const nodeTree = await api('GET', `/api/v1/property/nodes/tree?communityId=${communityId}`, undefined, token)
  const villaRow = (nodeTree.data || []).find((n) => n.id === String(villaNode.data?.id))
  check('层级回显类别与面积字段', !!villaRow && villaRow.category === 2 && Number(villaRow.buildArea) === 320, JSON.stringify({ category: villaRow?.category, area: villaRow?.buildArea }))

  const villaHouse = await api(
    'POST',
    '/api/v1/property/houses'.replace('/houses', '/structures/houses'),
    {
      communityId,
      nodeId: villaNode.data?.id,
      code: 'V-' + suffix,
      houseNo: '1号',
      usageType: 1,
      areaBuild: 320,
      areaUse: 280,
      areaShare: 0,
      layout: '5室3厅',
      orientation: '南',
      decoration: 3,
      propertyRight: 1,
      ceilingHeight: 3.3,
      facilities: '地暖/中央空调',
      isSale: true
    },
    token
  )
  check('新增别墅房屋（富字段）', villaHouse.code === 0, villaHouse.message)

  const shopHouse = await api(
    'POST',
    '/api/v1/property/structures/houses',
    {
      communityId,
      nodeId: villaNode.data?.id,
      code: 'S-' + suffix,
      houseNo: '101铺',
      usageType: 2,
      areaBuild: 120,
      areaUse: 110,
      propertyRight: 1,
      isStreet: true,
      shopWidth: 8.5,
      ceilingHeight: 4.5,
      houseStatus: 3
    },
    token
  )
  check('新增临街商铺（面积/门宽/临街）', shopHouse.code === 0, shopHouse.message)

  const shopList = await api('GET', `/api/v1/property/structures/houses?communityId=${communityId}&nodeId=${villaNode.data?.id}&page=1&pageSize=50`, undefined, token)
  const shopRow = (shopList.data?.list || []).find((h) => h.code === 'S-' + suffix)
  check('房屋回显公摊/门宽/临街/层高', !!shopRow && Number(shopRow.areaShare) === 0 && Number(shopRow.shopWidth) === 8.5 && shopRow.isStreet === true && Number(shopRow.ceilingHeight) === 4.5, JSON.stringify({ share: shopRow?.areaShare, width: shopRow?.shopWidth, street: shopRow?.isStreet }))

  const summary1 = await api('GET', `/api/v1/property/spaces/summary?communityId=${communityId}`, undefined, token)
  check('基础信息概览（楼栋/房屋/面积）', summary1.code === 0 && summary1.data.houses > 0 && summary1.data.buildings >= 1, JSON.stringify(summary1.data))

  // 房屋地址导入：模板三列（编码/地址/面积），自动补齐层级
  const tpl = await fetch(BASE + `/api/v1/property/structures/import-template?communityId=${communityId}`, {
    headers: { Authorization: `Bearer ${token}` }
  })
  const tplBuf = await tpl.arrayBuffer()
  check('导入模板可下载（三列）', tpl.ok && tplBuf.byteLength > 500, String(tplBuf.byteLength))
  const uploadSpaceTemplate = async (cid) => {
    const fd = new FormData()
    fd.append('file', new Blob([tplBuf]), '小区基础信息导入模板.xlsx')
    return (
      await fetch(BASE + `/api/v1/property/structures/import?communityId=${cid}`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
        body: fd
      })
    ).json()
  }
  const imported = await uploadSpaceTemplate(communityId)
  check('按房屋地址导入成功（自动补齐层级）', imported.code === 0 && imported.data?.success >= 3, JSON.stringify(imported.data?.errors || imported.data))

  const treeAfterImport = await api('GET', `/api/v1/property/nodes/tree?communityId=${communityId}`, undefined, token)
  const nodeNames = (treeAfterImport.data || []).map((n) => n.name)
  check('导入后生成层级：1栋/4单元/1楼', ['1栋', '4单元', '1楼'].every((n) => nodeNames.includes(n)), nodeNames.join(','))

  const importedHouses = await api('GET', `/api/v1/property/structures/houses?communityId=${communityId}&keyword=`, undefined, token)
  check('导入的房屋已挂载并带地址全称', (importedHouses.data?.list || []).some((h) => String(h.fullName).includes('1栋4单元1楼1号')), String(importedHouses.data?.total))
  const impCom = await api('GET', `/api/v1/property/communities/${communityId}`, undefined, token)
  const impBase = String(impCom.data?.address || '') + String(impCom.data?.name || '')
  const impNode = (treeAfterImport.data || []).find((n) => n.name === '1栋')
  check(
    '导入生成的层级地址包含小区地址+小区名称',
    !!impNode && String(impNode.fullAddress || '').startsWith(impBase),
    String(impNode?.fullAddress)
  )
  const impHouse = (importedHouses.data?.list || []).find((h) => String(h.fullName).includes('1栋4单元1楼1号'))
  check(
    '导入生成的房屋全称包含小区地址+小区名称',
    !!impHouse && String(impHouse.fullName || '').startsWith(impBase),
    String(impHouse?.fullName)
  )

  const housesBeforeDup = await api('GET', `/api/v1/property/structures/houses?communityId=${communityId}&page=1&pageSize=1`, undefined, token)
  const cascader = await api('GET', `/api/v1/property/houses/cascader?communityId=${communityId}`, undefined, token)
  const walkCascader = (list, acc = { nodes: 0, houses: 0, houseLabels: [] }) => {
    ;(list || []).forEach((n) => {
      if (n.isHouse) {
        acc.houses++
        acc.houseLabels.push(n.label)
      } else {
        acc.nodes++
        walkCascader(n.children, acc)
      }
    })
    return acc
  }
  const cascStats = walkCascader(cascader.data)
  check(
    '房屋级联树包含层级与房屋叶子(房号)',
    cascader.code === 0 &&
      cascStats.nodes >= 3 &&
      cascStats.houses >= 3 &&
      cascStats.houseLabels.every((l) => l && l.length > 0) &&
      cascStats.houseLabels.some((l) => /号$/.test(l)),
    JSON.stringify({ nodes: cascStats.nodes, houses: cascStats.houses, labels: cascStats.houseLabels.slice(0, 3) })
  )

  const dupImport = await uploadSpaceTemplate(communityId)
  check(
    '重复导入整批拒绝（不写入任何数据）',
    dupImport.code === 0 &&
      dupImport.data?.aborted === true &&
      dupImport.data?.success === 0 &&
      dupImport.data?.failed >= 3 &&
      /已存在/.test(JSON.stringify(dupImport.data?.errors)),
    JSON.stringify(dupImport.data?.errors || {}).slice(0, 80)
  )
  const housesAfterDup = await api('GET', `/api/v1/property/structures/houses?communityId=${communityId}&page=1&pageSize=1`, undefined, token)
  check('校验失败时房屋总数不变', housesAfterDup.data?.total === housesBeforeDup.data?.total, String(housesBeforeDup.data?.total) + '→' + String(housesAfterDup.data?.total))

  // 部分行有错（一条重复）→ 整批不导入，证明不是"导一半"
  const atomicCom = await api('POST', '/api/v1/property/communities', { tenantId, code: 'ATM' + suffix, name: '原子导入小区' + suffix, address: '原子省原子市原子区原子路1号' }, token)
  let atomicRes = { code: 0, data: {} }
  let atomicHouses = 0
  if (atomicCom.code === 0) {
    await api('POST', '/api/v1/property/structures/houses', { communityId: atomicCom.data?.id, code: '1-4-101', houseNo: '1号', usageType: 1, areaBuild: 50 }, token)
    const tplAtomic = await fetch(BASE + `/api/v1/property/structures/import-template?communityId=${atomicCom.data?.id}`, { headers: { Authorization: `Bearer ${token}` } })
    const atomicBuf = await tplAtomic.arrayBuffer()
    const fd2 = new FormData()
    fd2.append('file', new Blob([atomicBuf]), 't.xlsx')
    atomicRes = await (
      await fetch(BASE + `/api/v1/property/structures/import?communityId=${atomicCom.data?.id}`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
        body: fd2
      })
    ).json()
    const listAtomic = await api('GET', `/api/v1/property/structures/houses?communityId=${atomicCom.data?.id}&page=1&pageSize=10`, undefined, token)
    atomicHouses = listAtomic.data?.total ?? 0
  }
  check(
    '部分行有错时整批回滚（仅保留原有1套）',
    atomicCom.code === 0 && atomicRes.data?.aborted === true && atomicRes.data?.success === 0 && atomicHouses === 1,
    `aborted=${atomicRes.data?.aborted} success=${atomicRes.data?.success} houses=${atomicHouses}`
  )

  // 地址与目标小区不匹配 → 全部失败且提示应以前缀开头
  const otherCom = await api('POST', '/api/v1/property/communities', { tenantId, code: 'IMP' + suffix, name: '导入校验小区' + suffix, address: '某省某市某区某路1号' }, token)
  let mismatch = { code: 0, data: {} }
  if (otherCom.code === 0) mismatch = await uploadSpaceTemplate(otherCom.data?.id)
  check(
    '地址与小区不匹配的数据被明确拒绝且不导入',
    otherCom.code === 0 && mismatch.code === 0 && mismatch.data?.aborted === true && mismatch.data?.failed >= 3 && /不匹配/.test(JSON.stringify(mismatch.data?.errors)),
    JSON.stringify(mismatch.data?.errors || {}).slice(0, 80)
  )

  const exportRes = await fetch(BASE + `/api/v1/property/spaces/export?communityId=${communityId}`, { headers: { Authorization: `Bearer ${token}` } })
  const exportBuf = await exportRes.arrayBuffer()
  check('基础信息可导出 Excel', exportRes.ok && exportBuf.byteLength > 1000, String(exportBuf.byteLength))

  // ---------- 4.3 清空层级与房屋 ----------
  console.log('\n[4.3] 清空层级与房屋（导入出错后的重置）')
  const clearCom = await api('POST', '/api/v1/property/communities', { tenantId, code: 'CLR' + suffix, name: '清理测试小区' + suffix, address: '清理省清理市清理区清理路1号' }, token)
  if (clearCom.code === 0) {
    const cid = clearCom.data?.id
    const tplClear = await fetch(BASE + `/api/v1/property/structures/import-template?communityId=${cid}`, { headers: { Authorization: `Bearer ${token}` } })
    const bufClear = await tplClear.arrayBuffer()
    const uClear = async () => {
      const fd = new FormData()
      fd.append('file', new Blob([bufClear]), 'c.xlsx')
      return (await fetch(BASE + `/api/v1/property/structures/import?communityId=${cid}`, { method: 'POST', headers: { Authorization: `Bearer ${token}` }, body: fd })).json()
    }
    const impClear = await uClear()
    check('清理前置：导入 3 套房屋', impClear.code === 0 && impClear.data?.success >= 3, JSON.stringify(impClear.data?.errors || impClear.data))
    const housesClear = await api('GET', `/api/v1/property/structures/houses?communityId=${cid}&page=1&pageSize=10`, undefined, token)
    const oneHouse = (housesClear.data?.list || [])[0]
    const vehClear = await api('POST', '/api/v1/property/vehicles', { communityId: cid, plateNo: '清A' + suffix, plateColor: 1, vehicleType: 1, ownerType: 1, houseId: oneHouse?.id }, token)
    const blockedClear = await api('POST', `/api/v1/property/spaces/clear?communityId=${cid}`, undefined, token)
    check('有车辆登记时拒绝清空', blockedClear.code !== 0 && /车辆/.test(blockedClear.message), blockedClear.message)
    if (vehClear.code === 0) await api('DELETE', `/api/v1/property/vehicles/${vehClear.data?.id}`, undefined, token)
    const clearStats = await api('POST', '/api/v1/property/spaces/clear', { communityId: cid }, token)
    check('清空层级与房屋成功', clearStats.code === 0 && (clearStats.data?.houses ?? 0) >= 3 && (clearStats.data?.nodes ?? 0) >= 3, JSON.stringify(clearStats.data))
    const afterHouses = await api('GET', `/api/v1/property/structures/houses?communityId=${cid}&page=1&pageSize=10`, undefined, token)
    const afterNodes = await api('GET', `/api/v1/property/nodes/tree?communityId=${cid}`, undefined, token)
    const afterSum = await api('GET', `/api/v1/property/spaces/summary?communityId=${cid}`, undefined, token)
    check('清空后房屋/层级/统计均为空', afterHouses.data?.total === 0 && (afterNodes.data || []).length === 0 && (afterSum.data?.buildings ?? -1) === 0, JSON.stringify({ houses: afterHouses.data?.total, nodes: (afterNodes.data || []).length, buildings: afterSum.data?.buildings }))
  }

  // ---------- 4.2 信息卡片（层级/房屋/小区）与"直接挂小区"的房屋 ----------
  console.log('\n[4.2] 信息卡片与小区直挂房屋')
  const newLeaf = await api('POST', '/api/v1/property/nodes', { communityId, name: '2栋' + suffix, nodeType: 2, parentId: 0 }, token)
  check('新增层级（上级+名称+类型）', newLeaf.code === 0, newLeaf.message)
  const dupLeaf = await api('POST', '/api/v1/property/nodes', { communityId, name: '2栋' + suffix, nodeType: 2, parentId: 0 }, token)
  check('同一上级下名称唯一', dupLeaf.code !== 0 && /同名/.test(dupLeaf.message), dupLeaf.message)
  const badType = await api('POST', '/api/v1/property/nodes', { communityId, name: '非法类型' + suffix, nodeType: 9, parentId: 0 }, token)
  check('层级类型必须来自字典(1-4)', badType.code !== 0, badType.message)

  const tree2 = await api('GET', `/api/v1/property/nodes/tree?communityId=${communityId}`, undefined, token)
  const leafRow = (tree2.data || []).find((n) => n.id === String(newLeaf.data?.id))
  check('层级地址自动生成（小区地址+名称）', !!leafRow && String(leafRow.fullAddress || '').includes('2栋' + suffix), String(leafRow?.fullAddress))
  check('层级无编码输入也可用（自动生成）', !!leafRow && String(leafRow.code || '').length > 0, String(leafRow?.code))
  check('层级类型名回显', leafRow?.typeName === '楼栋', String(leafRow?.typeName))
  const comDetail = await api('GET', `/api/v1/property/communities/${communityId}`, undefined, token)
  const comBase = String(comDetail.data?.address || '') + String(comDetail.data?.name || '')
  check(
    '层级地址包含小区地址+小区名称',
    !!leafRow && comBase.length > 0 && String(leafRow.fullAddress || '').startsWith(comBase),
    String(leafRow?.fullAddress)
  )

  const nodeCard2 = await api('GET', `/api/v1/property/nodes/${newLeaf.data?.id}/card`, undefined, token)
  check('层级卡片（名称/地址/范围/房屋数/住户/设备）', nodeCard2.code === 0 && nodeCard2.data.typeName === '楼栋' && !!nodeCard2.data.address && Array.isArray(nodeCard2.data.childNames) && Array.isArray(nodeCard2.data.devices), JSON.stringify(nodeCard2.data).slice(0, 120))

  const rootHouse = await api('POST', '/api/v1/property/structures/houses', { communityId, code: 'R-' + suffix, houseNo: '小区直挂' + suffix, usageType: 1, areaBuild: 66.6 }, token)
  check('房屋可直接挂在小区的房屋', rootHouse.code === 0, rootHouse.message)
  const rootList = await api('GET', `/api/v1/property/structures/houses?communityId=${communityId}&rootOnly=true&page=1&pageSize=50`, undefined, token)
  check('小区直挂房屋可在根级列出', rootList.code === 0 && (rootList.data?.list || []).some((h) => h.code === 'R-' + suffix), String(rootList.data?.total))

  const houseCard2 = await api('GET', `/api/v1/property/structures/houses/${rootHouse.data?.id}/card`, undefined, token)
  check('房屋卡片含住户与车辆数组', houseCard2.code === 0 && Array.isArray(houseCard2.data.residents) && Array.isArray(houseCard2.data.vehicles) && !!houseCard2.data.address, JSON.stringify(houseCard2.data).slice(0, 100))

  const commCard = await api('GET', `/api/v1/property/spaces/community-card?communityId=${communityId}`, undefined, token)
  check('小区卡片含楼栋/房屋/住户/设备', commCard.code === 0 && commCard.data.buildings >= 1 && commCard.data.houses >= 1, JSON.stringify(commCard.data).slice(0, 120))

  // ---------- 5. 设备 ----------
  console.log('\n[5] 设备与层级绑定')
  const device = await api(
    'POST',
    '/api/v1/property/devices',
    {
      communityId,
      code: 'D' + suffix,
      name: '1栋门禁机',
      deviceType: 1,
      brand: '海康',
      serialNo: 'SN' + suffix,
      bindLevel: 3,
      bindId: buildingId,
      accessEnabled: true,
      status: 1
    },
    token
  )
  check('新增门禁设备（绑定楼栋）', device.code === 0, device.message)
  const deviceId = device.data?.id

  const gate = await api(
    'POST',
    '/api/v1/property/devices',
    { communityId, code: 'G' + suffix, name: '南门道闸', deviceType: 2, bindLevel: 1, accessEnabled: false },
    token
  )
  check('新增道闸设备（小区级）', gate.code === 0, gate.message)

  const deviceSummary = await api('GET', `/api/v1/property/devices/summary?communityId=${communityId}`, undefined, token)
  check('设备概览统计', deviceSummary.code === 0 && deviceSummary.data.total === 2, String(deviceSummary.data?.total))

  const deviceOpts = await api('GET', `/api/v1/property/devices/options?communityId=${communityId}`, undefined, token)
  check('门禁设备下拉只含参与授权的设备', deviceOpts.code === 0 && deviceOpts.data.length === 1, String(deviceOpts.data?.length))

  // ---------- 6. 住户 ----------
  console.log('\n[6] 住户登记 / 房屋关系 / 门禁下发')
  const idNo = '44030519900101' + suffix.slice(0, 4)
  const resident = await api(
    'POST',
    '/api/v1/property/residents',
    {
      name: '李四',
      idType: 1,
      idNo,
      gender: 1,
      phone: '13900000001',
      communityId,
      houseId: firstHouse?.id,
      relRole: 1,
      relation: '本人',
      isMain: true
    },
    token
  )
  check('登记住户并绑定房屋', resident.code === 0, resident.message)
  const residentId = resident.data?.residentId
  check('返回住户ID', !!residentId)

  const lookup = await api('GET', `/api/v1/property/residents/lookup?idType=1&idNo=${encodeURIComponent(idNo)}`, undefined, token)
  check('按证件号回显', lookup.code === 0 && lookup.data.found === true && lookup.data.name === '李四')

  const relist = await api('GET', `/api/v1/property/residents?communityId=${communityId}`, undefined, token)
  check('住户列表可见该房屋关系', relist.code === 0 && relist.data.total >= 1, String(relist.data?.total))

  const residentDetail = await api('GET', `/api/v1/property/residents/${residentId}`, undefined, token)
  check('住户详情含房屋', residentDetail.code === 0 && residentDetail.data.houses.length >= 1)

  const grants = await api('GET', `/api/v1/property/access/grants?communityId=${communityId}`, undefined, token)
  check('登记后自动产生门禁授权', grants.code === 0 && grants.data.total >= 1, String(grants.data?.total))
  const grantFailed = (grants.data?.list || []).filter((g) => Number(g.status) === 4).length
  check('未对接平台时下发标记为失败（可重试）', grantFailed >= 1, String(grantFailed))

  const retry = await api('POST', '/api/v1/property/access/retry', { communityId }, token)
  check('失败授权可重试', retry.code === 0, retry.message)

  const accessStats = await api('GET', `/api/v1/property/access/stats?communityId=${communityId}`, undefined, token)
  check('授权统计可查', accessStats.code === 0 && accessStats.data.total >= 1)

  // 租户住户有效期（1天后到期）→ 续期
  const tenantRel = await api(
    'POST',
    '/api/v1/property/residents/bind',
    {
      residentId,
      houseId: firstHouse?.id,
      communityId,
      relRole: 3,
      relation: '租户本人',
      startDate: new Date().toISOString().slice(0, 10),
      endDate: new Date(Date.now() + 86400000).toISOString().slice(0, 10)
    },
    token
  )
  // 同一房屋同一住户已有有效关系，绑定幂等返回原关系
  check('同一房屋重复绑定幂等', tenantRel.code === 0)

  const houseResidents = await api('GET', `/api/v1/property/residents/houses?houseId=${firstHouse?.id}`, undefined, token)
  check('房屋住户列表', houseResidents.code === 0 && houseResidents.data.length >= 1)

  const history = await api('GET', `/api/v1/property/residents/${residentId}/history`, undefined, token)
  check('关系变更历史可查', history.code === 0 && history.data.length >= 1)

  // ---------- 7. 访客 ----------
  console.log('\n[7] 访客登记')
  const visitor = await api(
    'POST',
    '/api/v1/property/visitors',
    {
      communityId,
      name: '王五',
      phone: '13700000002',
      visitType: 1,
      visitReason: '探亲',
      visitHouseId: firstHouse?.id,
      peopleCount: 2,
      accompanies: [{ name: '王小明', relation: '子女' }]
    },
    token
  )
  check('访客登记成功', visitor.code === 0, visitor.message)
  const visitorId = visitor.data?.id

  const visitorList = await api('GET', `/api/v1/property/visitors?communityId=${communityId}`, undefined, token)
  check('访客列表可查', visitorList.code === 0 && visitorList.data.total >= 1)

  const visitorEnter = await api('PUT', `/api/v1/property/visitors/${visitorId}/status`, { action: 'enter' }, token)
  check('访客进入', visitorEnter.code === 0, visitorEnter.message)
  const visitorLeave = await api('PUT', `/api/v1/property/visitors/${visitorId}/status`, { action: 'leave' }, token)
  check('访客离场', visitorLeave.code === 0, visitorLeave.message)

  // ---------- 8. 其他人员 ----------
  console.log('\n[8] 其他人员与类别')
  const category = await api(
    'POST',
    '/api/v1/property/person-categories',
    { communityId, name: '保洁' + suffix, needFace: true, defaultDays: 7 },
    token
  )
  check('新增人员类别', category.code === 0, category.message)
  const categoryId = category.data?.id

  const otherPerson = await api(
    'POST',
    '/api/v1/property/other-people',
    {
      communityId,
      categoryId,
      name: '赵六',
      phone: '13600000003',
      companyName: '洁净保洁公司',
      workContent: '公共区域保洁',
      faceUrl: '/static/demo/face.jpg',
      deviceIds: [deviceId]
    },
    token
  )
  check('登记其他人员并勾选设备', otherPerson.code === 0, otherPerson.message)

  const otherList = await api('GET', `/api/v1/property/other-people?communityId=${communityId}`, undefined, token)
  check('其他人员列表', otherList.code === 0 && otherList.data.total >= 1)

  // ---------- 9. 单位/商铺 ----------
  console.log('\n[9] 单位（商铺）')
  const merchant = await api(
    'POST',
    '/api/v1/property/merchants',
    {
      communityId,
      name: '便利超市' + suffix,
      merchantType: 2,
      houseId: firstHouse?.id,
      contactName: '店长',
      contactPhone: '13500000004',
      area: 120
    },
    token
  )
  check('登记单位', merchant.code === 0, merchant.message)

  // ---------- 9.1 房屋关联必须是真实 ID ----------
  console.log('\n[9.1] 房屋必须用 ID 关联')
  const merchantList = await api('GET', `/api/v1/property/merchants?communityId=${communityId}&keyword=`, undefined, token)
  const merchantRow = (merchantList.data?.list || []).find((m) => String(m.name).includes(suffix))
  check('单位记录保存了房屋ID', !!merchantRow && merchantRow.houseId === String(firstHouse?.id), JSON.stringify({ houseId: merchantRow?.houseId, expect: firstHouse?.id }))
  check('单位记录回显房屋名称', !!merchantRow?.houseName, String(merchantRow?.houseName))

  const residentRows = await api('GET', `/api/v1/property/residents?communityId=${communityId}`, undefined, token)
  const residentRow = (residentRows.data?.list || []).find((r) => r.residentId === String(residentId))
  check('住户房屋关系保存了房屋ID', !!residentRow && residentRow.houseId === String(firstHouse?.id), String(residentRow?.houseId))

  const visitorRows = await api('GET', `/api/v1/property/visitors?communityId=${communityId}`, undefined, token)
  const visitorRow = (visitorRows.data?.list || []).find((v) => v.id === String(visitorId))
  check('访客登记保存了受访房屋ID', !!visitorRow && visitorRow.visitHouseId === String(firstHouse?.id), String(visitorRow?.visitHouseId))
  check('访客记录回显房屋名称', !!visitorRow?.visitHouseName, String(visitorRow?.visitHouseName))

  // ---------- 10. 车辆 ----------
  console.log('\n[10] 车辆与车位')
  const space = await api(
    'POST',
    '/api/v1/property/parking-spaces',
    { communityId, code: 'P' + suffix, spaceNo: 'B1-001', spaceType: 1, area: 12 },
    token
  )
  check('新增车位', space.code === 0, space.message)
  const spaceId = space.data?.id

  const vehicle = await api(
    'POST',
    '/api/v1/property/vehicles',
    {
      communityId,
      plateNo: '粤B' + suffix,
      plateColor: 1,
      vehicleType: 1,
      ownerType: 1,
      residentId,
      houseId: firstHouse?.id,
      parkingSpaceId: spaceId,
      brand: '比亚迪'
    },
    token
  )
  check('登记业主车辆并绑定车位', vehicle.code === 0, vehicle.message)
  const vehicleRows = await api('GET', `/api/v1/property/vehicles?communityId=${communityId}`, undefined, token)
  const vehicleRow = (vehicleRows.data?.list || []).find((v) => v.plateNo === '粤B' + suffix)
  check('车辆记录保存了房屋ID', !!vehicleRow && vehicleRow.houseId === String(firstHouse?.id), String(vehicleRow?.houseId))

  const dupVehicle = await api(
    'POST',
    '/api/v1/property/vehicles',
    { communityId, plateNo: '粤B' + suffix, vehicleType: 1 },
    token
  )
  check('同小区车牌重复被拒绝', dupVehicle.code !== 0)

  // ---------- 11. 收费 ----------
  console.log('\n[11] 收费项目 / 账单 / 缴费')
  const feeItem = await api(
    'POST',
    '/api/v1/property/fee/items',
    {
      communityId,
      code: 'WYF',
      name: '物业费',
      itemType: 1,
      calcMode: 1,
      unitPrice: 3.5,
      houseUsage: 1,
      dueDays: 30
    },
    token
  )
  check('新增收费项目（按面积·月）', feeItem.code === 0, feeItem.message)
  const feeItemId = feeItem.data?.id

  const month = new Date().toISOString().slice(0, 7).replace('-', '')
  const generate = await api(
    'POST',
    '/api/v1/property/fee/bills/generate',
    { communityId, feeItemId, billMonth: month },
    token
  )
  check('生成当月账单', generate.code === 0 && generate.data.created > 0, generate.message || String(generate.data?.created))

  const generateAgain = await api(
    'POST',
    '/api/v1/property/fee/bills/generate',
    { communityId, feeItemId, billMonth: month },
    token
  )
  check('重复生成幂等（全部跳过）', generateAgain.code === 0 && generateAgain.data.created === 0, String(generateAgain.data?.created))

  const bills = await api('GET', `/api/v1/property/fee/bills?communityId=${communityId}&billMonth=${month}`, undefined, token)
  check('账单列表可查', bills.code === 0 && bills.data.total > 0, String(bills.data?.total))
  const bill = bills.data?.list?.[0]

  const summary = await api('GET', `/api/v1/property/fee/summary?communityId=${communityId}&billMonth=${month}`, undefined, token)
  check('收费汇总', summary.code === 0 && summary.data.receivable > 0, String(summary.data?.receivable))

  const pay = await api('POST', `/api/v1/property/fee/bills/${bill?.id}/pay`, { payMethod: 3 }, token)
  check('现金缴费成功', pay.code === 0, pay.message)
  check('缴费后账单状态为已缴', pay.code === 0 && Number(pay.data.status) === 3, String(pay.data?.status))

  const payAgain = await api('POST', `/api/v1/property/fee/bills/${bill?.id}/pay`, { payMethod: 3 }, token)
  check('已缴账单不可重复缴费', payAgain.code !== 0)

  const payments = await api('GET', `/api/v1/property/fee/payments?communityId=${communityId}`, undefined, token)
  check('缴费流水可查', payments.code === 0 && payments.data.total >= 1)

  // 抄表 → 生成水电账单
  const meter = await api(
    'POST',
    '/api/v1/property/fee/meters',
    { communityId, houseId: firstHouse?.id, meterType: 1, meterNo: 'WM' + suffix, ratio: 1, initReading: 100 },
    token
  )
  check('新增水表', meter.code === 0, meter.message)
  const meterId = meter.data?.id

  const reading = await api(
    'POST',
    '/api/v1/property/fee/readings',
    { meterId, period: month, currReading: 130, unitPrice: 2.8 },
    token
  )
  check('录入抄表（用量30）', reading.code === 0, reading.message)

  const waterItem = await api(
    'POST',
    '/api/v1/property/fee/items',
    { communityId, code: 'SF', name: '水费', itemType: 2, calcMode: 3, unitPrice: 2.8 },
    token
  )
  check('新增水费项目（按用量）', waterItem.code === 0)

  const meterBills = await api(
    'POST',
    '/api/v1/property/fee/bills/generate-meter',
    { communityId, feeItemId: waterItem.data?.id, period: month },
    token
  )
  check('按抄表生成水电账单', meterBills.code === 0 && meterBills.data.created === 1, meterBills.message || String(meterBills.data?.created))

  // 账户充值 → 余额划扣缴费
  const recharge = await api(
    'POST',
    '/api/v1/property/fee/accounts/recharge',
    { communityId, ownerType: 1, residentId, targetType: 1, amount: 500, payMethod: 3 },
    token
  )
  check('账户充值成功', recharge.code === 0, recharge.message)

  const accounts = await api('GET', `/api/v1/property/fee/accounts?communityId=${communityId}`, undefined, token)
  check('账户余额可查', accounts.code === 0 && (accounts.data || []).length >= 1)

  const waterBills = await api('GET', `/api/v1/property/fee/bills?communityId=${communityId}&feeItemId=${waterItem.data?.id}`, undefined, token)
  const waterBill = waterBills.data?.list?.[0]
  if (waterBill) {
    const balancePay = await api('POST', `/api/v1/property/fee/bills/${waterBill.id}/pay`, { payMethod: 6 }, token)
    check('余额划扣缴费成功', balancePay.code === 0, balancePay.message)
    const accAfter = await api('GET', `/api/v1/property/fee/accounts?communityId=${communityId}`, undefined, token)
    const acc = (accAfter.data || []).find((a) => String(a.residentId) === String(residentId))
    check('余额已扣减', !!acc && acc.balance < 500, String(acc?.balance))
  }

  // ---------- 11.1 Excel 导入（楼宇结构 / 设备） ----------
  console.log('\n[11.1] Excel 导入')
  const uploadTemplate = async (tplPath, importPath, filename) => {
    const res = await fetch(BASE + tplPath, { headers: { Authorization: `Bearer ${token}` } })
    if (!res.ok) return { code: res.status, message: '模板下载失败' }
    const buf = await res.arrayBuffer()
    const fd = new FormData()
    fd.append('file', new Blob([buf]), filename)
    const r = await fetch(BASE + importPath, { method: 'POST', headers: { Authorization: `Bearer ${token}` }, body: fd })
    return r.json()
  }
  // 楼宇结构导入已在 [4.1] 用新模板覆盖，这里只验证设备导入
  const deviceImport = await uploadTemplate(
    '/api/v1/property/devices/import-template',
    `/api/v1/property/devices/import?communityId=${communityId}`,
    '设备导入模板.xlsx'
  )
  check('设备模板可下载并导入', deviceImport.code === 0 && deviceImport.data?.success >= 1, deviceImport.message || String(deviceImport.data?.success))

  const housesAfter = await api('GET', `/api/v1/property/structures/houses?communityId=${communityId}&page=1&pageSize=200`, undefined, token)
  check('房屋总数已包含导入数据', housesAfter.code === 0 && housesAfter.data.total >= 6, String(housesAfter.data?.total))

  // ---------- 12. 概览 ----------
  console.log('\n[12] 物业概览')
  const dash = await api('GET', `/api/v1/property/dashboard?communityId=${communityId}`, undefined, token)
  check('概览返回小区指标', dash.code === 0 && Number(dash.data.community.houseTotal) >= 6)
  check('概览包含收费趋势', dash.code === 0 && Array.isArray(dash.data.feeTrend))

  // ---------- 13. 员工与组织 ----------
  console.log('\n[13] 员工与小区授权')
  // 平台管理员为该公司创建"只能管这一个小区"的客服角色（模拟公司管理员日常操作）
  const csRole = await api(
    'POST',
    '/api/v1/system/roles',
    {
      tenantId,
      name: '客服' + suffix,
      code: 'cs' + suffix,
      dataScope: 4,
      status: 1,
      communityScope: 2,
      communityIds: [communityId]
    },
    token
  )
  check('平台为公司创建限定小区的角色', csRole.code === 0, csRole.message)
  const csGrant = await api(
    'PUT',
    `/api/v1/system/roles/${csRole.data?.id}/grant`,
    {
      menuIds: ['3200', '3210', '3030', '3031', '3032', '3034', '3035'],
      communityScope: 2,
      communityIds: [communityId]
    },
    token
  )
  check('角色授权（受限菜单 + 指定小区）', csGrant.code === 0, csGrant.message)

  const emp = await api(
    'POST',
    '/api/v1/property/employees',
    {
      name: '客服小王' + suffix,
      phone: '13400000005',
      deptId: 1,
      communityId,
      position: '客服管家',
      empType: 1,
      username: `kf${suffix}`,
      password: 'Abc123456',
      roleIds: [csRole.data?.id],
      communityIds: [communityId]
    },
    token
  )
  check('新增员工（含登录账号）', emp.code === 0, emp.message)
  const empId = emp.data?.id

  const deptTree = await api('GET', '/api/v1/property/employees/dept-tree', undefined, token)
  check('组织架构树可查', deptTree.code === 0 && Array.isArray(deptTree.data))

  // ---------- 13.1 物业公司管理员的权限边界 ----------
  console.log('\n[13.1] 物业公司管理员权限边界')
  if (companyAdmin.username && companyAdmin.password) {
    const caLogin = await login(companyAdmin.username, companyAdmin.password)
    check('公司管理员可登录', caLogin.code === 0, caLogin.message)
    const caToken = caLogin.data?.accessToken
    if (caToken) {
      const caRoutes = await api('GET', '/api/v1/auth/routes', undefined, caToken)
      const titles = []
      const walk = (nodes) => {
        ;(nodes || []).forEach((n) => {
          titles.push(n.meta?.title || '')
          walk(n.children)
        })
      }
      walk(caRoutes.data)
      const has = (t) => titles.includes(t)
      check('公司管理员拥有用户/角色/部门/岗位管理', ['用户管理', '角色管理', '部门管理', '岗位管理'].every(has), titles.join('/'))
      check('公司管理员看不到租户/小区管理', !has('物业公司与小区') && !has('租户管理'), titles.join('/'))
      check('公司管理员看不到系统设置/系统监控', !has('系统设置') && !has('系统监控'), titles.join('/'))

      const tList = await api('GET', '/api/v1/system/tenants', undefined, caToken)
      check('公司管理员访问租户管理被拒', tList.code !== 0, String(tList.code))
      const cCreate = await api('POST', '/api/v1/property/communities', { code: 'X' + suffix, name: '越权小区' }, caToken)
      check('公司管理员不能新增小区', cCreate.code !== 0, String(cCreate.code))

      // 可在本公司内创建角色（限定小区）与用户
      const caRole = await api(
        'POST',
        '/api/v1/system/roles',
        { name: '收费员' + suffix, code: 'cash' + suffix, dataScope: 4, status: 1, communityScope: 2, communityIds: [communityId] },
        caToken
      )
      check('公司管理员可创建限定小区的角色', caRole.code === 0, caRole.message)
      const overGrant = await api(
        'PUT',
        `/api/v1/system/roles/${caRole.data?.id}/grant`,
        { menuIds: ['1080', '1200'], communityScope: 2, communityIds: [communityId] },
        caToken
      )
      check('不能授予超出自身权限的菜单', overGrant.code !== 0 && /权限/.test(overGrant.message), overGrant.message)
      const ownMenus = await api('GET', '/api/v1/system/roles/assignable-menus', undefined, caToken)
      const flatMenus = []
      const walkMenu = (nodes) => {
        ;(nodes || []).forEach((n) => {
          flatMenus.push(n)
          walkMenu(n.children)
        })
      }
      walkMenu(ownMenus.data)
      check('可授出菜单不含平台级菜单', !flatMenus.some((m) => ['1080', '1200', '1300', '1030'].includes(m.id)), String(flatMenus.length))
      const okGrant = await api(
        'PUT',
        `/api/v1/system/roles/${caRole.data?.id}/grant`,
        { menuIds: flatMenus.slice(0, 4).map((m) => m.id), communityScope: 2, communityIds: [communityId] },
        caToken
      )
      check('可在自身权限内完成授权', okGrant.code === 0, okGrant.message)
      const caUser = await api(
        'POST',
        '/api/v1/system/users',
        { username: 'cash' + suffix, nickname: '收费员' + suffix, password: 'Abc123456', roleIds: [caRole.data?.id], status: 1 },
        caToken
      )
      check('公司管理员可创建用户并绑定角色', caUser.code === 0, caUser.message)
      const alienRole = await api(
        'POST',
        '/api/v1/system/users',
        { username: 'bad' + suffix, nickname: '越权', password: 'Abc123456', roleIds: ['1'], status: 1 },
        caToken
      )
      check('不能把别家公司的角色分配给用户', alienRole.code !== 0, alienRole.message)
    }
  }

  // ---------- 13.2 住户多房屋关系与人脸唯一 ----------
  console.log('\n[13.2] 住户多房屋关系与人脸唯一')
  const multiHouseRes = await api('GET', `/api/v1/property/structures/houses?communityId=${communityId}&page=1&pageSize=10`, undefined, token)
  const twoHouses = (multiHouseRes.data?.list || []).slice(0, 2)
  if (twoHouses.length >= 2) {
    const resident = await api('POST', '/api/v1/property/residents', {
      communityId,
      idType: 1,
      idNo: 'MULTI' + suffix,
      name: '多房住户' + suffix,
      phone: '13900000000'
    }, token)
    const residentId = resident.data?.residentId
    check('住户登记成功（可无房屋先建档）', resident.code === 0 && !!residentId, resident.message)
    const b1 = await api('POST', '/api/v1/property/residents/bind', { residentId, communityId, houseId: twoHouses[0].id, relRole: 1, startDate: '2024-01-01' }, token)
    const b2 = await api('POST', '/api/v1/property/residents/bind', { residentId, communityId, houseId: twoHouses[1].id, relRole: 3, startDate: '2024-01-01', endDate: '2026-12-31' }, token)
    check('同一住户可绑定多套房屋', b1.code === 0 && b2.code === 0, b1.message + '/' + b2.message)
    const rels = await api('GET', `/api/v1/property/residents/${residentId}`, undefined, token)
    check('住户房屋关系返回 2 条', rels.code === 0 && (rels.data?.houses || []).filter((r) => r.status === 1).length === 2, String((rels.data?.houses || []).length))
    const relToRemove = (rels.data?.houses || []).find((r) => r.houseId === twoHouses[0].id && r.status === 1)
    if (relToRemove) {
      const del = await api('PUT', `/api/v1/property/residents/relations/${relToRemove.id}/unbind`, undefined, token)
      check('可删除其中一条房屋关系', del.code === 0, del.message)
      const rels2 = await api('GET', `/api/v1/property/residents/${residentId}`, undefined, token)
      check('删除后仅剩 1 条有效关系', (rels2.data?.houses || []).filter((r) => r.status === 1).length === 1, String((rels2.data?.houses || []).filter((r) => r.status === 1).length))
    }
    const f1 = await api('POST', `/api/v1/property/residents/${residentId}/faces`, { communityId, fileId: '1', fileUrl: '/static/test1.jpg', isPrimary: true }, token)
    const f2 = await api('POST', `/api/v1/property/residents/${residentId}/faces`, { communityId, fileId: '2', fileUrl: '/static/test2.jpg', isPrimary: true }, token)
    const detailF = await api('GET', `/api/v1/property/residents/${residentId}`, undefined, token)
    check('住户人脸唯一（重复上传覆盖）', f1.code === 0 && f2.code === 0 && (detailF.data?.faces || []).length === 1, String((detailF.data?.faces || []).length))
  }

  // ---------- 14. 数据隔离（跨租户越权） ----------
  console.log('\n[14] 小区级数据隔离')
  const empLogin = await login(`kf${suffix}`, 'Abc123456')
  check('物业员工可登录', empLogin.code === 0, empLogin.message)
  const empToken = empLogin.data?.accessToken
  if (empToken) {
    const empCommunities = await api('GET', '/api/v1/property/communities/options', undefined, empToken)
    check('员工只能看到被授权小区', empCommunities.code === 0 && empCommunities.data.length === 1, String(empCommunities.data?.length))

    // 员工访问其它小区（构造一个不存在的 ID）应被拒绝
    const denied = await api('GET', '/api/v1/property/communities/999999999', undefined, empToken)
    check('访问未授权小区返回拒绝/不存在', denied.code !== 0, String(denied.code))

    const deniedHouse = await api('GET', `/api/v1/property/nodes/tree?communityId=999999999`, undefined, empToken)
    check('未授权小区的结构树被拒绝', deniedHouse.code !== 0, String(deniedHouse.code))

    // 员工不能跨小区查询到别人的住户
    const empResidents = await api('GET', '/api/v1/property/residents', undefined, empToken)
    check('员工住户列表仅限授权小区', empResidents.code === 0)
  }

  // 角色小区范围调整后立即生效（角色是新的数据来源）
  if (empId && csRole.data?.id) {
    const otherComm = await api('POST', '/api/v1/property/communities', { tenantId, code: 'O' + suffix, name: '另一个小区' + suffix }, token)
    check('为同一物业公司创建第二个小区', otherComm.code === 0, otherComm.message)
    const regroup = await api(
      'PUT',
      `/api/v1/system/roles/${csRole.data.id}/grant`,
      { menuIds: ['3200', '3210', '3030', '3031', '3032', '3034', '3035'], communityScope: 2, communityIds: [otherComm.data?.id] },
      token
    )
    check('调整角色可管理小区', regroup.code === 0, regroup.message)
    const dropDirect = await api('PUT', `/api/v1/property/employees/${empId}/communities`, { communityIds: [], defaultId: 0 }, token)
    check('清空员工直接授权小区（改由角色决定）', dropDirect.code === 0, dropDirect.message)
    if (empToken) {
      const afterRevoke = await api('GET', '/api/v1/property/residents', undefined, empToken)
      check('角色小区范围变更后立即生效（看不到原小区数据）', afterRevoke.code === 0 && (afterRevoke.data?.total ?? 0) === 0, String(afterRevoke.data?.total))
    }
  }

  // ---------- 14.1 组织与权限：按公司隔离 + 平台切换 ----------
  console.log('\n[14.1] 组织与权限（按公司隔离）')
  const orgRoles = await api('GET', '/api/v1/system/roles?page=1&pageSize=100&tenantId=1', undefined, token)
  const builtinRoles = (orgRoles.data?.list || []).filter((r) => r.isBuiltin)
  const builtinCodes = builtinRoles.map((r) => r.code).sort()
  check(
    '默认内置角色只有超级管理员与物业管理员',
    orgRoles.code === 0 && builtinRoles.length === 2 && builtinCodes.join(',') === 'pm_company_admin,super_admin',
    builtinCodes.join(',')
  )

  // 在目标物业公司下创建部门/岗位/角色/用户（平台管理员可切换）
  const tDept = await api('POST', '/api/v1/system/depts', { tenantId, parentId: '0', name: '公司部门' + suffix, sort: 1, status: 1 }, token)
  check('平台可为指定公司建部门', tDept.code === 0, tDept.message)
  const tPost = await api('POST', '/api/v1/system/posts', { tenantId, code: 'p' + suffix, name: '公司岗位' + suffix, sort: 1, status: 1 }, token)
  check('平台可为指定公司建岗位', tPost.code === 0, tPost.message)
  const tRole = await api('POST', '/api/v1/system/roles', { tenantId, name: '公司角色' + suffix, code: 'cr' + suffix, dataScope: 4, status: 1, communityScope: 1 }, token)
  check('平台可为指定公司建角色', tRole.code === 0, tRole.message)
  const tUser = await api('POST', '/api/v1/system/users', { tenantId, username: 'tu' + suffix, nickname: '公司用户', password: 'Abc123456', roleIds: [tRole.data?.id], status: 1 }, token)
  check('平台可为指定公司建用户', tUser.code === 0, tUser.message)

  const companyUsers = await api('GET', `/api/v1/system/users?page=1&pageSize=50&tenantId=${tenantId}`, undefined, token)
  check('按公司查询用户命中刚建的用户', companyUsers.code === 0 && (companyUsers.data?.list || []).some((u) => u.username === 'tu' + suffix), String(companyUsers.data?.total))
  const defaultUsers = await api('GET', '/api/v1/system/users?page=1&pageSize=50&tenantId=1', undefined, token)
  check('默认租户用户列表不含该公司用户', defaultUsers.code === 0 && !(defaultUsers.data?.list || []).some((u) => u.username === 'tu' + suffix), String(defaultUsers.data?.total))
  const companyDepts = await api('GET', `/api/v1/system/depts/tree?tenantId=${tenantId}`, undefined, token)
  check('按公司查询部门树命中刚建的部门', companyDepts.code === 0 && JSON.stringify(companyDepts.data || []).includes('公司部门' + suffix), 'tree')
  const companyPosts = await api('GET', `/api/v1/system/posts?tenantId=${tenantId}`, undefined, token)
  check('按公司查询岗位命中刚建的岗位', companyPosts.code === 0 && (companyPosts.data || []).some((x) => x.name === '公司岗位' + suffix), 'posts')

  // 物业公司管理员不能越界到其它公司
  if (companyAdmin.username && companyAdmin.password) {
    const ca = await login(companyAdmin.username, companyAdmin.password)
    const caToken = ca.data?.accessToken
    if (caToken) {
      const crossUsers = await api('GET', '/api/v1/system/users?page=1&pageSize=10&tenantId=1', undefined, caToken)
      check('公司管理员不能切换到其它物业公司', crossUsers.code !== 0, crossUsers.message)
      const crossDept = await api('POST', '/api/v1/system/depts', { tenantId: 1, parentId: '0', name: '越权部门', sort: 1, status: 1 }, caToken)
      check('公司管理员不能把部门建到其它公司', crossDept.code !== 0, crossDept.message)
      const ownUsers = await api('GET', '/api/v1/system/users?page=1&pageSize=10', undefined, caToken)
      check('公司管理员能看到本公司用户且不含其它公司数据', ownUsers.code === 0 && (ownUsers.data?.list || []).some((u) => u.username === 'tu' + suffix) && !(ownUsers.data?.list || []).some((u) => u.username === 'admin'), String(ownUsers.data?.total))
    }
  }

  // ---------- 15. 租户间隔离 ----------
  console.log('\n[15] 租户间隔离')
  const secondCompany = await api(
    'POST',
    '/api/v1/property/companies',
    { tenantCode: 'pm2' + suffix, name: '第二物业' + suffix, contactPhone: '13800000009' },
    token
  )
  check('创建第二家物业公司', secondCompany.code === 0, secondCompany.message)
  if (secondCompany.code === 0) {
    const secondLogin = await login(secondCompany.data.adminUsername, secondCompany.data.adminPassword)
    check('第二家物业公司管理员可登录', secondLogin.code === 0, secondLogin.message)
    if (secondLogin.code === 0) {
      const t2 = secondLogin.data.accessToken
      const t2Communities = await api('GET', '/api/v1/property/communities', undefined, t2)
      check('第二家物业公司看不到第一家的小区', t2Communities.code === 0 && t2Communities.data.total === 0, String(t2Communities.data?.total))
      const t2House = await api('GET', `/api/v1/property/structures/houses?communityId=${communityId}`, undefined, t2)
      check('跨租户查询房屋被拒绝', t2House.code !== 0, String(t2House.code))
      const t2Emp = await api('GET', '/api/v1/property/employees', undefined, t2)
      check('第二家物业公司员工列表为空', t2Emp.code === 0 && t2Emp.data.total === 0, String(t2Emp.data?.total))
    }
  }

  // ---------- 汇总 ----------
  console.log(`\n=== 结果：通过 ${pass}，失败 ${fail} ===`)
  if (fail > 0) {
    console.log('失败项：')
    failures.forEach((f) => console.log('  - ' + f))
    process.exit(1)
  }
}

main().catch((e) => {
  console.error('E2E 运行异常：', e)
  process.exit(1)
})
