package api

import (
	"github.com/gin-gonic/gin"
	"strconv"

	"web-basic/internal/principal"
	"web-basic/internal/service"
	"web-basic/pkg/errs"
	"web-basic/pkg/jsonx"
	"web-basic/pkg/response"
)

// ============================ 区域 / 楼栋 / 单元 ============================

// CreateRegion 新增区域。
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

// NodeCard 层级信息卡片。
func (h *Handler) NodeCard(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	card, err := h.Svc.NodeCard(c.Request.Context(), id, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, card)
}

// NodeChildren 树懒加载：子层级 + 直接挂载房屋（parentId=0 表示小区根级）。
func (h *Handler) NodeChildren(c *gin.Context) {
	cid := queryID(c, "communityId")
	parentID := queryID(c, "parentId")
	list, err := h.Svc.NodeChildren(c.Request.Context(), cid, parentID, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// HouseCard 房屋信息卡片（住户 + 车辆）。
func (h *Handler) HouseCard(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	card, err := h.Svc.HouseCard(c.Request.Context(), id, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, card)
}

// CommunityCard 小区信息卡片。
func (h *Handler) CommunityCard(c *gin.Context) {
	cid := queryID(c, "communityId")
	card, err := h.Svc.CommunityCard(c.Request.Context(), cid, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, card)
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

// SpaceSummary 小区基础信息概览（楼栋/单元/楼层/房屋/面积）。
func (h *Handler) SpaceSummary(c *gin.Context) {
	cid := queryID(c, "communityId")
	vo, err := h.Svc.SpaceSummary(c.Request.Context(), cid, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, vo)
}

// ExportSpaceHouses 导出小区基础信息（层级+房屋）。
func (h *Handler) ExportSpaceHouses(c *gin.Context) {
	cid := queryID(c, "communityId")
	nodeID := queryID(c, "nodeId")
	data, name, err := h.Svc.SpaceExport(c.Request.Context(), cid, nodeID, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	writeExcel(c, data, name)
}

// HouseCascader 房屋级联树（层级 + 房屋）。
func (h *Handler) HouseCascader(c *gin.Context) {
	cid := queryID(c, "communityId")
	usageType := 0
	if v := c.Query("usageType"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			usageType = n
		}
	}
	tree, err := h.Svc.HouseCascaderTree(c.Request.Context(), cid, usageType, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, tree)
}

// ClearSpace 清空小区层级与房屋（导入出错后的整体重置）。
func (h *Handler) ClearSpace(c *gin.Context) {
	// communityId 既支持查询参数，也支持 JSON 请求体（前端 POST 会把 params 放进 body）
	cid := queryID(c, "communityId")
	if cid == 0 {
		var body struct {
			CommunityID jsonx.Uint64 `json:"communityId"`
		}
		if err := c.ShouldBindJSON(&body); err == nil && body.CommunityID.Uint64() != 0 {
			cid = body.CommunityID.Uint64()
		}
	}
	stats, err := h.Svc.ClearCommunitySpace(c.Request.Context(), cid, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, stats)
}

// StructureImportTemplate 楼宇结构导入模板。
func (h *Handler) StructureImportTemplate(c *gin.Context) {
	data, err := h.Svc.SpaceImportTemplate(c.Request.Context(), queryID(c, "communityId"), principal.Must(c))
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
