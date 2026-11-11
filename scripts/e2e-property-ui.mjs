/**
 * 物业管理前端冒烟测试（零依赖，CDP 驱动本机 Edge/Chrome）
 *
 *   node scripts/e2e-property-ui.mjs http://127.0.0.1:8090 admin Admin@123456
 *
 * 覆盖：真实登录表单 → 物业概览 / 小区 / 楼宇结构 / 住户 / 账单 / 门禁授权 六个页面渲染
 * 断言：页面关键文案出现、无浏览器运行时异常、无 5xx 接口
 *
 * 说明：
 *   - 图形验证码无法在自动化里识别，脚本会用接口登录后**临时关闭**登录验证码，
 *     跑完自动恢复（需要管理员账号有「参数配置」权限）；
 *   - 前端需已构建（web/dist）或使用 vite dev 代理。
 */
import { spawn, execFileSync } from 'node:child_process'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const BASE = (process.argv[2] || 'http://127.0.0.1:8090').replace(/\/$/, '')
const USERNAME = process.argv[3] || 'admin'
const PASSWORD = process.argv[4] || process.env.E2E_ADMIN_PASSWORD || ''
const CDP_PORT = 9444

const BROWSERS = [
  process.env.E2E_BROWSER,
  'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Google/Chrome/Application/chrome.exe',
  'C:/Program Files (x86)/Google/Chrome/Application/chrome.exe'
].filter(Boolean)

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

let pass = 0
let fail = 0
const failures = []
function check(name, ok, extra = '') {
  if (ok) {
    pass++
    console.log(`  ✓ ${name}`)
  } else {
    fail++
    failures.push(`${name}${extra ? ' → ' + extra : ''}`)
    console.log(`  ✗ ${name}${extra ? ' → ' + extra : ''}`)
  }
}

/** 从 Redis 读取验证码答案（容器名/前缀可用环境变量覆盖） */
function readCaptchaFromRedis(captchaId) {
  const containers = [process.env.E2E_REDIS_CONTAINER, 'wb-redis', 'web-basic-redis', 'web-basic-redis57'].filter(Boolean)
  const prefix = process.env.E2E_REDIS_PREFIX || 'web:'
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
  return ''
}

/* ---------------------------- 接口辅助（测试前置） ---------------------------- */

const jsonHeaders = { 'Content-Type': 'application/json' }

async function apiGet(path, token) {
  const res = await fetch(BASE + path, {
    headers: token ? { ...jsonHeaders, Authorization: `Bearer ${token}` } : jsonHeaders
  })
  return res.json()
}

async function apiPost(path, body) {
  const res = await fetch(BASE + path, { method: 'POST', headers: jsonHeaders, body: JSON.stringify(body || {}) })
  return res.json()
}

/** 接口登录（验证码开启时从 Redis 取答案） */
async function apiLogin() {
  let captcha = {}
  try {
    const pub = await apiGet('/api/v1/system/configs/public')
    if (String(pub?.data?.['sys.login.captcha'] ?? 'true').toLowerCase() !== 'false') {
      const cap = await apiGet('/api/v1/auth/captcha')
      const captchaId = cap?.data?.captchaId || ''
      const captchaCode = captchaId ? readCaptchaFromRedis(captchaId) : ''
      captcha = { captchaId, captchaCode }
    }
  } catch {
    /* 保底：不带验证码尝试 */
  }
  const login = await apiPost('/api/v1/auth/login', { username: USERNAME, password: PASSWORD, ...captcha })
  return login?.data?.accessToken || ''
}

/** 切换登录开关（验证码 / 滑块；测试用，结束后恢复） */
async function setLoginSwitch(token, key, value) {
  const list = await apiGet('/api/v1/system/configs', token)
  const item = (list?.data || []).find((c) => c.key === key)
  if (!item) return false
  const res = await fetch(`${BASE}/api/v1/system/configs/${item.id}`, {
    method: 'PUT',
    headers: { ...jsonHeaders, Authorization: `Bearer ${token}` },
    body: JSON.stringify({
      name: item.name,
      key: item.key,
      value,
      valueType: item.valueType,
      isPublic: item.isPublic,
      remark: item.remark
    })
  })
  const out = await res.json()
  return out.code === 0
}

/* ---------------------------- 极简 CDP 客户端 ---------------------------- */

class CDP {
  constructor(ws) {
    this.ws = ws
    this.id = 0
    this.pending = new Map()
    ws.addEventListener('message', (ev) => {
      const msg = JSON.parse(ev.data)
      if (msg.id && this.pending.has(msg.id)) {
        const { resolve, reject } = this.pending.get(msg.id)
        this.pending.delete(msg.id)
        msg.error ? reject(new Error(JSON.stringify(msg.error))) : resolve(msg.result)
      }
    })
  }
  send(method, params = {}) {
    const id = ++this.id
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject })
      this.ws.send(JSON.stringify({ id, method, params }))
    })
  }
  async eval(expression) {
    const res = await this.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true })
    return res?.result?.value
  }
}

async function launch() {
  const dir = mkdtempSync(join(tmpdir(), 'pm-ui-'))
  let proc
  for (const bin of BROWSERS) {
    try {
      proc = spawn(
        bin,
        [
          `--remote-debugging-port=${CDP_PORT}`,
          `--user-data-dir=${dir}`,
          '--headless=new',
          '--disable-gpu',
          '--no-first-run',
          '--no-default-browser-check',
          '--window-size=1440,900',
          'about:blank'
        ],
        { stdio: 'ignore' }
      )
      break
    } catch {
      /* 尝试下一个 */
    }
  }
  if (!proc) throw new Error('未找到可用浏览器，请设置 E2E_BROWSER')
  for (let i = 0; i < 40; i++) {
    await sleep(250)
    try {
      const list = await (await fetch(`http://127.0.0.1:${CDP_PORT}/json/list`)).json()
      const page = list.find((t) => t.type === 'page')
      if (page) return { proc, dir, page }
    } catch {
      /* 继续等 */
    }
  }
  throw new Error('浏览器调试端口未就绪')
}

async function main() {
  if (!PASSWORD) {
    console.error('缺少密码：node scripts/e2e-property-ui.mjs <baseUrl> <user> <password>')
    process.exit(2)
  }
  console.log(`\n=== 物业前端冒烟 @ ${BASE} ===\n`)

  // 前置：接口登录 + 临时关闭图形验证码（跑完恢复）
  const adminToken = await apiLogin()
  // 图形验证码无法识别、滑块无法自动拖动：两个开关都临时关掉，跑完恢复
  const switched = adminToken
    ? (await setLoginSwitch(adminToken, 'sys.login.captcha', 'false')) && (await setLoginSwitch(adminToken, 'sys.login.slider', 'false'))
    : false
  console.log('  · 临时关闭登录验证码/滑块：' + (switched ? '是' : '否（将无法自动登录，请手工关闭后重试）'))

  /* 前置数据：确保至少有 2 家物业公司、1 个小区（空库也能独立跑） */
async function ensureBaseData(token) {
  const auth = { Authorization: `Bearer ${token}` }
  const post = async (path, body) =>
    (await fetch(BASE + path, { method: 'POST', headers: { ...jsonHeaders, ...auth }, body: JSON.stringify(body || {}) })).json()
  const companies = await apiGet('/api/v1/property/companies?page=1&pageSize=50', token)
  let list = companies?.data?.records || companies?.data?.list || []
  const stamp = String(Date.now()).slice(-6)
  while (list.length < 2) {
    const res = await post('/api/v1/property/companies', {
      tenantCode: `uit${stamp}${list.length}`,
      name: `UI测试物业${stamp}${list.length}`,
      contactPhone: '13800001111'
    })
    if (res.code !== 0) break
    const again = await apiGet('/api/v1/property/companies?page=1&pageSize=50', token)
    list = again?.data?.records || again?.data?.list || []
  }
  const comms = await apiGet('/api/v1/property/communities/options', token)
  if (!(comms?.data || []).length && list.length) {
    const tenantId = list[0].tenantId || list[0].id
    await post('/api/v1/property/communities', {
      tenantId,
      code: `UIC${stamp}`,
      name: `UI测试小区${stamp}`,
      communityType: 1,
      address: '自动化测试地址'
    })
  }
  // 保证有层级与房屋，树/卡片断言才有数据
  const comms2 = await apiGet('/api/v1/property/communities/options', token)
  const communityId = (comms2?.data || [])[0]?.id
  if (!communityId) return
  const tree = await apiGet(`/api/v1/property/nodes/tree?communityId=${communityId}`, token)
  let nodes = tree?.data || []
  const ensureNode = async (parentId, name, nodeType) => {
    const hit = nodes.find((n) => n.name === name && String(n.parentId) === String(parentId))
    if (hit) return hit
    const res = await post('/api/v1/property/nodes', { communityId, parentId, name, nodeType })
    if (res.code !== 0) return null
    const t = await apiGet(`/api/v1/property/nodes/tree?communityId=${communityId}`, token)
    nodes = t?.data || []
    return nodes.find((n) => n.id === String(res.data?.id)) || null
  }
  const building = await ensureNode(0, '1栋', 2)
  const unit = building ? await ensureNode(building.id, '1单元', 3) : null
  const floor = unit ? await ensureNode(unit.id, '1层', 4) : null
  const target = floor || unit || building
  if (target) {
    const houses = await apiGet(`/api/v1/property/structures/houses?communityId=${communityId}&nodeId=${target.id}&page=1&pageSize=10`, token)
    if (!(houses?.data?.list || []).length) {
      await post('/api/v1/property/structures/houses', {
        communityId,
        nodeId: target.id,
        code: `UIH${stamp}`,
        houseNo: '101',
        usageType: 1,
        areaBuild: 89.5,
        areaUse: 75,
        houseStatus: 2
      })
    }
  }
}

await ensureBaseData(adminToken)

const { proc, dir, page } = await launch()
  const ws = new WebSocket(page.webSocketDebuggerUrl)
  await new Promise((res, rej) => {
    ws.addEventListener('open', res, { once: true })
    ws.addEventListener('error', rej, { once: true })
  })
  const cdp = new CDP(ws)
  const consoleErrors = []
  const badRequests = []
  await cdp.send('Runtime.enable')
  await cdp.send('Network.enable')
  await cdp.send('Page.enable')
  ws.addEventListener('message', (ev) => {
    const msg = JSON.parse(ev.data)
    if (msg.method === 'Runtime.exceptionThrown') {
      consoleErrors.push(msg.params?.exceptionDetails?.exception?.description || 'exception')
    }
    if (msg.method === 'Runtime.consoleAPICalled' && msg.params?.type === 'error') {
      consoleErrors.push((msg.params.args || []).map((a) => a.value || a.description).join(' '))
    }
    if (msg.method === 'Network.responseReceived') {
      const { response } = msg.params
      if (response.status >= 500 && response.url.includes('/api/')) badRequests.push(`${response.status} ${response.url}`)
    }
  })

  try {
    // 1. 登录页
    await cdp.send('Page.navigate', { url: `${BASE}/#/auth/login` })
    await sleep(2500)
    check('登录页渲染', (await cdp.eval(`!!document.querySelector('input[type="password"]')`)) === true)

    // 2. 真实表单登录
    await cdp.eval(`
      (() => {
        const setVal = (el, val) => {
          el.value = val
          el.dispatchEvent(new Event('input', { bubbles: true }))
          el.dispatchEvent(new Event('change', { bubbles: true }))
        }
        const pwd = document.querySelector('input[type="password"]')
        const form = pwd?.closest('form') || document
        const inputs = [...form.querySelectorAll('input')].filter((i) => i.type === 'text')
        setVal(inputs[0], ${JSON.stringify(USERNAME)})
        setVal(pwd, ${JSON.stringify(PASSWORD)})
        return true
      })()
    `)
    await sleep(300)
    await cdp.eval(`
      (() => {
        const b = [...document.querySelectorAll('button')].find((x) => /登\\s*录/.test(x.innerText))
        if (b) b.click()
        return !!b
      })()
    `)
    let loggedIn = false
    for (let i = 0; i < 40; i++) {
      await sleep(500)
      const hash = (await cdp.eval('location.hash')) || ''
      if (hash.includes('/dashboard')) {
        loggedIn = true
        break
      }
    }
    check('真实账号密码登录成功', loggedIn, await cdp.eval('location.href'))

    // 3. 物业各页面渲染
    // 菜单已按功能分组：小区管理 / 人员管理 / 物业公司 / 收费管理 / 设备管理
    const pages = [
      { path: '/dashboard', expect: /工作台|物业概览|房屋总数/ },
      { path: '/setting/tenant', expect: /物业公司|小区/ },
      { path: '/property/site/space', expect: /基础信息|楼栋|房屋/ },
      { path: '/property/site/merchant', expect: /单位|商铺/ },
      { path: '/property/site/vehicle', expect: /车牌|车位/ },
      { path: '/property/people/resident', expect: /登记住户|房屋关系/ },
      { path: '/property/people/visitor', expect: /访客/ },
      { path: '/property/people/other', expect: /其他人员|人员类别|登记人员/ },

      { path: '/system/employee', expect: /员工|组织架构/ },
      { path: '/system/post', expect: /岗位/ },
      { path: '/property/fee/item', expect: /收费项目|新增收费项目/ },
      { path: '/property/fee/bill', expect: /账单|生成账单/ },
      { path: '/property/fee/meter', expect: /水表|抄表/ },
      { path: '/property/fee/payment', expect: /缴费|账户余额/ },
      { path: '/property/devices/info', expect: /设备|新增设备/ },
      { path: '/property/devices/access', expect: /门禁授权|授权总数/ }
    ]
    for (const item of pages) {
      await cdp.send('Page.navigate', { url: `${BASE}/#${item.path}` })
      await sleep(2600)
      const text = (await cdp.eval('document.body.innerText')) || ''
      check(`页面渲染：${item.path}`, item.expect.test(text), text.slice(0, 60).replace(/\n/g, ' '))
    }

    // 单位管理：所在房屋级联可以选到房屋（不只到层级）
    await cdp.send('Page.navigate', { url: `${BASE}/#/property/site/merchant` })
    await sleep(2600)
    await cdp.eval("(() => { const b=[...document.querySelectorAll('button')].find(x=>/登记单位|新增/.test(x.innerText)); b&&b.click() })()")
    await sleep(900)
    const cascPick = await cdp.eval(`JSON.stringify((() => {
      const d = document.querySelector('.el-dialog')
      const cas = d && d.querySelector('.el-cascader')
      if (!cas) return { ok: false, reason: 'no-cascader' }
      cas.querySelector('.el-input__wrapper')?.click()
      return { ok: true }
    })())`)
    await sleep(700)
    let leafLabel = ''
    for (let lv = 0; lv < 6 && !leafLabel; lv++) {
      const res = await cdp.eval(`(() => {
        const panels = [...document.querySelectorAll('.el-cascader-panel .el-cascader-menu')]
        const last = panels[panels.length - 1]
        if (!last) return ''
        const nodes = [...last.querySelectorAll('.el-cascader-node')]
        const leaf = nodes.find((n) => !n.querySelector('.el-icon-arrow-right'))
        if (leaf) {
          const t = (leaf.innerText || '').trim()
          leaf.click()
          return t
        }
        nodes[0] && nodes[0].click()
        return ''
      })()`)
      if (res) leafLabel = String(res)
      await sleep(700)
    }
    const pickedHouse = (await cdp.eval("(() => { const c=document.querySelector('.el-dialog .el-cascader'); if(!c) return ''; const inp=c.querySelector('input.el-input__inner') || c.querySelector('input'); return (inp && inp.value) || c.innerText.trim(); })()")) || ''
    const cascInfo = JSON.parse(cascPick || '{}')
    check('单位管理可按层级选到具体房屋', cascInfo.ok && leafLabel.length > 0, leafLabel || pickedHouse || JSON.stringify(cascInfo))
    await cdp.eval("(() => { const b=[...document.querySelectorAll('.el-dialog button')].find(x=>x.innerText.trim()==='取消'); b&&b.click() })()")
    await sleep(400)

    // 车辆管理已移除车位管理
    await cdp.send('Page.navigate', { url: `${BASE}/#/property/site/vehicle` })
    await sleep(2400)
    const vehicleText = (await cdp.eval('document.body.innerText')) || ''
    check('车辆管理不再有车位管理入口', !/新增车位|车位管理/.test(vehicleText), vehicleText.slice(0, 60).replace(/\n/g, '/'))

    // 住户登记弹窗：房屋使用层级级联，身份/关系字段宽度正常
    await cdp.send('Page.navigate', { url: `${BASE}/#/property/people/resident` })
    await sleep(2600)
    await cdp.eval("(() => { const b=[...document.querySelectorAll('button')].find(x=>x.innerText.trim()==='登记住户'); b&&b.click() })()")
    await sleep(900)
    const residentDlg = await cdp.eval(`JSON.stringify((() => {
      const d = document.querySelector('.el-dialog')
      if (!d) return { has: false }
      const cas = d.querySelector('.el-cascader')
      const selects = [...d.querySelectorAll('.el-select')].map((s) => Math.round(s.getBoundingClientRect().width))
      return { has: true, cascader: !!cas, widths: selects }
    })())`)
    const rd = JSON.parse(residentDlg || '{}')
    check('住户登记房屋选择为层级级联', rd.has && rd.cascader, JSON.stringify(rd))
    check('身份/关系选择框宽度足够', Array.isArray(rd.widths) && rd.widths.filter((w) => w >= 150).length >= 2, JSON.stringify(rd.widths))
    await cdp.eval("(() => { const b=[...document.querySelectorAll('.el-dialog button')].find(x=>x.innerText.includes('添加房屋')); b&&b.click() })()")
    await sleep(500)
    const cascCount = await cdp.eval("document.querySelectorAll('.el-dialog .el-cascader').length")
    check('住户可添加多套房屋关系', Number(cascCount) >= 2, String(cascCount))
    const faceUi = await cdp.eval("(() => { const d=document.querySelector('.el-dialog'); if(!d) return ''; const txt=d.innerText; return JSON.stringify({ title: /人脸照片/.test(txt), btn: !!([...d.querySelectorAll('button')].find(b=>/上传人脸|重新上传/.test(b.innerText))) }) })()")
    const fu = JSON.parse(faceUi || '{}')
    check('登记住户可上传人脸照片', !!fu.title && !!fu.btn, faceUi)
    await cdp.eval("(() => { const b=[...document.querySelectorAll('.el-dialog button')].find(x=>x.innerText.trim()==='取消'); b&&b.click() })()")
    await sleep(400)

    // 组织与权限：平台管理员可见"物业公司"切换
    await cdp.send('Page.navigate', { url: `${BASE}/#/system/user` })
    await sleep(2800)
    const orgText = (await cdp.eval('document.body.innerText')) || ''
    check('用户管理页显示物业公司切换', /物业公司/.test(orgText), orgText.slice(0, 60).replace(/\n/g, '/'))

    // 基础信息管理：顶部小区卡片 + 左树 + 右信息卡片
    await cdp.send('Page.navigate', { url: `${BASE}/#/property/site/space` })
    await sleep(3200)
    const spaceText = (await cdp.eval('document.body.innerText')) || ''
    check('顶部小区卡片展示小区信息', /楼栋/.test(spaceText) && /房屋/.test(spaceText) && /住户/.test(spaceText), spaceText.slice(0, 60).replace(/\n/g, '/'))

    // 展开根节点 → 出现第一层
    await cdp.eval(`(() => { const i = document.querySelector('.el-tree-node__expand-icon'); i && i.click() })()`)
    await sleep(1200)
    const treeRows = await cdp.eval("document.querySelectorAll('.el-tree-node').length")
    check('左侧层级树可展开', Number(treeRows) > 1, String(treeRows))

    // 点击第一个层级 → 右侧信息卡片（名称/范围/房屋总数/住户/地址/设备）
    await cdp.eval(`(() => {
      const children = [...document.querySelectorAll('.el-tree-node__children > .el-tree-node > .el-tree-node__content')]
      const target = children.find((n) => n.innerText && !/房屋/.test(n.innerText)) || children[0]
      target && target.click()
      return !!target
    })()`)
    await sleep(1600)
    const cardText = (await cdp.eval("document.querySelector('.art-table-card')?.innerText || document.body.innerText")) || ''
    check('层级卡片显示名称/范围/房屋总数/住户人数/地址', /名称：/.test(cardText) && /范围/.test(cardText) && /房屋总数/.test(cardText) && /住户人数/.test(cardText) && /地址/.test(cardText), cardText.slice(0, 100).replace(/\n/g, '/'))
    check('层级卡片按类型命名（不再写死"层级"）', /(楼栋|区域|单元|楼层)(名称|范围|地址)：?/.test(cardText), cardText.match(/(楼栋|区域|单元|楼层)(名称|范围|地址)/g)?.join(','))
    check('层级卡片显示关联设备', /关联设备/.test(cardText))

    // 展开层级找到房屋 → 房屋卡片含住户/车辆信息
    // 逐层展开，直到出现房屋行
    for (let i = 0; i < 5; i++) {
      const hasHouse = await cdp.eval("!![...document.querySelectorAll('.el-tree-node__content')].find((n) => /房屋/.test(n.innerText))")
      if (hasHouse) break
      await cdp.eval(`(() => {
        const icons = [...document.querySelectorAll('.el-tree-node__expand-icon')].filter((x) => !x.classList.contains('expanded'))
        icons[icons.length - 1] && icons[icons.length - 1].click()
      })()`)
      await sleep(1200)
    }
    const houseClicked = await cdp.eval(`(() => {
      const rows = [...document.querySelectorAll('.el-tree-node__content')]
      const house = rows.find((n) => /房屋/.test(n.innerText))
      house && house.click()
      return !!house
    })()`)
    await sleep(1500)
    const houseCardText = (await cdp.eval('document.body.innerText')) || ''
    check('房屋卡片显示住户与车辆信息', houseClicked && /房屋名称：/.test(houseCardText) && /住户信息/.test(houseCardText) && /车辆信息/.test(houseCardText), houseCardText.slice(0, 80).replace(/\n/g, '/'))

    // 顶部按钮只保留导入/下载模板
    const toolbarText = (await cdp.eval("[...document.querySelectorAll('.el-card')][0] ? [...document.querySelectorAll('.el-card')][0].querySelectorAll('button') && [...[...document.querySelectorAll('.el-card')][0].querySelectorAll('button')].map((b) => b.innerText.trim()).join('|') : ''")) || ''
    check('顶部仅保留导入与下载模板按钮', /导入 Excel/.test(toolbarText) && /下载模板/.test(toolbarText) && !/导出|房屋类型|批量生成/.test(toolbarText), toolbarText.slice(0, 120))

    // 清空层级与房屋：按钮存在且二次确认
    check('存在清空层级与房屋按钮', /清空层级与房屋/.test(toolbarText), toolbarText.slice(0, 120))
    await cdp.eval("(() => { const b=[...document.querySelectorAll('button')].find(x=>x.innerText.includes('清空层级与房屋')); b&&b.click() })()")
    await sleep(700)
    const confirmText = (await cdp.eval("document.querySelector('.el-message-box')?.innerText || ''")) || ''
    check('清空前有二次确认提示', /不可恢复/.test(confirmText), confirmText.slice(0, 60).replace(/\n/g, '/'))
    await cdp.eval("(() => { const b=[...document.querySelectorAll('.el-message-box button')].find(x=>x.innerText.trim()==='取消'); b&&b.click() })()")
    await sleep(400)

    // 新建层级：上级层级为可折叠树形下拉（默认折叠，可选中任一节点）
    await cdp.eval("(() => { const b=[...document.querySelectorAll('button')].find(x=>x.innerText.trim()==='新建'); b&&b.click() })()")
    await sleep(1200)
    await cdp.eval("(() => { const d=document.querySelector('.el-dialog'); const w=d&&d.querySelector('.el-select .el-select__wrapper, .el-select .el-input__wrapper'); w&&w.click() })()")
    await sleep(700)
    const treeInfo = await cdp.eval("JSON.stringify({ tree: !!document.querySelector('.el-select-dropdown .el-tree'), nodes: document.querySelectorAll('.el-select-dropdown .el-tree-node').length })")
    const ti = JSON.parse(treeInfo || '{}')
    check('新建层级上级为可折叠树形下拉', !!ti.tree, treeInfo)
    check('上级层级默认折叠（只显示小区根节点）', Number(ti.nodes) === 1, String(ti.nodes))
    await cdp.eval("(() => { const i=document.querySelector('.el-select-dropdown .el-tree-node__expand-icon'); i&&i.click() })()")
    await sleep(600)
    const afterExpand = await cdp.eval("document.querySelectorAll('.el-select-dropdown .el-tree-node').length")
    check('展开后显示下级层级', Number(afterExpand) > 1, String(afterExpand))
    await cdp.eval("(() => { const b=[...document.querySelectorAll('.el-dialog button')].find(x=>x.innerText.trim()==='取消'); b&&b.click() })()")
    await sleep(500)

    // 房屋弹窗：所属层级同样为可折叠树形下拉
    await cdp.eval(`(() => {
      const rows = [...document.querySelectorAll('.el-tree-node__children > .el-tree-node > .el-tree-node__content')]
      rows[0] && rows[0].click()
    })()`)
    await sleep(900)
    await cdp.eval("(() => { const b=[...document.querySelectorAll('button')].find(x=>x.innerText.trim()==='新增房屋'); b&&b.click() })()")
    await sleep(900)
    await cdp.eval("(() => { const d=document.querySelector('.el-dialog'); const w=d&&d.querySelector('.el-select .el-select__wrapper, .el-select .el-input__wrapper'); w&&w.click() })()")
    await sleep(600)
    const houseTree = await cdp.eval("!!document.querySelector('.el-select-dropdown .el-tree')")
    check('新建房屋所属层级为可折叠树形下拉', !!houseTree)
    await cdp.eval("(() => { const b=[...document.querySelectorAll('.el-dialog button')].find(x=>x.innerText.trim()==='取消'); b&&b.click() })()")
    await sleep(400)

    // 新建层级弹窗：上级 + 名称 + 类型（类型来自字典），且无面积/年份/编码
    await cdp.eval("(() => { const b=[...document.querySelectorAll('button')].find(x=>x.innerText.trim()==='新建'); b&&b.click() })()")
    await sleep(900)
    const nodeDialogText = (await cdp.eval("document.querySelector('.el-dialog')?.innerText || ''")) || ''
    check('新建层级只含上级/名称/类型', /上级层级/.test(nodeDialogText) && /层级名称/.test(nodeDialogText) && /层级类型/.test(nodeDialogText), nodeDialogText.slice(0, 80).replace(/\n/g, '/'))
    check('新建层级不含面积/年份/结构/交付/编码', !/建筑面积|建成年份|建筑结构|交付日期|编码/.test(nodeDialogText))
    await cdp.eval("(() => { const b=[...document.querySelectorAll('.el-dialog button')].find(x=>x.innerText.trim()==='取消'); b&&b.click() })()")
    await sleep(500)

    // 当前小区选择持久化：在 A 页面切换后，进入 B 页面应沿用同一个小区
    const readSelected = async () =>
      (await cdp.eval(
        "document.querySelector('.el-select .el-select__selected-item, .el-select .el-select__placeholder')?.innerText || ''"
      )) || ''
    await cdp.send('Page.navigate', { url: `${BASE}/#/property/site/hierarchy` })
    await sleep(2600)
    const storedFirst = (await cdp.eval("localStorage.getItem('property.currentCommunityId') || ''")) || ''
    await cdp.send('Page.navigate', { url: `${BASE}/#/property/people/resident` })
    await sleep(2600)
    const storedSecond = (await cdp.eval("localStorage.getItem('property.currentCommunityId') || ''")) || ''
    check('当前小区在本地持久化且跨页面沿用', !!storedFirst && storedFirst === storedSecond, storedFirst + ' / ' + storedSecond)
    void readSelected

    // 侧边菜单应按功能分组（不再是「物业管理」一个目录塞满）
    const menuText =
      (await cdp.eval(
        "[...document.querySelectorAll('aside, .art-sidebar, .layout-sidebar')].map((e) => e.innerText || '').join(' ')"
      )) || ''
    const groups = ['小区管理', '人员管理', '收费管理', '设备管理']
    const missing = groups.filter((g) => !menuText.includes(g))
    check('侧边菜单包含 4 个物业功能分组', missing.length === 0, missing.join(','))
    // 菜单合并用接口数据校验（DOM 文本太长且受折叠影响）
    const routeTitles = await cdp.eval(`
      (async () => {
        const tk = JSON.parse(localStorage.getItem('sys-v1-user') || '{}').accessToken || ''
        const res = await fetch('/api/v1/auth/routes', { headers: { Authorization: 'Bearer ' + tk } })
        const json = await res.json()
        const out = []
        const walk = (n) => (n || []).forEach((x) => { out.push(x.meta?.title || ''); walk(x.children) })
        walk(json.data)
        return JSON.stringify(out)
      })()
    `)
    const titlesArr = JSON.parse(routeTitles || '[]')
    check('员工管理已并入组织与权限', titlesArr.includes('组织与权限') && titlesArr.includes('员工管理'), titlesArr.slice(0, 12).join('/'))
    check('不再有独立物业公司分组', !titlesArr.includes('物业公司'), titlesArr.filter((t) => t === '物业公司').length + '')
    check('物业概览已并入工作台（无独立菜单项）', !/物业概览/.test(menuText), menuText.slice(0, 60).replace(/\n/g, ' '))

    // 租户管理页：公司 / 小区两个页签 + 新增小区弹窗（物业公司下拉、五级行政区划）
    await cdp.send('Page.navigate', { url: `${BASE}/#/setting/tenant` })
    await sleep(2600)
    const tabsText = (await cdp.eval('document.body.innerText')) || ''
    check('租户管理包含「物业公司 / 小区」两个页签', /物业公司/.test(tabsText) && /小区/.test(tabsText))
    await cdp.eval(`(() => {
      const tabs = [...document.querySelectorAll('.el-tabs__item')]
      const t = tabs.find((x) => x.innerText.trim() === '小区')
      t && t.click()
      return !!t
    })()`)
    await sleep(1200)
    await sleep(2600)
    await cdp.eval(`(() => { const b = [...document.querySelectorAll('button')].find((x) => x.innerText.includes('新增小区')); b && b.click(); return true })()`)
    await sleep(1200)
    const tenantOptions = await cdp.eval(`(async () => {
      const dialog = document.querySelector('.el-dialog')
      if (!dialog) return -1
      const sel = dialog.querySelector('.el-select')
      if (!sel) return -2
      sel.querySelector('.el-select__wrapper')?.click()
      await new Promise((r) => setTimeout(r, 700))
      // 只统计当前弹窗打开的 popper（页面顶部还有小区选择器，不能全局数）
      const poppers = [...document.querySelectorAll('.el-popper')].filter((p) => p.style.display !== 'none' && p.innerText.trim())
      const items = poppers.length ? [...poppers[poppers.length - 1].querySelectorAll('.el-select-dropdown__item')] : []
      const texts = items.map((e) => e.innerText.trim())
      document.body.click()
      return texts.length
    })()`)
    check('新增小区可选的物业公司数量 ≥ 2', tenantOptions >= 2, String(tenantOptions))

    const districtCount = await cdp.eval(`(async () => {
      const dialog = document.querySelector('.el-dialog')
      if (!dialog) return -1
      const cas = dialog.querySelector('.el-cascader')
      if (!cas) return -2
      cas.querySelector('.el-input__wrapper')?.click()
      await new Promise((r) => setTimeout(r, 900))
      const nodes = [...document.querySelectorAll('.el-cascader-node')].filter((e) => e.offsetParent !== null)
      return nodes.length
    })()`)
    check('行政区划级联加载出省级列表（≥ 30）', districtCount >= 30, String(districtCount))

    // 选到社区：逐级点第一个节点，最终应回填"省/市/区/街道/社区"文字（不能空白）
    const picked = await cdp.eval(`
      (async () => {
        const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
        const dialog = document.querySelector('.el-dialog')
        if (!dialog) return 'no-dialog'
        const cas = dialog.querySelector('.el-cascader')
        if (!cas) return 'no-cascader'
        cas.querySelector('.el-input__wrapper')?.click()
        await sleep(600)
        for (let lv = 0; lv < 5; lv++) {
          const panels = [...document.querySelectorAll('.el-cascader-panel .el-cascader-menu')]
          const last = panels[panels.length - 1]
          if (!last) break
          const nodes = [...last.querySelectorAll('.el-cascader-node')].filter((n) => n.offsetParent !== null)
          if (!nodes.length) break
          nodes[0].click()
          await sleep(700)
        }
        const label = dialog.querySelector('.text-xs.text-g-500')?.innerText || ''
        return label
      })()
    `)
    check('行政区划选择后回填完整路径（不空白）', typeof picked === 'string' && /\//.test(picked) && picked.split('/').length >= 3, JSON.stringify(picked))
    await cdp.eval(`(() => { document.body.click(); return true })()`)

    check('无浏览器运行时异常', consoleErrors.length === 0, consoleErrors.slice(0, 2).join(' | '))
    check('无 5xx 接口', badRequests.length === 0, badRequests.slice(0, 3).join(' | '))
  } finally {
    // 恢复验证码开关
    if (switched && adminToken) {
      await setLoginSwitch(adminToken, 'sys.login.captcha', 'true')
      await setLoginSwitch(adminToken, 'sys.login.slider', 'true')
    }
    try {
      ws.close()
    } catch {
      /* ignore */
    }
    proc.kill()
    await sleep(300)
    try {
      rmSync(dir, { recursive: true, force: true })
    } catch {
      /* ignore */
    }
  }

  console.log(`\n=== 结果：通过 ${pass}，失败 ${fail} ===`)
  if (fail) {
    failures.forEach((f) => console.log('  - ' + f))
    process.exit(1)
  }
}

main().catch((e) => {
  console.error('冒烟测试异常：', e)
  process.exit(1)
})
