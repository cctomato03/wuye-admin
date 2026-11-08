package api

import (
	"github.com/gin-gonic/gin"

	"web-basic/internal/principal"
	"web-basic/internal/service"
	"web-basic/pkg/jsonx"
	"web-basic/pkg/response"
)

// ============================ 住户 ============================

// ListResidents 住户列表（房屋关系维度）。
func (h *Handler) ListResidents(c *gin.Context) {
	var q service.ResidentQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListResidents(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// GetResident 住户详情。
func (h *Handler) GetResident(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	vo, err := h.Svc.GetResident(c.Request.Context(), id, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, vo)
}

// CreateResident 登记住户。
func (h *Handler) CreateResident(c *gin.Context) {
	var req service.ResidentReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.Svc.CreateResident(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, res)
}

// UpdateResident 修改住户。
func (h *Handler) UpdateResident(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.ResidentReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateResident(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteResident 删除住户。
func (h *Handler) DeleteResident(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteResident(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// LookupResident 按证件号回显住户资料。
func (h *Handler) LookupResident(c *gin.Context) {
	idType := int(queryID(c, "idType"))
	idNo := c.Query("idNo")
	vo, err := h.Svc.LookupResidentByIDNo(c.Request.Context(), idType, idNo, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, vo)
}

// BindResidentHouse 绑定住户房屋。
func (h *Handler) BindResidentHouse(c *gin.Context) {
	var req service.BindHouseReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.BindHousePublic(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UnbindResidentHouse 解除住户房屋关系。
func (h *Handler) UnbindResidentHouse(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.UnbindHouse(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// RenewResidentHouse 续期。
func (h *Handler) RenewResidentHouse(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.RenewHouseReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.RenewHouse(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// RestoreResidentHouse 还原关系。
func (h *Handler) RestoreResidentHouse(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.RestoreHouse(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ListResidentHistory 住户关系变更历史。
func (h *Handler) ListResidentHistory(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	list, err := h.Svc.ListResidentHistory(c.Request.Context(), id, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// ListHouseResidents 房屋下的住户。
func (h *Handler) ListHouseResidents(c *gin.Context) {
	hid := queryID(c, "houseId")
	var status *int
	if v := c.Query("status"); v != "" {
		n := int(queryID(c, "status"))
		status = &n
	}
	list, err := h.Svc.ListHouseResidents(c.Request.Context(), hid, status, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// AddResidentFace 新增人脸。
func (h *Handler) AddResidentFace(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.FaceReq
	if !bindJSON(c, &req) {
		return
	}
	fid, err := h.Svc.AddResidentFace(c.Request.Context(), id, &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, fid)
}

// DeleteResidentFace 删除人脸。
func (h *Handler) DeleteResidentFace(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteResidentFace(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ListResidentFaces 人脸列表。
func (h *Handler) ListResidentFaces(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	list, err := h.Svc.ListResidentFaces(c.Request.Context(), id, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// ============================ 访客 ============================

// ListVisitors 访客列表。
func (h *Handler) ListVisitors(c *gin.Context) {
	var q service.VisitorQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListVisitors(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// GetVisitor 访客详情。
func (h *Handler) GetVisitor(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	vo, err := h.Svc.GetVisitor(c.Request.Context(), id, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, vo)
}

// CreateVisitor 访客登记。
func (h *Handler) CreateVisitor(c *gin.Context) {
	var req service.VisitorReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateVisitor(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// ChangeVisitorStatus 访客状态流转（enter/leave/cancel）。
func (h *Handler) ChangeVisitorStatus(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		Action string `json:"action" binding:"required,oneof=enter leave cancel"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.ChangeVisitorStatus(c.Request.Context(), id, req.Action, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteVisitor 删除访客。
func (h *Handler) DeleteVisitor(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteVisitor(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ============================ 其他人员 ============================

// ListPersonCategories 人员类别列表。
func (h *Handler) ListPersonCategories(c *gin.Context) {
	cid := queryID(c, "communityId")
	list, err := h.Svc.ListPersonCategories(c.Request.Context(), cid, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// CreatePersonCategory 新增类别。
func (h *Handler) CreatePersonCategory(c *gin.Context) {
	var req service.PersonCategoryReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreatePersonCategory(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdatePersonCategory 修改类别。
func (h *Handler) UpdatePersonCategory(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.PersonCategoryReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdatePersonCategory(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeletePersonCategory 删除类别。
func (h *Handler) DeletePersonCategory(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeletePersonCategory(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ListOtherPeople 其他人员列表。
func (h *Handler) ListOtherPeople(c *gin.Context) {
	var q service.OtherPersonQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListOtherPeople(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// CreateOtherPerson 登记其他人员。
func (h *Handler) CreateOtherPerson(c *gin.Context) {
	var req service.OtherPersonReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateOtherPerson(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateOtherPerson 修改其他人员。
func (h *Handler) UpdateOtherPerson(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.OtherPersonReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateOtherPerson(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteOtherPerson 删除其他人员。
func (h *Handler) DeleteOtherPerson(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteOtherPerson(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ============================ 单位 ============================

// ListMerchants 单位列表。
func (h *Handler) ListMerchants(c *gin.Context) {
	var q service.MerchantQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListMerchants(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// CreateMerchant 新增单位。
func (h *Handler) CreateMerchant(c *gin.Context) {
	var req service.MerchantReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateMerchant(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateMerchant 修改单位。
func (h *Handler) UpdateMerchant(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.MerchantReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateMerchant(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteMerchant 删除单位。
func (h *Handler) DeleteMerchant(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteMerchant(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ============================ 车辆 ============================

// ListVehicles 车辆列表。
func (h *Handler) ListVehicles(c *gin.Context) {
	var q service.VehicleQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListVehicles(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// CreateVehicle 登记车辆。
func (h *Handler) CreateVehicle(c *gin.Context) {
	var req service.VehicleReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateVehicle(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateVehicle 修改车辆。
func (h *Handler) UpdateVehicle(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.VehicleReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateVehicle(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteVehicle 删除车辆。
func (h *Handler) DeleteVehicle(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteVehicle(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ListParkingSpaces 车位列表。
func (h *Handler) ListParkingSpaces(c *gin.Context) {
	var q service.ParkingSpaceQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListParkingSpaces(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// CreateParkingSpace 新增车位。
func (h *Handler) CreateParkingSpace(c *gin.Context) {
	var req service.ParkingSpaceReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateParkingSpace(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateParkingSpace 修改车位。
func (h *Handler) UpdateParkingSpace(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.ParkingSpaceReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateParkingSpace(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteParkingSpace 删除车位。
func (h *Handler) DeleteParkingSpace(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteParkingSpace(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ListVehicleAccessLogs 车辆进出记录。
func (h *Handler) ListVehicleAccessLogs(c *gin.Context) {
	var q service.VehicleAccessLogQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListVehicleAccessLogs(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// ============================ 门禁授权 ============================

// ListAccessGrants 授权明细列表。
func (h *Handler) ListAccessGrants(c *gin.Context) {
	var q service.AccessGrantQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListAccessGrants(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// AccessGrantStats 授权统计。
func (h *Handler) AccessGrantStats(c *gin.Context) {
	cid := queryID(c, "communityId")
	st, err := h.Svc.AccessGrantStats(c.Request.Context(), cid, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, st)
}

// RetryAccessGrants 重试下发。
func (h *Handler) RetryAccessGrants(c *gin.Context) {
	var req struct {
		IDs         jsonx.Uint64Slice `json:"ids"`
		CommunityID jsonx.Uint64      `json:"communityId"`
	}
	if !bindJSON(c, &req) {
		return
	}
	success, failed, err := h.Svc.RetryGrants(c.Request.Context(), req.IDs.ToUint64(), req.CommunityID.Uint64(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"success": success, "failed": failed})
}

// RevokeAccessGrants 手工解除授权。
func (h *Handler) RevokeAccessGrants(c *gin.Context) {
	var req struct {
		IDs jsonx.Uint64Slice `json:"ids" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	n, err := h.Svc.RevokeGrants(c.Request.Context(), req.IDs.ToUint64(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"revoked": n})
}

// IssuePersonAccess 手工触发某人员权限下发。
func (h *Handler) IssuePersonAccess(c *gin.Context) {
	var req struct {
		CommunityID jsonx.Uint64 `json:"communityId" binding:"required"`
		PersonType  int          `json:"personType" binding:"required,oneof=1 2 3 4"`
		PersonID    jsonx.Uint64 `json:"personId" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	n, err := h.Svc.IssuePersonAccess(c.Request.Context(), req.CommunityID.Uint64(), req.PersonType, req.PersonID.Uint64(), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, gin.H{"grants": n})
}

// ListAccessCards 门禁卡列表。
func (h *Handler) ListAccessCards(c *gin.Context) {
	cid := queryID(c, "communityId")
	var pt *int
	if v := c.Query("personType"); v != "" {
		n := int(queryID(c, "personType"))
		pt = &n
	}
	list, err := h.Svc.ListAccessCards(c.Request.Context(), cid, pt, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// CreateAccessCard 登记门禁卡。
func (h *Handler) CreateAccessCard(c *gin.Context) {
	var req service.AccessCardReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateAccessCard(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// DeleteAccessCard 注销门禁卡。
func (h *Handler) DeleteAccessCard(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteAccessCard(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}
