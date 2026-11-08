package api

import (
	"github.com/gin-gonic/gin"

	"web-basic/internal/principal"
	"web-basic/internal/service"
	"web-basic/pkg/errs"
	"web-basic/pkg/response"
)

// ============================ 区域 / 楼栋 / 单元 ============================

// CreateRegion 新增区域。
func (h *Handler) CreateRegion(c *gin.Context) {
	var req service.RegionReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateRegion(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateRegion 修改区域。
func (h *Handler) UpdateRegion(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.RegionReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateRegion(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteRegion 删除区域。
func (h *Handler) DeleteRegion(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteRegion(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// CreateBuilding 新增楼栋。
func (h *Handler) CreateBuilding(c *gin.Context) {
	var req service.BuildingReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateBuilding(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateBuilding 修改楼栋。
func (h *Handler) UpdateBuilding(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.BuildingReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateBuilding(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteBuilding 删除楼栋。
func (h *Handler) DeleteBuilding(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteBuilding(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// CreateUnit 新增单元。
func (h *Handler) CreateUnit(c *gin.Context) {
	var req service.UnitReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateUnit(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateUnit 修改单元。
func (h *Handler) UpdateUnit(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.UnitReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateUnit(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteUnit 删除单元。
func (h *Handler) DeleteUnit(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteUnit(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// NodeTree 自定义层级树（层级管理页）。
func (h *Handler) NodeTree(c *gin.Context) {
	cid := queryID(c, "communityId")
	tree, err := h.Svc.NodeTree(c.Request.Context(), cid, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, tree)
}

// CreateNode 新增层级。
func (h *Handler) CreateNode(c *gin.Context) {
	var req service.NodeReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateNode(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateNode 修改层级。
func (h *Handler) UpdateNode(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.NodeReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateNode(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteNode 删除层级。
func (h *Handler) DeleteNode(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteNode(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// NodeOptions 层级下拉（房屋/设备选择用）。
func (h *Handler) NodeOptions(c *gin.Context) {
	cid := queryID(c, "communityId")
	leafOnly := c.Query("leafOnly") == "true"
	list, err := h.Svc.NodeOptions(c.Request.Context(), cid, leafOnly, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// StructureTree 楼宇结构树。
func (h *Handler) StructureTree(c *gin.Context) {
	cid := queryID(c, "communityId")
	tree, err := h.Svc.StructureTree(c.Request.Context(), cid, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, tree)
}

// ============================ 房屋 ============================

// ListHouses 房屋列表。
func (h *Handler) ListHouses(c *gin.Context) {
	var q service.HouseQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListHouses(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// CreateHouse 新增房屋。
func (h *Handler) CreateHouse(c *gin.Context) {
	var req service.HouseReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateHouse(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateHouse 修改房屋。
func (h *Handler) UpdateHouse(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.HouseReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateHouse(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteHouse 删除房屋。
func (h *Handler) DeleteHouse(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteHouse(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// BatchCreateHouses 批量生成房屋。
func (h *Handler) BatchCreateHouses(c *gin.Context) {
	var req service.BatchHouseReq
	if !bindJSON(c, &req) {
		return
	}
	n, err := h.Svc.BatchCreateHouses(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"created": n})
}

// ============================ 房屋类型 ============================

// ListHouseTypes 房屋类型列表。
func (h *Handler) ListHouseTypes(c *gin.Context) {
	cid := queryID(c, "communityId")
	list, err := h.Svc.ListHouseTypes(c.Request.Context(), cid, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// CreateHouseType 新增房屋类型。
func (h *Handler) CreateHouseType(c *gin.Context) {
	var req service.HouseTypeReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateHouseType(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateHouseType 修改房屋类型。
func (h *Handler) UpdateHouseType(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.HouseTypeReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateHouseType(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteHouseType 删除房屋类型。
func (h *Handler) DeleteHouseType(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteHouseType(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ============================ 结构导入 ============================

// StructureImportTemplate 楼宇结构导入模板。
func (h *Handler) StructureImportTemplate(c *gin.Context) {
	data, err := h.Svc.StructureImportTemplate()
	if err != nil {
		response.Fail(c, err)
		return
	}
	writeExcel(c, data, "楼宇结构导入模板.xlsx")
}

// ImportStructures 导入楼宇结构。
func (h *Handler) ImportStructures(c *gin.Context) {
	cid := queryID(c, "communityId")
	if cid == 0 {
		response.Fail(c, errs.Invalid("请先选择小区"))
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.Invalid("请选择要上传的 Excel 文件"))
		return
	}
	if file.Size > 10<<20 {
		response.Fail(c, errs.Invalid("文件不能超过 10MB"))
		return
	}
	f, err := file.Open()
	if err != nil {
		response.Fail(c, errs.Invalid("文件读取失败，请重新选择文件"))
		return
	}
	defer func() { _ = f.Close() }()
	res, err := h.Svc.ImportStructures(c.Request.Context(), cid, f, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, res)
}

// ============================ 设备 ============================

// ListDevices 设备列表。
func (h *Handler) ListDevices(c *gin.Context) {
	var q service.DeviceQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListDevices(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// CreateDevice 新增设备。
func (h *Handler) CreateDevice(c *gin.Context) {
	var req service.DeviceReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateDevice(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateDevice 修改设备。
func (h *Handler) UpdateDevice(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.DeviceReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateDevice(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteDevice 删除设备。
func (h *Handler) DeleteDevice(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteDevice(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeviceOptions 门禁设备下拉。
func (h *Handler) DeviceOptions(c *gin.Context) {
	cid := queryID(c, "communityId")
	list, err := h.Svc.DeviceOptions(c.Request.Context(), cid, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// DeviceSummary 设备概览。
func (h *Handler) DeviceSummary(c *gin.Context) {
	cid := queryID(c, "communityId")
	st, err := h.Svc.DeviceSummary(c.Request.Context(), cid, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, st)
}

// DeviceImportTemplate 设备导入模板。
func (h *Handler) DeviceImportTemplate(c *gin.Context) {
	data, err := h.Svc.DeviceImportTemplate()
	if err != nil {
		response.Fail(c, err)
		return
	}
	writeExcel(c, data, "设备导入模板.xlsx")
}

// ImportDevices 导入设备。
func (h *Handler) ImportDevices(c *gin.Context) {
	cid := queryID(c, "communityId")
	if cid == 0 {
		response.Fail(c, errs.Invalid("请先选择小区"))
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, errs.Invalid("请选择要上传的 Excel 文件"))
		return
	}
	if file.Size > 10<<20 {
		response.Fail(c, errs.Invalid("文件不能超过 10MB"))
		return
	}
	f, err := file.Open()
	if err != nil {
		response.Fail(c, errs.Invalid("文件读取失败，请重新选择文件"))
		return
	}
	defer func() { _ = f.Close() }()
	res, err := h.Svc.ImportDevices(c.Request.Context(), cid, f, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, res)
}
