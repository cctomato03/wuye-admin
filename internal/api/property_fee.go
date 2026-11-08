package api

import (
	"github.com/gin-gonic/gin"

	"web-basic/internal/principal"
	"web-basic/internal/service"
	"web-basic/pkg/response"
)

// ============================ 收费项目 ============================

// ListFeeItems 收费项目列表。
func (h *Handler) ListFeeItems(c *gin.Context) {
	var q service.FeeItemQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListFeeItems(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// CreateFeeItem 新增收费项目。
func (h *Handler) CreateFeeItem(c *gin.Context) {
	var req service.FeeItemReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateFeeItem(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// UpdateFeeItem 修改收费项目。
func (h *Handler) UpdateFeeItem(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.FeeItemReq
	if !bindJSON(c, &req) {
		return
	}
	if err := h.Svc.UpdateFeeItem(c.Request.Context(), id, &req, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// DeleteFeeItem 删除收费项目。
func (h *Handler) DeleteFeeItem(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteFeeItem(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ListFeeStandards 收费标准列表。
func (h *Handler) ListFeeStandards(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	list, err := h.Svc.ListFeeStandards(c.Request.Context(), id, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// CreateFeeStandard 新增收费标准。
func (h *Handler) CreateFeeStandard(c *gin.Context) {
	var req service.FeeStandardReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateFeeStandard(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// DeleteFeeStandard 删除收费标准。
func (h *Handler) DeleteFeeStandard(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteFeeStandard(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// ============================ 水电表 / 抄表 ============================

// ListMeters 水电表列表。
func (h *Handler) ListMeters(c *gin.Context) {
	var q service.MeterQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListMeters(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// CreateMeter 新增水电表。
func (h *Handler) CreateMeter(c *gin.Context) {
	var req service.MeterReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateMeter(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// DeleteMeter 删除水电表。
func (h *Handler) DeleteMeter(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteMeter(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// CreateMeterReading 录入抄表。
func (h *Handler) CreateMeterReading(c *gin.Context) {
	var req service.MeterReadingReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.CreateMeterReading(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// ListMeterReadings 抄表记录列表。
func (h *Handler) ListMeterReadings(c *gin.Context) {
	var q service.MeterReadingQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListMeterReadings(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// ============================ 账单 ============================

// GenerateBills 生成账单。
func (h *Handler) GenerateBills(c *gin.Context) {
	var req service.GenerateBillReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.Svc.GenerateBills(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, res)
}

// GenerateMeterBills 按抄表生成账单。
func (h *Handler) GenerateMeterBills(c *gin.Context) {
	var req service.GenerateMeterBillsReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.Svc.GenerateMeterBills(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, res)
}

// ListBills 账单列表。
func (h *Handler) ListBills(c *gin.Context) {
	var q service.BillQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListBills(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// GetBill 账单详情。
func (h *Handler) GetBill(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	vo, err := h.Svc.GetBill(c.Request.Context(), id, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, vo)
}

// PayBill 收费。
func (h *Handler) PayBill(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req service.PayBillReq
	if !bindJSON(c, &req) {
		return
	}
	res, err := h.Svc.PayBill(c.Request.Context(), id, &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, res)
}

// CancelBill 作废账单。
func (h *Handler) CancelBill(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.Svc.CancelBill(c.Request.Context(), id, principal.Must(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.Ok(c)
}

// BillSummary 收费汇总。
func (h *Handler) BillSummary(c *gin.Context) {
	cid := queryID(c, "communityId")
	month := c.Query("billMonth")
	sum, err := h.Svc.BillSummaryOf(c.Request.Context(), cid, month, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, sum)
}

// ============================ 缴费记录 ============================

// ListPayments 缴费记录列表。
func (h *Handler) ListPayments(c *gin.Context) {
	var q service.PaymentQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListPayments(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// ============================ 账户 ============================

// ListAccounts 账户列表。
func (h *Handler) ListAccounts(c *gin.Context) {
	cid := queryID(c, "communityId")
	list, err := h.Svc.ListAccounts(c.Request.Context(), cid, c.Query("keyword"), principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkData(c, list)
}

// RechargeAccount 账户充值 / 押金缴纳。
func (h *Handler) RechargeAccount(c *gin.Context) {
	var req service.RechargeReq
	if !bindJSON(c, &req) {
		return
	}
	id, err := h.Svc.RechargeAccount(c.Request.Context(), &req, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OkID(c, id)
}

// ListAccountLogs 账户流水。
func (h *Handler) ListAccountLogs(c *gin.Context) {
	var q service.AccountLogQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, response.BindErr(err))
		return
	}
	list, total, err := h.Svc.ListAccountLogs(c.Request.Context(), &q, principal.Must(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}
