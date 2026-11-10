# 小区物业管理系统（多租户）

面向物业服务企业的多租户小区管理平台：以「物业公司 = 租户」隔离数据，支持万级小区规模，
覆盖小区基础信息（自定义层级 + 房屋）、住户与人员、车辆车位、收费账单、设备门禁、组织与权限。

- 后端：Go 1.26 · Gin · GORM · MySQL 8 · Redis（JWT 鉴权、RBAC 权限、逻辑删除）
- 前端：Vue 3 · TypeScript · Vite · Element Plus（动态菜单、按钮级权限指令 `v-auth`）
- 交付形态：单可执行文件 + 前端静态资源（可前后端分离部署）

---

## 一、功能总览

### 平台侧（平台超级管理员，绑定默认租户）
| 模块 | 能力 |
| --- | --- |
| 工作台 | 平台概览、物业概览、我的待办/消息 |
| 组织与权限 | 用户 / 角色 / 部门 / 岗位 / 员工（按物业公司隔离；平台角色可切换任意物业公司管理；角色可绑定「可管理小区」） |
| 系统设置 → 物业公司与小区 | 物业公司（=租户）建档、管理员账号、小区与行政区划 |
| 系统设置 | 字典、参数、文件、定时任务、站点个性化、接口文档 |
| 系统监控 | 在线用户、登录日志、操作日志、服务器信息 |
| 能力开放 | 代码生成、消息与待办推送接口 |

### 物业公司侧（公司管理员，含除平台配置外的全部业务菜单）
| 模块 | 能力 |
| --- | --- |
| 小区管理 → 基础信息管理 | 自定义层级树（区域/楼栋/单元/楼层，类型取自字典）+ 房屋；层级/房屋信息卡片；Excel 模板导入导出 |
| 小区管理 | 单位商铺、车辆与车位（一律按房屋 ID 关联） |
| 人员管理 | 住户（多小区住户档案、房屋关系、续期、人脸）、访客登记、其他人员（保姆/装修工等） |
| 收费管理 | 收费项目与标准、生成账单、抄表账单、缴费与余额划扣、缴费记录 |
| 设备管理 | 设备档案（可绑定任意层级）、门禁授权（人员-房屋-设备-卡片） |
| 组织与权限 | 用户 / 角色 / 部门 / 岗位 / 员工（子角色菜单与小区不得超过创建者） |

### 数据隔离
- **租户隔离**：所有业务数据带 `tenant_id`，写操作按调用者租户落库，跨租户读写一律拒绝。
- **小区隔离**：用户可见小区 = 角色「可管理小区」∪ 直接授权；超出范围返回 403。
- **逻辑删除**：所有表软删除（`deleted` 毫秒时间戳），列表默认过滤。
- **审计字段**：`create_by/create_time/update_by/update_time` 全表统一。

---

## 二、快速开始

### 1. 依赖
| 组件 | 版本 | 说明 |
| --- | --- | --- |
| Go | 1.26+ | 后端编译 |
| Node.js + pnpm | 20+ / 9+ | 前端构建 |
| MySQL | 8.0 | 必须 `parseTime=True&clientFoundRows=true` |
| Redis | 6+ | 登录态、验证码、权限缓存 |

仓库自带 `deploy/docker-compose.yml`，可一键起 MySQL(3307) + Redis(6380)：

```bash
docker compose -f deploy/docker-compose.yml up -d
```

### 2. 配置
`configs/config.yaml`（生产用环境变量覆盖，如 `APP__MYSQL__DSN`、`APP__JWT__SECRET`）：

```yaml
mysql:
  dsn: "root:root123@tcp(127.0.0.1:3307)/web_basic?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s&clientFoundRows=true"
  auto_migrate: true          # 首次启动自动建表 + 迁移 + 基础数据
redis:
  addr: "127.0.0.1:6380"
  prefix: "web:"
```

### 3. 建库并启动后端

```sql
CREATE DATABASE web_basic CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
```

```bash
go build -o bin/wuye-server.exe ./cmd/server
# 首次建议显式指定超管密码（留空则随机生成并打印在日志中）
APP__APP__ADMIN_PASSWORD='Admin@123456' ./bin/wuye-server.exe -c configs/config.yaml
```

首次启动自动完成：建表 → 执行 `migrations/*.up.sql` → 写入超管/菜单/字典/参数 → 异步导入五级行政区划（66 万行）。
健康检查：`GET http://127.0.0.1:8090/healthz`，接口文档：`http://127.0.0.1:8090/swagger/index.html`。

默认账号：`admin / <APP__APP__ADMIN_PASSWORD>`（示例 `Admin@123456`）。

### 4. 前端

```bash
cd web
pnpm install
pnpm dev          # 开发：http://localhost:5173（已放行 CORS）
pnpm build        # 生产：产物到 web/dist，由后端直接托管
```

一键构建（前端 + 后端，产物 `bin/wuye-server.exe`）：

```bash
pwsh scripts/build.ps1          # 需要内嵌前端时加 -tags embed 相关参数见脚本
```

---

## 三、目录结构

```
cmd/server            程序入口（版本注入、优雅退出）
configs               配置文件（config.yaml）
internal/
  api                  HTTP 处理层（参数绑定、错误转换）
  router               路由与权限中间件（含静态资源、Swagger）
  service              业务逻辑（物业、权限、收费、门禁、工作台…）
  repository           数据访问（通用查询、权限查询）
  model/entity         GORM 实体（带软删除与审计字段）
  middleware           鉴权、权限、日志、限流、恢复
  bootstrap            依赖装配、启动期结构校验（EnsureSchema）
  migrate              极简迁移执行器（schema_migrations）
  excel                导入导出封装
  openapi              接口文档页面
  webui                前端产物托管（磁盘 / 内嵌两种模式）
pkg/                   errs、response、cache、jwt、storage、id（雪花）、jsonx 等基础设施
migrations             版本化 SQL（按文件名顺序执行，幂等）
scripts                构建脚本、行政区划数据生成、E2E 脚本
web/                   前端工程（Vue3 + Vite + Element Plus）
docs/                  操作手册 / 开发手册 / 系统设计
```

---

## 四、测试与质量

```bash
# 静态检查
go vet ./... && gofmt -l internal pkg
cd web && npx vue-tsc --noEmit && pnpm build

# 接口 + 浏览器端到端（需服务已启动）
node scripts/e2e-api.mjs        http://127.0.0.1:8090 admin   # 脚手架基线 76 条
node scripts/e2e-property.mjs   http://127.0.0.1:8090 admin   # 物业业务 + 权限 148 条
node scripts/e2e-property-ui.mjs http://127.0.0.1:8090 admin Admin@123456  # 浏览器 40 条
```

> 浏览器脚本会临时关闭 `sys.login.captcha` / `sys.login.slider` 两个登录开关，结束后自动恢复为 `true`。

---

## 五、部署要点

- **数据库**：生产建议 `auto_migrate: false`，由发布流程显式执行迁移；`deploy/` 提供 Dockerfile 与 nginx 示例。
- **密钥**：务必覆盖 `APP__JWT__SECRET`；首次启动设置 `APP__APP__ADMIN_PASSWORD`。
- **文件存储**：默认本地 `storage/`（`/static` 前缀）；可切 OSS（`storage.type=oss` + AK/SK）。
- **行政区划**：首次启动异步导入；也可 `POST /api/v1/property/districts/sync` 手动触发（带分布式锁）。
- **前端托管**：默认托管 `web/dist`；如需单文件分发，把产物拷到 `internal/webui/dist` 后以 `-tags embed` 构建。
- **日志与监控**：日志在 `logs/`（按大小切割）；`/metrics` 暴露 Prometheus 指标，`/healthz` 用于探活。

---

## 六、文档

| 文档 | 内容 |
| --- | --- |
| [docs/操作手册.md](docs/操作手册.md) | 按角色讲清每个页面怎么用（平台管理员 / 物业公司管理员） |
| [docs/开发手册.md](docs/开发手册.md) | 分层约定、错误与响应、多租户与数据权限、迁移与幂等、常见坑 |
| [docs/物业系统设计.md](docs/物业系统设计.md) | 需求范围、权限模型、数据模型、关键流程与非功能设计 |
