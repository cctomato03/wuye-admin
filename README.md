# Web Basic Admin · 小区物业管理系统

在开箱即用的后台脚手架（**Go + Gin + GORM** 后端，**Vue 3 + Web Admin Pro** 前端，内嵌迁移、单可执行文件部署）
基础上交付的**面向物业公司的多租户物业管理系统**：一个物业公司 = 一个租户，一个可管理多个小区，
小区级数据隔离，规模按「万级小区 / 百万级房屋」设计。

> 📘 二次开发看 [docs/开发手册.md](./docs/开发手册.md) ｜ 🖱 功能使用看 [docs/操作手册.md](./docs/操作手册.md)
> 🏢 物业业务与数据库设计看 [docs/物业系统设计.md](./docs/物业系统设计.md)

## 物业模块能力

| 模块 | 能力 |
| --- | --- |
| 小区 / 物业公司 | 平台建物业公司（自动建组织、内置角色、管理员）、建小区（唯一编码/行政区/地址）、公司-小区唯一归属 |
| 层级 / 房屋 | 层级**自定义**（小区为根，节点只需挂到上一级，可有可无区域/单元）；房屋挂在任意层级节点上，支持批量生成与 Excel 导入 |
| 住户 | 一户多人、一人多房、跨小区按证件回显（只回显基础资料）、业主/家人/租户 + 有效期、续期/还原/历史、人脸 |
| 访客 / 其他人员 | 访客预约与进出、随行人员、超时失效；保洁维修等临时人员 + 类别 + 有效期 |
| 车辆 / 单位 | 业主车/工作车/临时车、车位、进出记录；商铺与办公单位 |
| 收费 | 按面积·月/·天、按用量、固定金额、按套；住宅与商业分开计价；抄表；账单批量生成；9 种支付方式；余额与押金 |
| 设备 / 门禁 | 设备绑定到五级层级；住户/访客按房屋逐级追溯设备、物业人员全小区、其他人员手工勾选；人脸/卡/拨号下发、失败重试、到期解除 |
| 数据隔离 | 租户 + 小区双隔离，撤销授权立即生效；所有业务数据带 create/update/delete 时间，只做逻辑删除 |

## 特性

- **权限体系**：用户 / 角色 / 菜单（目录·页面·按钮·接口）/ 部门 / 岗位 / 数据权限 / 多租户
- **工作台**：4 张统计卡片 + 「我的待办 / 我的消息 / 通知公告」页签，顶栏铃铛显示未读
- **系统设置**：站点个性化（名称/logo/登录页文案/注册开关/验证码/滑块）、字典、参数、租户、文件、定时任务、代码生成
- **通知与协作**：通知公告（发布扇出站内信）、站内信、待办；业务系统可通过开放接口推送（`X-API-KEY` + 幂等键）
- **安全**：双 token 静默刷新、图形验证码、滑块校验、登录失败锁定、强口令策略、字段级操作日志
- **运维**：健康检查、Prometheus 指标、结构化日志、内嵌数据库迁移、Docker Compose

## 快速开始

```bash
# 基础设施（MySQL 3307 / Redis 6380）
docker compose -f deploy/docker-compose.yml up -d mysql redis

# 后端（:8090，首次启动自动建表 + 初始化 + 建超管）
APP__APP__ADMIN_PASSWORD=Admin@123456 go run ./cmd/server -c configs/config.yaml

# 前端（:5173）
cd web && pnpm install && pnpm dev
```

默认账号 `admin`，密码取 `APP__APP__ADMIN_PASSWORD`。

物业模块菜单按业务域分组：

| 菜单分组 | 页面 |
| --- | --- |
| 小区管理 | 层级管理（自定义层级树）/ 房屋管理 / 单位管理 / 车辆管理 |
| 人员管理 | 住户 / 访客 / 其他人员 |
| 收费管理 | 收费项目 / 账单管理 / 抄表管理 / 缴费记录 |
| 设备管理 | 设备信息 / 门禁授权 |
| 组织与权限 | 用户 / 角色 / 部门 / 岗位 / 员工管理（物业公司的组织与权限就在这里） |

> 物业公司（=租户）与小区由**平台管理员**在「系统设置 → 物业公司与小区」统一维护（两个页签）；
> 物业公司后台用户看不到该菜单，也创建不了公司/小区。
>
> 组织与权限：新建物业公司只内置一个「物业公司管理员」角色（拥有除系统监控/系统设置/租户与小区/菜单管理外的全部菜单），
> 公司管理员可在本公司内继续创建**角色、用户、部门、岗位**；角色可绑定"可管理小区"（本公司全部或指定若干），
> 子角色的菜单与小区范围都不能超过创建者自己的权限。

物业概览（房屋/住户/收费/门禁指标 + 近半年收费趋势）已并入**工作台**首页。

> 行政区划为省/市/区县/街道/社区五级（全国约 66 万条，内嵌数据集，首次启动自动导入；
> 需要刷新数据时执行 `node scripts/build-district-data.mjs` 后重启，或调用 `POST /api/v1/property/districts/sync?force=true`）。

生产部署（单二进制或 Docker）：

```bash
cd web && pnpm build && cd ..
powershell -ExecutionPolicy Bypass -File scripts/build.ps1   # 或 make build
docker compose -f deploy/docker-compose.yml up -d --build     # 前后端一体镜像 + MySQL + Redis
```

## 测试

```bash
node scripts/e2e-api.mjs   http://127.0.0.1:8090 admin  # 脚手架接口 75 条断言
node scripts/e2e-smoke.mjs http://127.0.0.1:8090 admin  # 脚手架浏览器 61 条断言
node scripts/e2e-property.mjs    http://127.0.0.1:8090 admin           # 物业接口 114 条断言（含越权/导入/角色与小区范围/房屋ID关联）
node scripts/e2e-property-ui.mjs http://127.0.0.1:8090 admin Admin@123456 # 物业浏览器 30 条断言
```

环境变量：`E2E_ADMIN_PASSWORD`（管理员密码）、`E2E_REDIS_CONTAINER`（读验证码用，默认自动探测）、`E2E_BROWSER`。

## 目录速览

```
cmd/server      入口            internal/{api,service,repository,model,router,middleware,migrate}
migrations      数据库迁移       pkg/{crypto,jwt,errs,response,captcha,id,storage}
scripts         构建 / 图标子集 / E2E          web/  前端（Vue3 + Web Admin Pro）
configs         默认配置（环境变量 APP__XXX__YYY 覆盖）      deploy/  Dockerfile + compose
```
