// Package router 路由注册：全局中间件 + 模块分组 + 权限码声明。
package router

import (
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"web-basic/internal/api"
	"web-basic/internal/config"
	"web-basic/internal/metrics"
	"web-basic/internal/middleware"
	"web-basic/internal/openapi"
	"web-basic/internal/service"
	"web-basic/internal/webui"
	"web-basic/pkg/errs"
	"web-basic/pkg/response"
)

// New 构建 Gin 引擎。
func New(cfg *config.Config, svc *service.Service, h *api.Handler) *gin.Engine {
	if cfg.App.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(
		middleware.Trace(),
		middleware.Recovery(),
		middleware.SecurityHeaders(),
		middleware.CORS(cfg.App.CorsOrigins),
		middleware.Metrics(),
	)

	// 健康检查（免认证，供探针使用）
	health := &middleware.Health{Svc: svc, Boot: time.Now()}
	health.Register(r)

	// 在线接口文档：/swagger/index.html（页面） + /swagger/openapi.json（规范）
	if cfg.Swagger.Enabled {
		index, spec := openapi.Handler(r.Routes, openapi.Info{
			Title:       cfg.Swagger.Title,
			Description: cfg.Swagger.Desc,
			Version:     middleware.BuildVersion(),
			BasePath:    "/api/v1",
		})
		r.GET("/swagger", func(c *gin.Context) { c.Redirect(http.StatusFound, "/swagger/index.html") })
		r.GET("/swagger/index.html", index)
		r.GET("/swagger/openapi.json", spec)
	}

	// Prometheus 指标（默认开启；配置 token 后需带 X-Metrics-Token 头或 ?token=）
	if cfg.Metrics.Enabled {
		r.GET("/metrics", metricsHandler(svc, cfg))
	}

	// 本地存储文件直出（OSS 模式由 bucket 域名/CDN 提供，不需此路由）
	if cfg.Storage.Type == "local" {
		dir := cfg.Storage.Local.BaseDir
		prefix := cfg.Storage.Local.URLPrefix
		if prefix == "" {
			prefix = "/static"
		}
		if err := os.MkdirAll(dir, 0o755); err == nil {
			r.Static(prefix, dir)
		}
	}

	v1 := r.Group("/api/v1")

	// ---------- 公开接口（限流保护） ----------
	pub := v1.Group("", middleware.RateLimit(svc, 30, time.Minute))
	{
		pub.GET("/auth/captcha", h.Captcha)
		pub.POST("/auth/login", h.Login)
		pub.POST("/auth/refresh", h.Refresh)
		pub.GET("/system/configs/public", h.PublicConfig)
		pub.GET("/public/site-config", h.GetSiteConfig)
		pub.GET("/public/password-policy", h.PasswordPolicy) // 密码规则：注册/改密前展示，避免用户盲试 // 站点配置：登录页渲染前就要拿到（名称/logo/背景/注册开关）
	}

	// ---------- 需登录接口 ----------
	authed := v1.Group("", middleware.Auth(svc), middleware.OperLog(svc))
	{
		authed.POST("/auth/logout", h.Logout)
		authed.GET("/auth/userinfo", h.UserInfo)
		authed.GET("/auth/routes", h.UserRoutes) // 前端动态路由表（Web Admin Pro 后端权限模式）
		authed.GET("/dicts/:code", h.DictData)   // 字典为高频只读接口，登录即可读

		// 站点个性化配置
		authed.GET("/system/site-config", middleware.Perm("system:site:list"), h.GetSiteConfig)
		authed.PUT("/system/site-config", middleware.Perm("system:site:update"), h.UpdateSiteConfig)

		authed.GET("/profile", h.MyProfile)
		authed.PUT("/profile", h.UpdateMyProfile)
		authed.PUT("/profile/password", h.ChangePassword)

		// 工作中心（每个用户都有：待办 / 消息 / 通知）
		my := authed.Group("/my")
		my.GET("/summary", h.MyWorkbenchSummary)
		my.GET("/todos", h.ListMyTodos)
		my.GET("/todos/summary", h.MyTodoSummary)
		my.POST("/todos", h.CreateMyTodo)
		my.PUT("/todos/:id/status", h.UpdateMyTodoStatus)
		my.DELETE("/todos/:id", h.DeleteMyTodo)
		my.GET("/notices", h.ListMyNotices)
		my.POST("/notices/:id/read", h.MarkNoticeRead)

		// 用户
		u := authed.Group("/system/users")
		u.GET("", middleware.Perm("system:user:list"), h.ListUsers)
		u.GET("/:id", middleware.Perm("system:user:list"), h.GetUser)
		u.POST("", middleware.Perm("system:user:create"), h.CreateUser)
		u.PUT("/:id", middleware.Perm("system:user:update"), h.UpdateUser)
		u.DELETE("", middleware.Perm("system:user:delete"), h.DeleteUsers)
		u.PUT("/:id/status", middleware.Perm("system:user:update"), h.ChangeUserStatus)
		u.PUT("/:id/reset-password", middleware.Perm("system:user:reset-pwd"), h.ResetUserPassword)
		u.PUT("/:id/roles", middleware.Perm("system:user:assign"), h.AssignUserRoles)

		// 角色
		ro := authed.Group("/system/roles")
		ro.GET("", middleware.Perm("system:role:list"), h.ListRoles)
		ro.GET("/all", middleware.Perm("system:role:list"), h.AllRoles)
		ro.POST("", middleware.Perm("system:role:create"), h.CreateRole)
		ro.PUT("/:id", middleware.Perm("system:role:update"), h.UpdateRole)
		ro.DELETE("/:id", middleware.Perm("system:role:delete"), h.DeleteRole)
		ro.GET("/:id/granted", middleware.Perm("system:role:list"), h.RoleGranted)
		ro.GET("/assignable-menus", middleware.Perm("system:role:grant"), h.AssignableMenus)
		ro.GET("/community-options", middleware.Perm("system:role:list"), h.RoleCommunityOptions)
		ro.PUT("/:id/grant", middleware.Perm("system:role:grant"), h.GrantRole)

		// 菜单
		m := authed.Group("/system/menus")
		m.GET("/tree", middleware.Perm("system:menu:list"), h.MenuTree)
		m.POST("", middleware.Perm("system:menu:create"), h.CreateMenu)
		m.PUT("/:id", middleware.Perm("system:menu:update"), h.UpdateMenu)
		m.DELETE("/:id", middleware.Perm("system:menu:delete"), h.DeleteMenu)

		// 部门
		d := authed.Group("/system/depts")
		d.GET("/tree", middleware.Perm("system:dept:list"), h.DeptTree)
		d.GET("/detail/:id", middleware.Perm("system:dept:list"), h.DeptDetail)
		d.POST("", middleware.Perm("system:dept:create"), h.CreateDept)
		d.PUT("/:id", middleware.Perm("system:dept:update"), h.UpdateDept)
		d.DELETE("/:id", middleware.Perm("system:dept:delete"), h.DeleteDept)

		// 岗位
		p := authed.Group("/system/posts")
		p.GET("", middleware.Perm("system:post:list"), h.ListPosts)
		p.POST("", middleware.Perm("system:post:create"), h.CreatePost)
		p.PUT("/:id", middleware.Perm("system:post:update"), h.UpdatePost)
		p.DELETE("/:id", middleware.Perm("system:post:delete"), h.DeletePost)

		// 字典
		dd := authed.Group("/system/dict")
		dd.GET("/types", middleware.Perm("system:dict:list"), h.ListDictTypes)
		dd.POST("/types", middleware.Perm("system:dict:create"), h.CreateDictType)
		dd.PUT("/types/:id", middleware.Perm("system:dict:update"), h.UpdateDictType)
		dd.DELETE("/types/:id", middleware.Perm("system:dict:delete"), h.DeleteDictType)
		dd.POST("/data", middleware.Perm("system:dict:create"), h.CreateDictData)
		dd.PUT("/data/:id", middleware.Perm("system:dict:update"), h.UpdateDictData)
		dd.DELETE("/data/:id", middleware.Perm("system:dict:delete"), h.DeleteDictData)

		// 参数
		cf := authed.Group("/system/configs")
		cf.GET("", middleware.Perm("system:config:list"), h.ListConfigs)
		cf.POST("", middleware.Perm("system:config:create"), h.CreateConfig)
		cf.PUT("/:id", middleware.Perm("system:config:update"), h.UpdateConfig)
		cf.DELETE("/:id", middleware.Perm("system:config:delete"), h.DeleteConfig)

		// 租户
		tn := authed.Group("/system/tenants")
		tn.GET("", middleware.Perm("system:tenant:list"), h.ListTenants)
		tn.POST("", middleware.Perm("system:tenant:create"), h.CreateTenant)
		tn.PUT("/:id", middleware.Perm("system:tenant:update"), h.UpdateTenant)
		tn.DELETE("/:id", middleware.Perm("system:tenant:delete"), h.DeleteTenant)

		// 代码生成器
		gen := authed.Group("/tool/gen")
		gen.GET("/tables", middleware.Perm("tool:gen:list"), h.GenTables)
		gen.GET("/tables/:table", middleware.Perm("tool:gen:list"), h.GenTableDetail)
		gen.GET("/tables/:table/preview", middleware.Perm("tool:gen:preview"), h.GenPreview)
		gen.POST("/download", middleware.Perm("tool:gen:download"), h.GenDownload)

		// Excel 导出 / 导入
		u.GET("/export", middleware.Perm("system:user:export"), h.ExportUsers)
		u.GET("/import-template", middleware.Perm("system:user:import"), h.UserImportTemplate)
		u.POST("/import", middleware.Perm("system:user:import"), h.ImportUsers)

		// 通知公告
		nt := authed.Group("/system/notices")
		nt.GET("", middleware.Perm("system:notice:list"), h.ListNotices)
		nt.GET("/:id", middleware.Perm("system:notice:list"), h.GetNotice)
		nt.POST("", middleware.Perm("system:notice:create"), h.CreateNotice)
		nt.PUT("/:id", middleware.Perm("system:notice:update"), h.UpdateNotice)
		nt.DELETE("/:id", middleware.Perm("system:notice:delete"), h.DeleteNotice)
		nt.POST("/:id/publish", middleware.Perm("system:notice:publish"), h.PublishNotice)

		// 站内信（当前登录用户自己的，登录即可用，无需额外权限码）
		// 消息与待办推送（后台手工发起；开放接口见下方 open 分组）
		push := authed.Group("/system/push")
		push.POST("/messages", middleware.Perm("system:push:create"), h.PushMessage)
		push.POST("/todos", middleware.Perm("system:push:create"), h.PushTodo)

		msg := authed.Group("/messages")
		msg.GET("", h.ListMyMessages)
		msg.GET("/unread-count", h.UnreadMessageCount)
		msg.PUT("/:id/read", h.MarkMessageRead)
		msg.PUT("/read-all", h.MarkAllMessagesRead)
		msg.DELETE("/:id", h.DeleteMyMessage)

		// 文件
		f := authed.Group("/files")
		f.POST("/upload", middleware.Perm("system:file:upload"), h.UploadFile)
		// OSS 直传：签名 + 回执（存储为本地磁盘时签名接口会返回「不支持」，前端自动回退到 /files/upload）
		f.POST("/oss-ticket", middleware.Perm("system:file:upload"), h.OSSUploadTicket)
		f.POST("/oss-record", middleware.Perm("system:file:upload"), h.OSSUploadRecord)
		f.GET("", middleware.Perm("system:file:list"), h.ListFiles)
		f.DELETE("/:id", middleware.Perm("system:file:delete"), h.DeleteFile)
		f.GET("/:id/url", h.FileURL)

		// 监控
		mon := authed.Group("/monitor")
		mon.GET("/login-logs", middleware.Perm("monitor:login-log:list"), h.ListLoginLogs)
		mon.GET("/oper-logs", middleware.Perm("monitor:oper-log:list"), h.ListOperLogs)
		mon.GET("/oper-logs/export", middleware.Perm("monitor:oper-log:export"), h.ExportOperLogs)
		mon.GET("/online", middleware.Perm("monitor:online:list"), h.OnlineUsers)
		mon.DELETE("/online", middleware.Perm("monitor:online:list"), h.KickUser)
		mon.GET("/server", middleware.Perm("monitor:server:view"), h.ServerInfo)

		// 首页概览
		authed.GET("/dashboard/summary", h.DashboardSummary)

		// 定时任务
		j := authed.Group("/system/jobs")
		j.GET("", middleware.Perm("system:job:list"), h.ListJobs)
		j.GET("/meta", middleware.Perm("system:job:list"), h.JobMeta)
		j.POST("", middleware.Perm("system:job:create"), h.CreateJob)
		j.PUT("/:id", middleware.Perm("system:job:update"), h.UpdateJob)
		j.PUT("/:id/status", middleware.Perm("system:job:update"), h.SetJobStatus)
		j.POST("/:id/run", middleware.Perm("system:job:run"), h.RunJob)
		j.DELETE("/:id", middleware.Perm("system:job:delete"), h.DeleteJob)
		j.GET("/logs", middleware.Perm("system:job:list"), h.ListJobLogs)
	}

	// ---------- 物业管理（小区级数据隔离：所有接口按授权小区过滤） ----------
	pm := authed.Group("/property")
	{
		// 小区
		pm.GET("/communities", middleware.Perm("property:community:list"), h.ListCommunities)
		pm.GET("/communities/options", h.CommunityOptions) // 下拉：登录即可读，仅返回被授权小区
		pm.GET("/communities/:id", middleware.Perm("property:community:list"), h.GetCommunity)
		pm.POST("/communities", middleware.Perm("property:community:create"), h.CreateCommunity)
		pm.PUT("/communities/:id", middleware.Perm("property:community:update"), h.UpdateCommunity)
		pm.DELETE("/communities/:id", middleware.Perm("property:community:delete"), h.DeleteCommunity)
		pm.PUT("/communities/:id/status", middleware.Perm("property:community:status"), h.ChangeCommunityStatus)
		pm.GET("/districts/tree", middleware.Perm("property:community:list"), h.DistrictTree)
		pm.GET("/districts/children", middleware.Perm("property:community:list"), h.DistrictChildren)
		pm.GET("/districts/path", middleware.Perm("property:community:list"), h.DistrictPath)
		pm.POST("/districts/sync", middleware.Perm("property:community:update"), h.SyncDistricts)
		pm.GET("/tenants/options", middleware.Perm("property:company:list"), h.TenantOptionsForCommunity)

		// 物业公司（平台侧）
		pm.GET("/companies", middleware.Perm("property:company:list"), h.ListCompanies)
		pm.POST("/companies", middleware.Perm("property:company:create"), h.CreateCompany)
		pm.PUT("/companies/:id", middleware.Perm("property:company:update"), h.UpdateCompany)
		pm.DELETE("/companies/:id", middleware.Perm("property:company:delete"), h.DeleteCompany)

		// 物业概览
		pm.GET("/dashboard", middleware.Perm("property:dashboard:view"), h.PropertyDashboard)
		pm.GET("/dashboard/communities", middleware.Perm("property:dashboard:view"), h.CommunityStats)

		// 员工与组织架构
		pm.GET("/employees", middleware.Perm("property:employee:list"), h.ListEmployees)
		pm.POST("/employees", middleware.Perm("property:employee:create"), h.CreateEmployee)
		pm.PUT("/employees/:id", middleware.Perm("property:employee:update"), h.UpdateEmployee)
		pm.DELETE("/employees/:id", middleware.Perm("property:employee:delete"), h.DeleteEmployee)
		pm.PUT("/employees/:id/communities", middleware.Perm("property:employee:grant"), h.GrantEmployeeCommunities)
		pm.GET("/employees/dept-tree", middleware.Perm("property:employee:list"), h.EmployeeDeptTree)
		pm.GET("/employees/roles", middleware.Perm("property:employee:list"), h.EmployeeRoleOptions)

		// 楼宇结构
		// 自定义层级（新模型）
		pm.GET("/nodes/tree", middleware.Perm("property:structure:list"), h.NodeTree)
		pm.GET("/nodes/options", middleware.Perm("property:structure:list"), h.NodeOptions)
		pm.GET("/spaces/summary", middleware.Perm("property:structure:list"), h.SpaceSummary)
		pm.GET("/spaces/community-card", middleware.Perm("property:structure:list"), h.CommunityCard)
		pm.GET("/nodes/children", middleware.Perm("property:structure:list"), h.NodeChildren)
		pm.GET("/nodes/:id/card", middleware.Perm("property:structure:list"), h.NodeCard)
		pm.GET("/structures/houses/:id/card", middleware.Perm("property:structure:list"), h.HouseCard)
		pm.GET("/spaces/export", middleware.Perm("property:structure:list"), h.ExportSpaceHouses)
		pm.POST("/nodes", middleware.Perm("property:structure:create"), h.CreateNode)
		pm.PUT("/nodes/:id", middleware.Perm("property:structure:update"), h.UpdateNode)
		pm.DELETE("/nodes/:id", middleware.Perm("property:structure:delete"), h.DeleteNode)
		pm.GET("/structures/houses", middleware.Perm("property:structure:list"), h.ListHouses)
		pm.POST("/structures/houses", middleware.Perm("property:structure:create"), h.CreateHouse)
		pm.POST("/structures/houses/batch", middleware.Perm("property:structure:create"), h.BatchCreateHouses)
		pm.PUT("/structures/houses/:id", middleware.Perm("property:structure:update"), h.UpdateHouse)
		pm.DELETE("/structures/houses/:id", middleware.Perm("property:structure:delete"), h.DeleteHouse)
		pm.GET("/structures/house-types", middleware.Perm("property:structure:list"), h.ListHouseTypes)
		pm.POST("/structures/house-types", middleware.Perm("property:structure:create"), h.CreateHouseType)
		pm.PUT("/structures/house-types/:id", middleware.Perm("property:structure:update"), h.UpdateHouseType)
		pm.DELETE("/structures/house-types/:id", middleware.Perm("property:structure:delete"), h.DeleteHouseType)
		pm.GET("/structures/import-template", middleware.Perm("property:structure:import"), h.StructureImportTemplate)
		pm.POST("/structures/import", middleware.Perm("property:structure:import"), h.ImportStructures)

		// 住户
		pm.GET("/residents", middleware.Perm("property:resident:list"), h.ListResidents)
		pm.GET("/residents/lookup", middleware.Perm("property:resident:list"), h.LookupResident)
		pm.GET("/residents/:id", middleware.Perm("property:resident:list"), h.GetResident)
		pm.POST("/residents", middleware.Perm("property:resident:create"), h.CreateResident)
		pm.PUT("/residents/:id", middleware.Perm("property:resident:update"), h.UpdateResident)
		pm.DELETE("/residents/:id", middleware.Perm("property:resident:delete"), h.DeleteResident)
		pm.POST("/residents/bind", middleware.Perm("property:resident:bind"), h.BindResidentHouse)
		pm.PUT("/residents/relations/:id/unbind", middleware.Perm("property:resident:bind"), h.UnbindResidentHouse)
		pm.PUT("/residents/relations/:id/renew", middleware.Perm("property:resident:bind"), h.RenewResidentHouse)
		pm.PUT("/residents/relations/:id/restore", middleware.Perm("property:resident:bind"), h.RestoreResidentHouse)
		pm.GET("/residents/:id/history", middleware.Perm("property:resident:list"), h.ListResidentHistory)
		pm.GET("/residents/houses", middleware.Perm("property:resident:list"), h.ListHouseResidents)
		pm.GET("/residents/:id/faces", middleware.Perm("property:resident:list"), h.ListResidentFaces)
		pm.POST("/residents/:id/faces", middleware.Perm("property:resident:face"), h.AddResidentFace)
		pm.DELETE("/residents/faces/:id", middleware.Perm("property:resident:face"), h.DeleteResidentFace)

		// 访客
		pm.GET("/visitors", middleware.Perm("property:visitor:list"), h.ListVisitors)
		pm.GET("/visitors/:id", middleware.Perm("property:visitor:list"), h.GetVisitor)
		pm.POST("/visitors", middleware.Perm("property:visitor:create"), h.CreateVisitor)
		pm.PUT("/visitors/:id/status", middleware.Perm("property:visitor:leave"), h.ChangeVisitorStatus)
		pm.DELETE("/visitors/:id", middleware.Perm("property:visitor:delete"), h.DeleteVisitor)

		// 其他人员
		pm.GET("/other-people", middleware.Perm("property:other:list"), h.ListOtherPeople)
		pm.POST("/other-people", middleware.Perm("property:other:create"), h.CreateOtherPerson)
		pm.PUT("/other-people/:id", middleware.Perm("property:other:update"), h.UpdateOtherPerson)
		pm.DELETE("/other-people/:id", middleware.Perm("property:other:delete"), h.DeleteOtherPerson)
		pm.GET("/person-categories", middleware.Perm("property:other:list"), h.ListPersonCategories)
		pm.POST("/person-categories", middleware.Perm("property:other:category"), h.CreatePersonCategory)
		pm.PUT("/person-categories/:id", middleware.Perm("property:other:category"), h.UpdatePersonCategory)
		pm.DELETE("/person-categories/:id", middleware.Perm("property:other:category"), h.DeletePersonCategory)

		// 单位/商铺
		pm.GET("/merchants", middleware.Perm("property:merchant:list"), h.ListMerchants)
		pm.POST("/merchants", middleware.Perm("property:merchant:create"), h.CreateMerchant)
		pm.PUT("/merchants/:id", middleware.Perm("property:merchant:update"), h.UpdateMerchant)
		pm.DELETE("/merchants/:id", middleware.Perm("property:merchant:delete"), h.DeleteMerchant)

		// 车辆与车位
		pm.GET("/vehicles", middleware.Perm("property:vehicle:list"), h.ListVehicles)
		pm.POST("/vehicles", middleware.Perm("property:vehicle:create"), h.CreateVehicle)
		pm.PUT("/vehicles/:id", middleware.Perm("property:vehicle:update"), h.UpdateVehicle)
		pm.DELETE("/vehicles/:id", middleware.Perm("property:vehicle:delete"), h.DeleteVehicle)
		pm.GET("/parking-spaces", middleware.Perm("property:vehicle:list"), h.ListParkingSpaces)
		pm.POST("/parking-spaces", middleware.Perm("property:vehicle:space"), h.CreateParkingSpace)
		pm.PUT("/parking-spaces/:id", middleware.Perm("property:vehicle:space"), h.UpdateParkingSpace)
		pm.DELETE("/parking-spaces/:id", middleware.Perm("property:vehicle:space"), h.DeleteParkingSpace)
		pm.GET("/vehicle-logs", middleware.Perm("property:vehicle:list"), h.ListVehicleAccessLogs)

		// 设备
		pm.GET("/devices", middleware.Perm("property:device:list"), h.ListDevices)
		pm.GET("/devices/options", h.DeviceOptions)
		pm.GET("/devices/summary", middleware.Perm("property:device:list"), h.DeviceSummary)
		pm.POST("/devices", middleware.Perm("property:device:create"), h.CreateDevice)
		pm.PUT("/devices/:id", middleware.Perm("property:device:update"), h.UpdateDevice)
		pm.DELETE("/devices/:id", middleware.Perm("property:device:delete"), h.DeleteDevice)
		pm.GET("/devices/import-template", middleware.Perm("property:device:import"), h.DeviceImportTemplate)
		pm.POST("/devices/import", middleware.Perm("property:device:import"), h.ImportDevices)

		// 门禁授权
		pm.GET("/access/grants", middleware.Perm("property:access:list"), h.ListAccessGrants)
		pm.GET("/access/stats", middleware.Perm("property:access:list"), h.AccessGrantStats)
		pm.POST("/access/issue", middleware.Perm("property:access:issue"), h.IssuePersonAccess)
		pm.POST("/access/retry", middleware.Perm("property:access:retry"), h.RetryAccessGrants)
		pm.POST("/access/revoke", middleware.Perm("property:access:revoke"), h.RevokeAccessGrants)
		pm.GET("/access/cards", middleware.Perm("property:access:list"), h.ListAccessCards)
		pm.POST("/access/cards", middleware.Perm("property:access:card"), h.CreateAccessCard)
		pm.DELETE("/access/cards/:id", middleware.Perm("property:access:card"), h.DeleteAccessCard)

		// 收费
		pm.GET("/fee/items", middleware.Perm("property:fee:item:list"), h.ListFeeItems)
		pm.POST("/fee/items", middleware.Perm("property:fee:item:create"), h.CreateFeeItem)
		pm.PUT("/fee/items/:id", middleware.Perm("property:fee:item:update"), h.UpdateFeeItem)
		pm.DELETE("/fee/items/:id", middleware.Perm("property:fee:item:delete"), h.DeleteFeeItem)
		pm.GET("/fee/items/:id/standards", middleware.Perm("property:fee:item:list"), h.ListFeeStandards)
		pm.POST("/fee/standards", middleware.Perm("property:fee:item:create"), h.CreateFeeStandard)
		pm.DELETE("/fee/standards/:id", middleware.Perm("property:fee:item:update"), h.DeleteFeeStandard)
		pm.GET("/fee/meters", middleware.Perm("property:fee:meter:list"), h.ListMeters)
		pm.POST("/fee/meters", middleware.Perm("property:fee:meter:create"), h.CreateMeter)
		pm.DELETE("/fee/meters/:id", middleware.Perm("property:fee:meter:create"), h.DeleteMeter)
		pm.GET("/fee/readings", middleware.Perm("property:fee:meter:list"), h.ListMeterReadings)
		pm.POST("/fee/readings", middleware.Perm("property:fee:meter:reading"), h.CreateMeterReading)
		pm.POST("/fee/bills/generate", middleware.Perm("property:fee:bill:generate"), h.GenerateBills)
		pm.POST("/fee/bills/generate-meter", middleware.Perm("property:fee:meter:generate"), h.GenerateMeterBills)
		pm.GET("/fee/bills", middleware.Perm("property:fee:bill:list"), h.ListBills)
		pm.GET("/fee/bills/:id", middleware.Perm("property:fee:bill:list"), h.GetBill)
		pm.POST("/fee/bills/:id/pay", middleware.Perm("property:fee:bill:pay"), h.PayBill)
		pm.PUT("/fee/bills/:id/cancel", middleware.Perm("property:fee:bill:cancel"), h.CancelBill)
		pm.GET("/fee/summary", middleware.Perm("property:fee:bill:list"), h.BillSummary)
		pm.GET("/fee/payments", middleware.Perm("property:fee:payment:list"), h.ListPayments)
		pm.GET("/fee/accounts", middleware.Perm("property:fee:payment:list"), h.ListAccounts)
		pm.POST("/fee/accounts/recharge", middleware.Perm("property:fee:bill:pay"), h.RechargeAccount)
		pm.GET("/fee/accounts/logs", middleware.Perm("property:fee:payment:list"), h.ListAccountLogs)
	}

	// ---------- 开放接口（业务系统对接：X-API-KEY，未配置时整体 404） ----------
	open := v1.Group("/open", middleware.APIKey(svc))
	{
		open.POST("/messages", h.OpenPushMessage)
		open.POST("/todos", h.OpenPushTodo)
	}

	// 未知 API 统一 404
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			response.Fail(c, errs.ErrNotFound)
			return
		}
		serveSPA(c)
	})

	return r
}

// metricsHandler 输出 Prometheus 文本格式指标（含 MySQL / Redis 探针）。
func metricsHandler(svc *service.Service, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.Metrics.Token != "" {
			token := c.GetHeader("X-Metrics-Token")
			if token == "" {
				token = c.Query("token")
			}
			if token != cfg.Metrics.Token {
				c.String(http.StatusUnauthorized, "metrics token required")
				return
			}
		}
		ctx := c.Request.Context()
		extras := metrics.Extras{
			Build: map[string]string{
				"version":   middleware.BuildVersion(),
				"commit":    middleware.BuildCommit(),
				"buildTime": middleware.BuildTime(),
			},
			MySQL: map[string]float64{},
		}
		if sqlDB, err := svc.Repo.DB.DB(); err == nil {
			st := sqlDB.Stats()
			extras.MySQL["open"] = float64(st.OpenConnections)
			extras.MySQL["in_use"] = float64(st.InUse)
			extras.MySQL["idle"] = float64(st.Idle)
		}
		if svc.Cache.Ping(ctx) == nil {
			extras.Redis = 1
		}
		c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		metrics.Default.Render(c.Writer, extras)
	}
}

// serveSPA 托管前端产物并支持 history 路由回退（前后端分离时此段不会命中）。
func serveSPA(c *gin.Context) {
	static, err := webui.Dist()
	if err != nil {
		c.String(http.StatusNotFound, "前端产物未就绪：请构建 web/dist 或使用前后端分离部署")
		return
	}
	p := strings.TrimPrefix(c.Request.URL.Path, "/")
	if p != "" {
		if f, err := static.Open(p); err == nil {
			defer f.Close()
			http.FileServer(http.FS(static)).ServeHTTP(c.Writer, c.Request)
			return
		}
	}
	index, err := fs.ReadFile(static, "index.html")
	if err != nil {
		c.String(http.StatusNotFound, "index.html 不存在")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", index)
}
