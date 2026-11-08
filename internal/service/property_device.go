// 设备管理：门禁机 / 车辆道闸 / 视频监控 / 人脸终端 / 可视对讲，支持绑定到五级层级。
package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gorm.io/gorm"

	"web-basic/internal/excel"
	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
	"web-basic/pkg/jsonx"
)

// DeviceQuery 设备查询。
type DeviceQuery struct {
	entity.PageQuery
	CommunityID jsonx.Uint64 `form:"communityId"`
	Keyword     string       `form:"keyword"`
	DeviceType  *int         `form:"deviceType"`
	BindLevel   *int         `form:"bindLevel"`
	BindID      jsonx.Uint64 `form:"bindId"`
	Status      *int         `form:"status"`
	AccessOnly  *bool        `form:"accessOnly"`
}

// ListDevices 设备分页列表。
func (s *Service) ListDevices(ctx context.Context, q *DeviceQuery, p *principal.Info) ([]entity.Device, int64, error) {
	q.Normalize()
	if q.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, q.CommunityID.Uint64()); err != nil {
			return nil, 0, err
		}
	}
	tx := s.scoped(ctx, &entity.Device{}, p, "")
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(code LIKE ? OR name LIKE ? OR serial_no LIKE ? OR location LIKE ?)", kw+"%", kw+"%", kw+"%", kw+"%")
	}
	if q.DeviceType != nil {
		tx = tx.Where("device_type = ?", *q.DeviceType)
	}
	if q.BindLevel != nil {
		tx = tx.Where("bind_level = ?", *q.BindLevel)
	}
	if q.BindID.Uint64() != 0 {
		tx = tx.Where("bind_id = ?", q.BindID.Uint64())
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	if q.AccessOnly != nil && *q.AccessOnly {
		tx = tx.Where("access_enabled = 1 AND status = 1")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Device, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// DeviceReq 设备入参。
type DeviceReq struct {
	CommunityID   jsonx.Uint64 `json:"communityId" binding:"required"`
	Code          string       `json:"code" binding:"required,max=64"`
	Name          string       `json:"name" binding:"required,max=100"`
	DeviceType    int          `json:"deviceType"`
	Brand         string       `json:"brand" binding:"max=64"`
	Model         string       `json:"model" binding:"max=64"`
	SerialNo      string       `json:"serialNo" binding:"max=100"`
	IP            string       `json:"ip" binding:"max=64"`
	MAC           string       `json:"mac" binding:"max=32"`
	Location      string       `json:"location" binding:"max=200"`
	BindLevel     int          `json:"bindLevel"`
	BindID        jsonx.Uint64 `json:"bindId"`
	BindNodeID    jsonx.Uint64 `json:"bindNodeId"`
	AccessEnabled bool         `json:"accessEnabled"`
	InstallTime   string       `json:"installTime"`
	Status        int          `json:"status"`
	Remark        string       `json:"remark" binding:"max=500"`
}

// resolveBindName 校验绑定层级与对象，返回绑定对象名称。
func (s *Service) resolveBindName(ctx context.Context, communityID uint64, level int, bindID uint64) (string, error) {
	if level <= 0 {
		level = 1
	}
	if level > 5 {
		return "", errs.Invalid("绑定层级取值应为 1~5")
	}
	if level == 1 || bindID == 0 {
		return "", nil
	}
	if level < 2 {
		return "", nil
	}
	n, err := s.nodeBelongsTo(ctx, communityID, bindID)
	if err != nil {
		return "", err
	}
	return n.Name, nil
}

// CreateDevice 新增设备。
func (s *Service) CreateDevice(ctx context.Context, req *DeviceReq, p *principal.Info) (uint64, error) {
	if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
		return 0, err
	}
	com, err := s.findCommunity(ctx, p, req.CommunityID.Uint64())
	if err != nil {
		return 0, err
	}
	bindName, err := s.resolveDeviceBind(ctx, com.ID, req)
	if err != nil {
		return 0, err
	}
	install, err := parseDate(req.InstallTime)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	d := &entity.Device{
		Base:          entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:      com.TenantID,
		CommunityID:   com.ID,
		Code:          strings.TrimSpace(req.Code),
		Name:          strings.TrimSpace(req.Name),
		DeviceType:    pmDefault(req.DeviceType, 1),
		Brand:         req.Brand,
		Model:         req.Model,
		SerialNo:      req.SerialNo,
		IP:            req.IP,
		MAC:           req.MAC,
		Location:      req.Location,
		BindLevel:     pmDefault(req.BindLevel, 1),
		BindID:        req.BindID.Uint64(),
		BindName:      bindName,
		BindNodeID:    req.BindNodeID.Uint64(),
		AccessEnabled: req.AccessEnabled,
		InstallTime:   install,
		Status:        pmDefault(req.Status, 1),
	}
	if err := s.Repo.DB.WithContext(ctx).Create(d).Error; err != nil {
		return 0, wrapTxErr(err)
	}
	return d.ID, nil
}

// UpdateDevice 修改设备。
func (s *Service) UpdateDevice(ctx context.Context, deviceID uint64, req *DeviceReq, p *principal.Info) error {
	d, err := s.deviceScoped(ctx, p, deviceID)
	if err != nil {
		return err
	}
	bindName, err := s.resolveDeviceBind(ctx, d.CommunityID, req)
	if err != nil {
		return err
	}
	install, err := parseDate(req.InstallTime)
	if err != nil {
		return err
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Device{}).Where("id = ?", d.ID).Updates(map[string]any{
		"code":           strings.TrimSpace(req.Code),
		"name":           strings.TrimSpace(req.Name),
		"device_type":    pmDefault(req.DeviceType, 1),
		"brand":          req.Brand,
		"model":          req.Model,
		"serial_no":      req.SerialNo,
		"ip":             req.IP,
		"mac":            req.MAC,
		"location":       req.Location,
		"bind_level":     pmDefault(req.BindLevel, 1),
		"bind_id":        req.BindID.Uint64(),
		"bind_name":      bindName,
		"bind_node_id":   req.BindNodeID.Uint64(),
		"access_enabled": req.AccessEnabled,
		"install_time":   install,
		"status":         pmDefault(req.Status, 1),
		"remark":         req.Remark,
		"update_by":      p.UserID,
		"update_time":    time.Now(),
	}).Error; err != nil {
		return wrapTxErr(err)
	}
	return nil
}

// DeleteDevice 删除设备：作废其授权明细。
func (s *Service) DeleteDevice(ctx context.Context, deviceID uint64, p *principal.Info) error {
	d, err := s.deviceScoped(ctx, p, deviceID)
	if err != nil {
		return err
	}
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.Device{}, d.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.AccessGrant{}).
		Where("device_id = ? AND status IN ?", d.ID, []int{GrantStatusPending, GrantStatusIssuing, GrantStatusSuccess, GrantStatusFailed}).
		Updates(map[string]any{"status": GrantStatusRevoked, "revoked_time": time.Now()}).Error
	return nil
}

// deviceScoped 带作用域读取设备。
func (s *Service) deviceScoped(ctx context.Context, p *principal.Info, deviceID uint64) (*entity.Device, error) {
	var d entity.Device
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Device{}).Where("id = ?", deviceID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if !all {
		if len(ids) == 0 {
			return nil, errs.ErrNotFound
		}
		tx = tx.Where("community_id IN ?", ids)
	}
	if err := tx.First(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &d, nil
}

// DeviceOptions 门禁设备下拉（其他人员登记时勾选）。
func (s *Service) DeviceOptions(ctx context.Context, communityID uint64, p *principal.Info) ([]OptionVO, error) {
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	var list []entity.Device
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Device{}).
		Where("community_id = ? AND status = 1 AND access_enabled = 1", communityID).
		Order("device_type ASC, id ASC").Limit(1000).Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	out := make([]OptionVO, 0, len(list))
	for _, d := range list {
		out = append(out, OptionVO{ID: d.ID, Name: d.Name, Code: d.Code, Extra: d.Location})
	}
	return out, nil
}

// DeviceImportTemplate 设备导入模板。
func (s *Service) DeviceImportTemplate() ([]byte, error) {
	headers := []string{"设备编码", "设备名称", "设备类型", "品牌", "型号", "序列号", "IP", "MAC", "安装位置", "绑定层级", "绑定对象编码", "状态"}
	rows := [][]any{
		{"D-001", "东门人脸门禁", "门禁机", "海康", "DS-K1T", "SN2026001", "192.168.1.10", "", "小区东门", "小区", "", "正常"},
		{"D-002", "1栋1单元门禁", "门禁机", "海康", "DS-K1T", "SN2026002", "192.168.1.11", "", "1栋1单元门口", "楼栋", "B01", "正常"},
		{"D-003", "南门车辆道闸", "车辆道闸", "捷顺", "JS-800", "SN2026003", "192.168.1.20", "", "小区南门", "小区", "", "正常"},
	}
	data, err := excel.Build("设备导入模板", headers, rows)
	if err != nil {
		return nil, errs.ErrInternal.WithErr(err)
	}
	return data, nil
}

// DeviceImportResult 设备导入结果。
type DeviceImportResult struct {
	Total   int      `json:"total"`
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors"`
}

// ImportDevices 导入设备（按设备编码去重）。
func (s *Service) ImportDevices(ctx context.Context, communityID uint64, r io.Reader, p *principal.Info) (*DeviceImportResult, error) {
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	if _, err := s.findCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	rows, err := excel.Read(r, 5000)
	if err != nil {
		return nil, errs.Invalid("文件解析失败：请确认上传的是 .xlsx 格式的 Excel 文件")
	}
	if len(rows) <= 1 {
		return nil, errs.Invalid("文件没有数据行")
	}
	// 楼栋/区域/单元/楼层编码映射，便于导入时按编码绑定
	buildingMap := map[string]uint64{}
	regionMap := map[string]uint64{}
	unitMap := map[string]uint64{}
	{
		var bs []entity.Building
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.Building{}).Select("id, code").Where("community_id = ?", communityID).Find(&bs).Error
		for _, b := range bs {
			buildingMap[b.Code] = b.ID
		}
		var rs []entity.Region
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.Region{}).Select("id, code").Where("community_id = ?", communityID).Find(&rs).Error
		for _, rg := range rs {
			regionMap[rg.Code] = rg.ID
		}
		var us []entity.Unit
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.Unit{}).Select("id, code").Where("community_id = ?", communityID).Find(&us).Error
		for _, u := range us {
			unitMap[u.Code] = u.ID
		}
	}
	res := &DeviceImportResult{Errors: []string{}}
	cell := func(row []string, i int) string {
		if i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	for i, row := range rows[1:] {
		lineNo := i + 2
		res.Total++
		code := cell(row, 0)
		if code == "" {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：设备编码为空", lineNo))
			continue
		}
		var exist int64
		if err := s.Repo.DB.WithContext(ctx).Model(&entity.Device{}).Where("community_id = ? AND code = ?", communityID, code).Count(&exist).Error; err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：%s", lineNo, err.Error()))
			continue
		}
		if exist > 0 {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：设备编码 %s 已存在", lineNo, code))
			continue
		}
		level := parseBindLevel(cell(row, 9))
		bindCode := cell(row, 10)
		var bindID uint64
		switch level {
		case 2:
			bindID = regionMap[bindCode]
		case 3:
			bindID = buildingMap[bindCode]
		case 4:
			bindID = unitMap[bindCode]
		}
		if level > 1 && bindID == 0 && bindCode != "" {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：绑定对象编码 %s 不存在", lineNo, bindCode))
			continue
		}
		if level > 1 && bindCode == "" {
			level = 1 // 未填绑定对象时退化为小区级
		}
		req := &DeviceReq{
			CommunityID: jsonx.Uint64(communityID),
			Code:        code,
			Name:        defaultIfEmpty(cell(row, 1), code),
			DeviceType:  parseDeviceType(cell(row, 2)),
			Brand:       cell(row, 3),
			Model:       cell(row, 4),
			SerialNo:    cell(row, 5),
			IP:          cell(row, 6),
			MAC:         cell(row, 7),
			Location:    cell(row, 8),
			BindLevel:   level,
			BindID:      jsonx.Uint64(bindID),
			Status:      parseDeviceStatus(cell(row, 11)),
		}
		req.AccessEnabled = req.DeviceType == 1 || req.DeviceType == 4
		if _, err := s.CreateDevice(ctx, req, p); err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行：%s", lineNo, fmtErr(err)))
			continue
		}
		res.Success++
	}
	return res, nil
}

func parseDeviceType(v string) int {
	switch strings.TrimSpace(v) {
	case "车辆道闸", "道闸", "2":
		return 2
	case "视频监控", "监控", "3":
		return 3
	case "人脸终端", "人脸", "4":
		return 4
	case "可视对讲", "5":
		return 5
	case "门禁机", "门禁", "1":
		return 1
	default:
		return 9
	}
}

func parseBindLevel(v string) int {
	switch strings.TrimSpace(v) {
	case "区域", "2":
		return 2
	case "楼栋", "3":
		return 3
	case "单元", "4":
		return 4
	case "楼层", "5":
		return 5
	default:
		return 1
	}
}

func parseDeviceStatus(v string) int {
	switch strings.TrimSpace(v) {
	case "停用", "2":
		return 2
	case "故障", "3":
		return 3
	default:
		return 1
	}
}

// DeviceStats 设备统计。
type DeviceStats struct {
	Total    int64 `json:"total"`
	Access   int64 `json:"access"`
	Gate     int64 `json:"gate"`
	Camera   int64 `json:"camera"`
	Online   int64 `json:"online"`
	Offline  int64 `json:"offline"`
	Maintain int64 `json:"maintain"`
}

// DeviceSummary 小区设备概览。
func (s *Service) DeviceSummary(ctx context.Context, communityID uint64, p *principal.Info) (*DeviceStats, error) {
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	var rows []struct {
		DeviceType int
		Online     int
		Status     int
		Count      int64
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Device{}).
		Select("device_type, online, status, COUNT(*) AS count").
		Where("community_id = ?", communityID).
		Group("device_type, online, status").Scan(&rows).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	st := &DeviceStats{}
	for _, r := range rows {
		st.Total += r.Count
		switch r.DeviceType {
		case 1, 4:
			st.Access += r.Count
		case 2:
			st.Gate += r.Count
		case 3:
			st.Camera += r.Count
		}
		if r.Online == 1 {
			st.Online += r.Count
		} else {
			st.Offline += r.Count
		}
		if r.Status == 3 {
			st.Maintain += r.Count
		}
	}
	return st, nil
}

// resolveDeviceBind 解析设备绑定位置：优先自定义层级节点（bindNodeId），
// 其次兼容旧接口的 bindLevel/bindId（2~5 级在老数据里就是节点 ID）。
func (s *Service) resolveDeviceBind(ctx context.Context, communityID uint64, req *DeviceReq) (string, error) {
	if req.BindNodeID.Uint64() != 0 {
		n, err := s.nodeBelongsTo(ctx, communityID, req.BindNodeID.Uint64())
		if err != nil {
			return "", err
		}
		return n.Name, nil
	}
	if req.BindLevel <= 1 || req.BindID.Uint64() == 0 {
		return "", nil
	}
	return s.resolveBindName(ctx, communityID, req.BindLevel, req.BindID.Uint64())
}
