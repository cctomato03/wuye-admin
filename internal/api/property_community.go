package api

import (
	"github.com/gin-gonic/gin"

	"web-basic/internal/principal"
	"web-basic/internal/service"
	"web-basic/pkg/jsonx"
	"web-basic/pkg/response"
)

// ============================ 小区 ============================

// ListCommunities 小区列表。
func (h *Handler) ListCommunities(c *gin.Context) {
	var q service.CommunityQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListCommunities(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// GetCommunity 小区详情。
func (h *Handler) GetCommunity(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	vo, err := h.Svc.GetCommunity(c.Request.Context(), id, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, vo)
}

// CreateCommunity 新增小区。
func (h *Handler) CreateCommunity(c *gin.Context) {
	var req service.CommunityReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateCommunity(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateCommunity 修改小区。
func (h *Handler) UpdateCommunity(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.CommunityReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateCommunity(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteCommunity 删除小区。
func (h *Handler) DeleteCommunity(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteCommunity(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ChangeCommunityStatus 启用/停用小区。
func (h *Handler) ChangeCommunityStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		Status int `json:"status" binding:"oneof=0 1"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.ChangeCommunityStatus(c.Request.Context(), id, req.Status, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// CommunityOptions 当前用户可见的小区下拉。
func (h *Handler) CommunityOptions(c *gin.Context) {
	list, err := h.Svc.CommunityOptions(c.Request.Context(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// DistrictTree 行政区划树。
func (h *Handler) DistrictTree(c *gin.Context) {
	tree, err := h.Svc.DistrictTree(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, tree)
}

// DistrictChildren 行政区划逐级查询（级联选择器懒加载：省 → 市 → 区县 → 街道 → 社区）。
func (h *Handler) DistrictChildren(c *gin.Context) {
	parentID := queryID(c, "parentId")
	level := int(queryID(c, "level"))
	list, err := h.Svc.ListDistricts(c.Request.Context(), parentID, level, c.Query("keyword"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// DistrictPath 按编码还原行政区划路径（省/市/区县/街道/社区）。
func (h *Handler) DistrictPath(c *gin.Context) {
	list, err := h.Svc.DistrictPath(c.Request.Context(), c.Query("code"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// SyncDistricts 重新导入行政区划数据（平台运维用）。
func (h *Handler) SyncDistricts(c *gin.Context) {
	force := c.Query("force") == "true"
	n, err := h.Svc.SyncDistricts(c.Request.Context(), force)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"imported": n})
}

// TenantOptionsForCommunity 平台侧：物业公司下拉（建小区时选归属）。
func (h *Handler) TenantOptionsForCommunity(c *gin.Context) {
	list, err := h.Svc.TenantListForCompany(c.Request.Context(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// ============================ 物业公司 ============================

// ListCompanies 物业公司列表。
func (h *Handler) ListCompanies(c *gin.Context) {
	var q service.CompanyQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListCompanies(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// CreateCompany 新增物业公司（租户 + 档案 + 管理员账号）。
func (h *Handler) CreateCompany(c *gin.Context) {
	var req service.CompanyReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.Svc.CreateCompany(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, res)
}

// UpdateCompany 修改物业公司。
func (h *Handler) UpdateCompany(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.CompanyReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateCompany(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteCompany 删除物业公司。
func (h *Handler) DeleteCompany(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteCompany(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ============================ 物业概览 ============================

// PropertyDashboard 物业概览。
func (h *Handler) PropertyDashboard(c *gin.Context) {
	cid := queryID(c, "communityId")
	vo, err := h.Svc.PropertyDashboard(c.Request.Context(), cid, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, vo)
}

// CommunityStats 小区运营概览（平台视角）。
func (h *Handler) CommunityStats(c *gin.Context) {
	page := int(queryID(c, "page"))
	size := int(queryID(c, "pageSize"))
	list, total, err := h.Svc.ListCommunityStats(c.Request.Context(), principal.Must(c), page, size)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, page, size)
}

// ============================ 员工与组织 ============================

// ListEmployees 员工列表。
func (h *Handler) ListEmployees(c *gin.Context) {
	var q service.EmployeeQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListEmployees(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// CreateEmployee 新增员工。
func (h *Handler) CreateEmployee(c *gin.Context) {
	var req service.EmployeeReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateEmployee(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateEmployee 修改员工。
func (h *Handler) UpdateEmployee(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.EmployeeReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateEmployee(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteEmployee 删除员工。
func (h *Handler) DeleteEmployee(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteEmployee(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// GrantEmployeeCommunities 设置员工小区授权。
func (h *Handler) GrantEmployeeCommunities(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		CommunityIDs jsonx.Uint64Slice `json:"communityIds"`
		DefaultID    jsonx.Uint64      `json:"defaultId"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.GrantEmployeeCommunities(c.Request.Context(), id, req.CommunityIDs.ToUint64(), req.DefaultID.Uint64(), principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// EmployeeDeptTree 组织架构树。
func (h *Handler) EmployeeDeptTree(c *gin.Context) {
	tree, err := h.Svc.EmployeeDeptTree(c.Request.Context(), principal.Must(c), queryID(c, "tenantId"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, tree)
}

// EmployeeRoleOptions 员工可分配角色。
func (h *Handler) EmployeeRoleOptions(c *gin.Context) {
	list, err := h.Svc.RoleOptions(c.Request.Context(), principal.Must(c), queryID(c, "tenantId"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}
