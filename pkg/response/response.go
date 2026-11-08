// Package response 统一 HTTP 响应封装。
package response

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"web-basic/internal/metrics"
	"web-basic/pkg/errs"
)

// Body 标准响应体：code=0 表示成功。
type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	TraceID string `json:"traceId"`
}

// PageData 分页数据结构。
type PageData struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

func traceID(c *gin.Context) string {
	if v, ok := c.Get("traceId"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Ok 返回成功（无数据）。
func Ok(c *gin.Context) { OkData(c, nil) }

// OkData 返回成功（带数据）。
func OkData(c *gin.Context, data any) {
	metrics.Default.IncCode("0")
	c.JSON(http.StatusOK, Body{Code: 0, Message: "success", Data: data, TraceID: traceID(c)})
}

// OkID 返回新增记录的雪花 ID。
// 注意：ID 必须以字符串输出，否则 JS Number 超过 2^53 会精度丢失（新增后按 ID 编辑会找不到数据）。
func OkID(c *gin.Context, id uint64) {
	OkData(c, gin.H{"id": strconv.FormatUint(id, 10)})
}

// Page 返回分页成功。
func Page(c *gin.Context, list any, total int64, page, pageSize int) {
	OkData(c, PageData{List: list, Total: total, Page: page, PageSize: pageSize})
}

// FailCode 返回指定错误码。
func FailCode(c *gin.Context, code int, msg string) {
	metrics.Default.IncCode(strconv.Itoa(code))
	c.JSON(httpStatus(code), Body{Code: code, Message: msg, TraceID: traceID(c)})
}

// Fail 返回业务错误；非 AppError 一律按系统内部错误处理，避免泄露内部细节。
func Fail(c *gin.Context, err error) {
	// 统一归一化：唯一键冲突 → 10005/409，记录不存在 → 10004/404
	err = errs.Normalize(err)
	// 写入 gin 错误链：操作日志中间件据此把本次操作记为"失败"并落错误信息
	_ = c.Error(err)
	var ae *errs.AppError
	if errors.As(err, &ae) {
		FailCode(c, ae.Code, ae.Msg)
		return
	}
	FailCode(c, errs.ErrInternal.Code, errs.ErrInternal.Msg)
}

// BindErr 参数绑定错误 → 统一业务错误。
func BindErr(err error) error { return errs.Invalid(bindMessage(err)) }

// bindMessage 把参数绑定/校验错误转成「用户看得懂」的提示。
//
// 硬约定：**任何情况下都不能把框架原始错误文本返回给用户**
// （形如 "Key: 'CompanyReq.AdminPassword' Error:Field validation for 'AdminPassword' failed on the 'min' tag"
// 既暴露结构体名又看不懂）。这里统一翻译成「字段中文名 + 具体怎么改」。
func bindMessage(err error) string {
	if err == nil {
		return errs.ErrBadRequest.Msg
	}
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) && len(verrs) > 0 {
		// 多个字段同时不合法时，提示里带上数量，避免用户改一个再报一个
		first := verrs[0]
		label := fieldLabel(first)
		msg := translateTag(first, label)
		if len(verrs) > 1 {
			msg += fmt.Sprintf("（共 %d 处需要修改）", len(verrs))
		}
		return msg
	}
	// 类型转换 / JSON 结构错误：只给结论，不给 SQL、结构体、字段路径
	raw := strings.ToLower(err.Error())
	switch {
	case strings.Contains(raw, "cannot unmarshal"),
		strings.Contains(raw, "unexpected end of json"),
		strings.Contains(raw, "invalid character"),
		strings.Contains(raw, "unexpected eof"):
		return "请求参数格式不正确（不是合法的 JSON，或字段类型不匹配）"
	case strings.Contains(raw, "is not valid"),
		strings.Contains(raw, "cannot parse"),
		strings.Contains(raw, "syntax error"):
		return "请求参数格式不正确，请检查填写的数字/日期格式"
	case strings.Contains(raw, "multipart"),
		strings.Contains(raw, "form file"):
		return "文件上传格式不正确，请重新选择文件"
	case strings.Contains(raw, "message type"):
		return "请求参数格式不正确"
	default:
		return errs.ErrBadRequest.Msg
	}
}

// fieldLabel 取字段中文名：优先用标签名映射，其次读结构体的 label/form 标签，最后回落字段名。
func fieldLabel(fe validator.FieldError) string {
	if label, ok := fieldLabels[fe.Field()]; ok {
		return label
	}
	// JSON 名（如 AdminPassword → adminPassword）也允许登记
	if label, ok := fieldLabels[strings.ToLower(fe.Field())]; ok {
		return label
	}
	return fe.Field()
}

// translateTag 把 validator 规则翻译成可执行的中文提示。
func translateTag(fe validator.FieldError, label string) string {
	param := fe.Param()
	switch fe.Tag() {
	case "required", "required_if", "required_with":
		return fmt.Sprintf("请填写「%s」", label)
	case "min":
		return fmt.Sprintf("「%s」不能少于 %s 个字符", label, param)
	case "max":
		return fmt.Sprintf("「%s」不能超过 %s 个字符", label, param)
	case "len":
		return fmt.Sprintf("「%s」长度必须为 %s 个字符", label, param)
	case "email":
		return fmt.Sprintf("「%s」不是合法的邮箱地址", label)
	case "url":
		return fmt.Sprintf("「%s」不是合法的网址", label)
	case "oneof":
		return fmt.Sprintf("「%s」取值不合法，可选值：%s", label, strings.ReplaceAll(param, " ", " / "))
	case "numeric", "number":
		return fmt.Sprintf("「%s」必须是数字", label)
	case "gte":
		return fmt.Sprintf("「%s」不能小于 %s", label, param)
	case "gt":
		return fmt.Sprintf("「%s」必须大于 %s", label, param)
	case "lte":
		return fmt.Sprintf("「%s」不能大于 %s", label, param)
	case "lt":
		return fmt.Sprintf("「%s」必须小于 %s", label, param)
	case "eq":
		return fmt.Sprintf("「%s」必须等于 %s", label, param)
	case "ne":
		return fmt.Sprintf("「%s」不能等于 %s", label, param)
	case "boolean":
		return fmt.Sprintf("「%s」必须是 true 或 false", label)
	case "alphanum":
		return fmt.Sprintf("「%s」只能包含字母和数字", label)
	case "dive", "structonly":
		return fmt.Sprintf("「%s」填写不正确", label)
	default:
		return fmt.Sprintf("「%s」填写不正确，请检查格式", label)
	}
}

// fieldLabels 结构体字段 → 中文名（未登记的回落到字段名本身）。
// 覆盖：脚手架通用字段 + 物业管理模块全部入参字段。
var fieldLabels = map[string]string{
	// 通用 / 认证
	"Username": "账号", "Password": "密码", "OldPassword": "当前密码", "NewPassword": "新密码",
	"Nickname": "昵称", "Email": "邮箱", "Mobile": "手机号", "Name": "名称", "Title": "标题",
	"Content": "内容", "Cron": "cron 表达式", "DeptID": "部门", "RoleIDs": "角色", "Ids": "ID 列表",
	"SiteName": "网站名称", "LoginDesc": "登录页描述", "LoginSubTitle": "登录页副标题",
	"Page": "页码", "PageSize": "每页条数", "Status": "状态", "Remark": "备注",
	"TenantID": "所属物业公司", "CaptchaCode": "验证码", "CaptchaID": "验证码标识",
	// 小区 / 物业公司
	"TenantCode": "物业公司编码", "CreditCode": "统一社会信用代码", "LegalPerson": "法人",
	"ContactName": "联系人", "ContactPhone": "联系电话", "AdminUsername": "管理员账号",
	"AdminPassword": "管理员密码", "ServiceScope": "服务范围", "LicenseNo": "资质证书号",
	"MaxUsers": "用户数上限", "ExpireAt": "到期时间", "Code": "编码", "ShortName": "简称",
	"CommunityType": "小区类型", "Province": "省", "City": "市", "District": "区县",
	"DistrictCode": "行政区划代码", "Address": "地址", "ManagerName": "物业负责人",
	"ManagerPhone": "负责人电话", "ServicePhone": "客服电话", "HandoverDate": "交付日期",
	// 楼宇结构
	"CommunityID": "小区", "RegionID": "区域",
	"BuildingID": "楼栋", "UnitID": "单元", "FloorID": "楼层", "FloorNo": "楼层号",
	"HouseTypeID": "房屋类型", "HouseNo": "房号", "UsageType": "房屋用途", "AreaBuild": "建筑面积",
	"AreaUse": "套内面积", "HouseStatus": "房屋状态", "BuildingType": "楼栋类型", "TotalFloors": "总层数",
	"HousesPerFloor": "每层户数", "FloorFrom": "起始楼层", "FloorTo": "结束楼层",
	// 人员
	"IDNo": "证件号码", "IDType": "证件类型", "Gender": "性别", "Birthday": "出生日期",
	"HouseID": "房屋", "RelRole": "身份", "EndDate": "到期日期", "StartDate": "开始日期",
	"VisitHouseID": "受访房屋", "VisitType": "来访类型", "PeopleCount": "来访人数",
	"ValidTo": "有效期结束", "ValidFrom": "有效期开始", "CategoryID": "人员类别",
	"PersonType": "人员类型", "PersonID": "人员", "CardNo": "卡号", "DeviceIDs": "门禁设备",
	// 车辆 / 单位
	"PlateNo": "车牌号", "VehicleType": "车辆类型", "ParkingSpaceID": "车位",
	"MerchantType": "单位类型", "BusinessLicense": "营业执照号",
	// 收费
	"FeeItemID": "收费项目", "CalcMode": "计费方式", "UnitPrice": "单价", "BillMonth": "账期",
	"Period": "账期", "PayMethod": "支付方式", "Amount": "金额", "CurrReading": "本期读数",
	"MeterID": "水表", "MeterNo": "表号", "OwnerType": "账户类型", "TargetType": "充值类型",
	// 设备 / 门禁
	"DeviceType": "设备类型", "BindLevel": "绑定层级", "BindID": "绑定对象", "SerialNo": "序列号",
	"GrantType": "下发类型",
}

// httpStatus 业务码 → HTTP 状态码。
func httpStatus(code int) int {
	switch {
	case code == 0:
		return http.StatusOK
	case code == errs.ErrNotFound.Code:
		return http.StatusNotFound
	case code == errs.ErrDuplicate.Code:
		return http.StatusConflict
	case code >= 20000 && code < 30000:
		return http.StatusUnauthorized
	case code >= 30000 && code < 40000:
		return http.StatusForbidden
	case code >= 50000 && code < 60000:
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}
