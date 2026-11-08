// 小区管理 + 物业公司（租户）管理。
//
// 层级关系：sys_tenant（物业公司）1 — n pm_community（小区）；小区只能归属一个物业公司。
// 平台超管（is_super）可以跨租户建小区并指定归属；物业公司管理员只能在本租户内建/改。
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/pkg/crypto"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
	"web-basic/pkg/jsonx"
)

// CommunityQuery 小区列表查询。
type CommunityQuery struct {
	entity.PageQuery
	Keyword       string `form:"keyword"`      // 名称/编码前缀
	DistrictCode  string `form:"districtCode"` // 行政区划
	Name          string `form:"name"`
	Code          string `form:"code"`
	Status        *int   `form:"status"`
	TenantID      uint64 `form:"tenantId"` // 平台侧按物业公司筛选
	CommunityType *int   `form:"communityType"`
}

// CommunityVO 小区出参（附带所属物业公司名称）。
type CommunityVO struct {
	entity.Community
	TenantName string `json:"tenantName"`
}

// ListCommunities 小区分页列表（自动按租户 + 授权小区过滤）。
func (s *Service) ListCommunities(ctx context.Context, q *CommunityQuery, p *principal.Info) ([]CommunityVO, int64, error) {
	q.Normalize()
	tx := s.communityTableScope(ctx, p)
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(name LIKE ? OR code LIKE ?)", kw+"%", kw+"%")
	}
	if q.Name != "" {
		tx = tx.Where("name LIKE ?", likePrefix(q.Name))
	}
	if q.Code != "" {
		tx = tx.Where("code LIKE ?", likePrefix(q.Code))
	}
	if q.DistrictCode != "" {
		tx = tx.Where("district_code = ?", q.DistrictCode)
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	if q.CommunityType != nil {
		tx = tx.Where("community_type = ?", *q.CommunityType)
	}
	// 平台侧（未按租户过滤）才允许用 tenantId 精确筛选
	if q.TenantID > 0 && s.tenantIDOf(p) == 0 {
		tx = tx.Where("tenant_id = ?", q.TenantID)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Community, 0, q.PageSize)
	if err := tx.Order("sort ASC, id ASC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	names, _ := s.tenantNameMap(ctx)
	out := make([]CommunityVO, 0, len(list))
	for _, c := range list {
		out = append(out, CommunityVO{Community: c, TenantName: names[c.TenantID]})
	}
	return out, total, nil
}

// tenantNameMap 租户 ID → 名称。
func (s *Service) tenantNameMap(ctx context.Context) (map[uint64]string, error) {
	var rows []struct {
		ID   uint64
		Name string
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Tenant{}).Select("id, name").Scan(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[uint64]string, len(rows))
	for _, r := range rows {
		m[r.ID] = r.Name
	}
	return m, nil
}

// GetCommunity 小区详情。
func (s *Service) GetCommunity(ctx context.Context, communityID uint64, p *principal.Info) (*CommunityVO, error) {
	c, err := s.findCommunity(ctx, p, communityID)
	if err != nil {
		return nil, err
	}
	names, _ := s.tenantNameMap(ctx)
	return &CommunityVO{Community: *c, TenantName: names[c.TenantID]}, nil
}

// CommunityReq 小区新增/修改入参。
type CommunityReq struct {
	TenantID        jsonx.Uint64 `json:"tenantId"`
	Code            string       `json:"code" binding:"required,max=64"`
	Name            string       `json:"name" binding:"required,max=100"`
	ShortName       string       `json:"shortName" binding:"max=64"`
	CommunityType   int          `json:"communityType" binding:"omitempty,oneof=1 2 3 4 5"`
	Province        string       `json:"province" binding:"max=64"`
	City            string       `json:"city" binding:"max=64"`
	District        string       `json:"district" binding:"max=64"`
	Town            string       `json:"town" binding:"max=64"`
	Village         string       `json:"village" binding:"max=64"`
	DistrictCode    string       `json:"districtCode" binding:"max=20"`
	StreetCode      string       `json:"streetCode" binding:"max=20"`
	VillageCode     string       `json:"villageCode" binding:"max=20"`
	Address         string       `json:"address" binding:"max=255"`
	Longitude       float64      `json:"longitude"`
	Latitude        float64      `json:"latitude"`
	TotalArea       float64      `json:"totalArea"`
	GreenArea       float64      `json:"greenArea"`
	Developer       string       `json:"developer" binding:"max=100"`
	PropertyCompany string       `json:"propertyCompany" binding:"max=100"`
	ManagerName     string       `json:"managerName" binding:"max=64"`
	ManagerPhone    string       `json:"managerPhone" binding:"max=20"`
	ServicePhone    string       `json:"servicePhone" binding:"max=20"`
	HandoverDate    string       `json:"handoverDate"`
	Status          int          `json:"status"`
	Sort            int          `json:"sort"`
	Remark          string       `json:"remark" binding:"max=500"`
}

// CreateCommunity 新增小区。
func (s *Service) CreateCommunity(ctx context.Context, req *CommunityReq, p *principal.Info) (uint64, error) {
	tenantID := uint64(req.TenantID)
	if tenantID == 0 {
		tenantID = p.TenantID
	}
	if tenantID == 0 {
		return 0, errs.Invalid("请选择所属物业公司")
	}
	// 非平台账号只能在自己租户下建小区
	if !p.IsSuper && tenantID != p.TenantID {
		return 0, errs.ErrDataForbidden
	}
	// 校验物业公司存在（并拿到公司名做冗余展示）
	var tenant entity.Tenant
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", tenantID).First(&tenant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, errs.Invalid("物业公司不存在")
		}
		return 0, errs.ErrDB.WithErr(err)
	}
	code := strings.TrimSpace(req.Code)
	var dup int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Community{}).
		Where("tenant_id = ? AND code = ?", tenantID, code).Count(&dup).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	if dup > 0 {
		return 0, errs.Invalid("该物业公司下已存在相同编码的小区")
	}
	handover, err := parseDate(req.HandoverDate)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	c := &entity.Community{
		Base:            entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		TenantID:        tenantID,
		Code:            code,
		Name:            strings.TrimSpace(req.Name),
		ShortName:       req.ShortName,
		CommunityType:   pmDefault(req.CommunityType, 1),
		Province:        req.Province,
		City:            req.City,
		District:        req.District,
		Town:            req.Town,
		Village:         req.Village,
		DistrictCode:    req.DistrictCode,
		StreetCode:      req.StreetCode,
		VillageCode:     req.VillageCode,
		Address:         req.Address,
		Longitude:       req.Longitude,
		Latitude:        req.Latitude,
		TotalArea:       req.TotalArea,
		GreenArea:       req.GreenArea,
		Developer:       req.Developer,
		PropertyCompany: pmDefaultStr(req.PropertyCompany, tenant.Name),
		ManagerName:     req.ManagerName,
		ManagerPhone:    req.ManagerPhone,
		ServicePhone:    req.ServicePhone,
		HandoverDate:    handover,
		Status:          pmDefault(req.Status, 1),
		Sort:            req.Sort,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(c).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	// 创建人自动获得该小区授权，避免"建完小区自己看不到"
	if p.UserID != 0 {
		uc := &entity.UserCommunity{
			Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now},
			TenantID:    tenantID,
			UserID:      p.UserID,
			CommunityID: c.ID,
			IsDefault:   p.DefaultCommunityID == 0,
		}
		if err := s.Repo.DB.WithContext(ctx).Create(uc).Error; err != nil {
			// 授权失败不影响小区创建，但要让用户知道
			return c.ID, nil
		}
		s.invalidatePrincipal(ctx, p.UserID)
	}
	return c.ID, nil
}

// UpdateCommunity 修改小区。
func (s *Service) UpdateCommunity(ctx context.Context, communityID uint64, req *CommunityReq, p *principal.Info) error {
	c, err := s.findCommunity(ctx, p, communityID)
	if err != nil {
		return err
	}
	// 更换归属物业公司：仅平台超管可操作，且小区必须还没有业务数据
	if req.TenantID.Uint64() != 0 && req.TenantID.Uint64() != c.TenantID {
		if !p.IsSuper {
			return errs.ErrDataForbidden
		}
		var n int64
		if err := s.Repo.DB.WithContext(ctx).Model(&entity.House{}).Where("community_id = ?", communityID).Count(&n).Error; err != nil {
			return errs.ErrDB.WithErr(err)
		}
		if n > 0 {
			return errs.Invalid("该小区已有房屋数据，不能更换物业公司")
		}
	}
	handover, err := parseDate(req.HandoverDate)
	if err != nil {
		return err
	}
	fields := map[string]any{
		"code":             strings.TrimSpace(req.Code),
		"name":             strings.TrimSpace(req.Name),
		"short_name":       req.ShortName,
		"community_type":   pmDefault(req.CommunityType, 1),
		"province":         req.Province,
		"city":             req.City,
		"district":         req.District,
		"town":             req.Town,
		"village":          req.Village,
		"district_code":    req.DistrictCode,
		"street_code":      req.StreetCode,
		"village_code":     req.VillageCode,
		"address":          req.Address,
		"longitude":        req.Longitude,
		"latitude":         req.Latitude,
		"total_area":       req.TotalArea,
		"green_area":       req.GreenArea,
		"developer":        req.Developer,
		"property_company": req.PropertyCompany,
		"manager_name":     req.ManagerName,
		"manager_phone":    req.ManagerPhone,
		"service_phone":    req.ServicePhone,
		"handover_date":    handover,
		"status":           pmDefault(req.Status, 1),
		"sort":             req.Sort,
		"remark":           req.Remark,
		"update_by":        p.UserID,
		"update_time":      time.Now(),
	}
	if req.TenantID.Uint64() != 0 && req.TenantID.Uint64() != c.TenantID {
		fields["tenant_id"] = req.TenantID.Uint64()
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Community{}).Where("id = ?", communityID).Updates(fields).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	s.InvalidateAll(ctx)
	return nil
}

// DeleteCommunity 删除小区（软删除）：有业务数据时拒绝，避免产生孤儿数据。
func (s *Service) DeleteCommunity(ctx context.Context, communityID uint64, p *principal.Info) error {
	if _, err := s.findCommunity(ctx, p, communityID); err != nil {
		return err
	}
	checks := []struct {
		model any
		label string
	}{
		{&entity.House{}, "房屋"},
		{&entity.ResidentHouse{}, "住户关系"},
		{&entity.Bill{}, "账单"},
		{&entity.Device{}, "设备"},
	}
	for _, c := range checks {
		var n int64
		if err := s.Repo.DB.WithContext(ctx).Model(c.model).Where("community_id = ?", communityID).Count(&n).Error; err != nil {
			return errs.ErrDB.WithErr(err)
		}
		if n > 0 {
			return errs.Invalid("该小区已存在" + c.label + "数据，不能删除（可改为停用）")
		}
	}
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", communityID).Delete(&entity.Community{}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	// 清理授权关系
	if err := s.Repo.DB.WithContext(ctx).Where("community_id = ?", communityID).Delete(&entity.UserCommunity{}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	s.InvalidateAll(ctx)
	return nil
}

// ChangeCommunityStatus 启用/停用小区。
func (s *Service) ChangeCommunityStatus(ctx context.Context, communityID uint64, status int, p *principal.Info) error {
	if _, err := s.findCommunity(ctx, p, communityID); err != nil {
		return err
	}
	if status != 0 && status != 1 {
		return errs.ErrBadRequest
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Community{}).Where("id = ?", communityID).
		Updates(map[string]any{"status": status, "update_by": p.UserID, "update_time": time.Now()}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// OptionVO 通用下拉项。
type OptionVO struct {
	ID       uint64 `json:"id,string"`
	Name     string `json:"name"`
	Code     string `json:"code,omitempty"`
	ParentID uint64 `json:"parentId,string,omitempty"`
	Extra    string `json:"extra,omitempty"`
}

// CommunityOptions 当前用户可见的小区下拉（默认小区排在最前）。
func (s *Service) CommunityOptions(ctx context.Context, p *principal.Info) ([]OptionVO, error) {
	tx := s.communityTableScope(ctx, p).Where("status = 1")
	list := make([]entity.Community, 0, 32)
	if err := tx.Order("sort ASC, id ASC").Limit(2000).Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	out := make([]OptionVO, 0, len(list))
	// 默认小区置顶，方便前端"首次进入自动选中"
	for _, c := range list {
		if c.ID == p.DefaultCommunityID {
			out = append(out, OptionVO{ID: c.ID, Name: c.Name, Code: c.Code, Extra: "default"})
		}
	}
	for _, c := range list {
		if c.ID == p.DefaultCommunityID {
			continue
		}
		out = append(out, OptionVO{ID: c.ID, Name: c.Name, Code: c.Code})
	}
	return out, nil
}

// DistrictNode 行政区划树节点。
type DistrictNode struct {
	ID       uint64          `json:"id,string"`
	ParentID uint64          `json:"parentId,string"`
	Code     string          `json:"code"`
	Name     string          `json:"name"`
	Level    int             `json:"level"`
	Children []*DistrictNode `json:"children,omitempty"`
}

// DistrictTree 行政区划树（只到市/直辖市区县，用于兼容旧页面）。
//
// 注意：全量数据约 66 万条（含街道/社区），**不要**一次性返回整棵树；
// 前端选择行政区请用 /property/districts/children 逐级懒加载。
func (s *Service) DistrictTree(ctx context.Context) ([]*DistrictNode, error) {
	var provinces []entity.District
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.District{}).
		Where("level = 1 AND status = 1").Order("code ASC").Limit(100).Find(&provinces).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if len(provinces) == 0 {
		return []*DistrictNode{}, nil
	}
	ids := make([]uint64, 0, len(provinces))
	for _, p := range provinces {
		ids = append(ids, p.ID)
	}
	var cities []entity.District
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.District{}).
		Where("level = 2 AND parent_id IN ? AND status = 1", ids).Order("code ASC").Find(&cities).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	byParent := make(map[uint64][]*DistrictNode, len(provinces))
	for _, c := range cities {
		byParent[c.ParentID] = append(byParent[c.ParentID], &DistrictNode{ID: c.ID, ParentID: c.ParentID, Code: c.Code, Name: c.Name, Level: c.Level})
	}
	roots := make([]*DistrictNode, 0, len(provinces))
	for _, p := range provinces {
		roots = append(roots, &DistrictNode{
			ID: p.ID, ParentID: p.ParentID, Code: p.Code, Name: p.Name, Level: p.Level, Children: byParent[p.ID],
		})
	}
	return roots, nil
}

// ============================ 物业公司（租户） ============================

// CompanyQuery 物业公司列表查询。
type CompanyQuery struct {
	entity.PageQuery
	Keyword string `form:"keyword"`
	Status  *int   `form:"status"`
}

// CompanyVO 物业公司出参（租户 + 档案 + 统计）。
type CompanyVO struct {
	entity.Company
	TenantCode     string `json:"tenantCode"`
	ExpireAt       string `json:"expireAt"`
	CommunityCount int64  `json:"communityCount"`
	UserCount      int64  `json:"userCount"`
}

// ListCompanies 物业公司列表（平台侧使用；物业侧只返回自己）。
func (s *Service) ListCompanies(ctx context.Context, q *CompanyQuery, p *principal.Info) ([]CompanyVO, int64, error) {
	q.Normalize()
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Company{})
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(name LIKE ? OR credit_code LIKE ?)", kw+"%", kw+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Company, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	out := make([]CompanyVO, 0, len(list))
	for _, co := range list {
		vo := CompanyVO{Company: co}
		var t entity.Tenant
		if err := s.Repo.DB.WithContext(ctx).Where("id = ?", co.TenantID).First(&t).Error; err == nil {
			vo.TenantCode = t.Code
			vo.ExpireAt = t.ExpireAt
		}
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.Community{}).Where("tenant_id = ?", co.TenantID).Count(&vo.CommunityCount).Error
		_ = s.Repo.DB.WithContext(ctx).Model(&entity.User{}).Where("tenant_id = ?", co.TenantID).Count(&vo.UserCount).Error
		out = append(out, vo)
	}
	return out, total, nil
}

// CompanyReq 物业公司新增/修改入参。
type CompanyReq struct {
	TenantCode    string `json:"tenantCode" binding:"required,max=64"`
	Name          string `json:"name" binding:"required,max=100"`
	ShortName     string `json:"shortName" binding:"max=64"`
	CreditCode    string `json:"creditCode" binding:"max=32"`
	LicenseNo     string `json:"licenseNo" binding:"max=64"`
	Level         string `json:"level" binding:"max=32"`
	LegalPerson   string `json:"legalPerson" binding:"max=64"`
	ContactName   string `json:"contactName" binding:"max=64"`
	ContactPhone  string `json:"contactPhone" binding:"max=20"`
	Email         string `json:"email" binding:"max=128"`
	Province      string `json:"province" binding:"max=64"`
	City          string `json:"city" binding:"max=64"`
	District      string `json:"district" binding:"max=64"`
	Address       string `json:"address" binding:"max=255"`
	ServiceScope  string `json:"serviceScope" binding:"max=500"`
	ContractNo    string `json:"contractNo" binding:"max=64"`
	ContractStart string `json:"contractStart"`
	ContractEnd   string `json:"contractEnd"`
	ExpireAt      string `json:"expireAt"`
	MaxUsers      int    `json:"maxUsers"`
	Status        int    `json:"status"`
	Remark        string `json:"remark" binding:"max=500"`

	// 管理员账号（首次创建时可选；不传则自动按公司编码生成）
	AdminUsername string `json:"adminUsername" binding:"omitempty,min=3,max=32"`
	AdminPassword string `json:"adminPassword" binding:"omitempty,min=8,max=32"`
}

// CompanyCreateResult 创建物业公司结果（管理员初始密码只在这里返回一次）。
type CompanyCreateResult struct {
	TenantID      uint64 `json:"tenantId,string"`
	CompanyID     uint64 `json:"companyId,string"`
	AdminUsername string `json:"adminUsername"`
	AdminPassword string `json:"adminPassword"`
}

// CreateCompany 创建物业公司：租户 + 档案 + 组织根部门 + 内置角色 + 管理员账号。
func (s *Service) CreateCompany(ctx context.Context, req *CompanyReq, p *principal.Info) (*CompanyCreateResult, error) {
	if !p.IsSuper {
		return nil, errs.ErrDataForbidden // 只有平台管理员可以新增物业公司租户
	}
	code := strings.TrimSpace(req.TenantCode)
	var dup int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Tenant{}).Where("code = ?", code).Count(&dup).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if dup > 0 {
		return nil, errs.Invalid("物业公司编码已存在")
	}
	expireAt, err := parseDateTime(req.ExpireAt)
	if err != nil {
		return nil, err
	}
	contractStart, err := parseDate(req.ContractStart)
	if err != nil {
		return nil, err
	}
	contractEnd, err := parseDate(req.ContractEnd)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	tenantID := id.Next()
	tenantExpire := ""
	if expireAt != nil {
		tenantExpire = expireAt.Format("2006-01-02 15:04:05")
	}
	result := &CompanyCreateResult{TenantID: tenantID}

	err = s.Repo.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tenant := &entity.Tenant{
			Name:         strings.TrimSpace(req.Name),
			Code:         code,
			ContactName:  req.ContactName,
			ContactPhone: req.ContactPhone,
			MaxUsers:     req.MaxUsers,
			ExpireAt:     tenantExpire,
			Status:       pmDefault(req.Status, 1),
			Base:         entity.Base{ID: tenantID, CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
		}
		if err := tx.Create(tenant).Error; err != nil {
			return err
		}
		companyID := id.Next()
		company := &entity.Company{
			Base:          entity.Base{ID: companyID, CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
			TenantID:      tenantID,
			Name:          strings.TrimSpace(req.Name),
			ShortName:     req.ShortName,
			CreditCode:    req.CreditCode,
			LicenseNo:     req.LicenseNo,
			Level:         req.Level,
			LegalPerson:   req.LegalPerson,
			ContactName:   req.ContactName,
			ContactPhone:  req.ContactPhone,
			Email:         req.Email,
			Province:      req.Province,
			City:          req.City,
			District:      req.District,
			Address:       req.Address,
			ServiceScope:  req.ServiceScope,
			ContractNo:    req.ContractNo,
			ContractStart: contractStart,
			ContractEnd:   contractEnd,
			Status:        1,
		}
		if err := tx.Create(company).Error; err != nil {
			return err
		}
		result.CompanyID = companyID

		// 组织根部门
		rootDeptID := id.Next()
		dept := &entity.Dept{
			Base:      entity.Base{ID: rootDeptID, CreateBy: p.UserID, CreateTime: now},
			ParentID:  0,
			Ancestors: "0",
			Name:      strings.TrimSpace(req.Name),
			Code:      "HQ",
			Leader:    req.LegalPerson,
			Phone:     req.ContactPhone,
			Sort:      1,
			Status:    1,
			TenantID:  tenantID,
		}
		if err := tx.Create(dept).Error; err != nil {
			return err
		}

		// 克隆内置物业角色（含菜单授权），让新租户开箱可用
		roleID, err := s.clonePropertyRoles(ctx, tx, tenantID, now)
		if err != nil {
			return err
		}

		// 管理员账号
		username := strings.TrimSpace(req.AdminUsername)
		if username == "" {
			username = "admin_" + code
		}
		pwd := req.AdminPassword
		if pwd == "" {
			pwd = crypto.RandomPassword(12)
		}
		if err := crypto.CheckStrength(pwd, s.Cfg.Auth.PwdMinLen); err != nil {
			return errs.InvalidErr(err)
		}
		hash, err := crypto.HashPassword(pwd)
		if err != nil {
			return err
		}
		var dupUser int64
		if err := tx.Model(&entity.User{}).Where("username = ?", username).Count(&dupUser).Error; err != nil {
			return err
		}
		if dupUser > 0 {
			return errs.Invalid("管理员账号 " + username + " 已被占用，请指定其它账号")
		}
		userID := id.Next()
		user := &entity.User{
			Base:          entity.Base{ID: userID, CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now},
			Username:      username,
			Nickname:      req.Name + "管理员",
			Password:      hash,
			DeptID:        rootDeptID,
			Mobile:        req.ContactPhone,
			Email:         req.Email,
			Status:        1,
			PwdUpdateTime: &now,
			TenantID:      tenantID,
		}
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		if err := tx.Create(&entity.UserRole{UserID: userID, RoleID: roleID}).Error; err != nil {
			return err
		}
		result.AdminUsername = username
		result.AdminPassword = pwd
		return nil
	})
	if err != nil {
		return nil, wrapTxErr(err)
	}
	return result, nil
}

// clonePropertyRoles 为新租户创建"物业公司管理员"角色并返回其 ID。
//
// 规则（与需求一致）：
//   - 新公司只内置一个管理员角色，其余角色由管理员自行创建；
//   - 管理员角色拥有「系统监控 / 系统设置 / 租户管理与小区管理 / 菜单管理」之外的全部菜单，
//     即可在本公司内创建角色、用户、部门、岗位，并管理本公司全部小区；
//   - 小区范围为"本租户全部小区"（community_scope=1，随新增小区自动覆盖）。
func (s *Service) clonePropertyRoles(ctx context.Context, tx *gorm.DB, tenantID uint64, now time.Time) (uint64, error) {
	roleID := id.Next()
	role := &entity.Role{
		Base:           entity.Base{ID: roleID, CreateTime: now},
		Name:           "物业公司管理员",
		Code:           "pm_company_admin",
		Sort:           1,
		DataScope:      entity.DataScopeAll,
		CommunityScope: entity.RoleCommunityAll,
		Status:         1,
		IsBuiltin:      true,
		TenantID:       tenantID,
	}
	if err := tx.Create(role).Error; err != nil {
		return 0, err
	}
	// 授权：除平台级菜单（系统监控 / 系统设置 / 租户与小区 / 菜单管理）外的全部菜单
	if err := tx.Exec(`INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
		SELECT ?, id FROM sys_menu
		WHERE deleted = 0
		  AND id NOT IN (1030, 1080, 1200, 1300)
		  AND parent_id NOT IN (1030, 1080, 1200, 1300)`, roleID).Error; err != nil {
		return 0, err
	}
	return roleID, nil
}

// UpdateCompany 修改物业公司档案（平台侧）。
func (s *Service) UpdateCompany(ctx context.Context, companyID uint64, req *CompanyReq, p *principal.Info) error {
	var co entity.Company
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", companyID).First(&co).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.ErrNotFound
		}
		return errs.ErrDB.WithErr(err)
	}
	if !p.IsSuper && co.TenantID != p.TenantID {
		return errs.ErrDataForbidden
	}
	contractStart, err := parseDate(req.ContractStart)
	if err != nil {
		return err
	}
	contractEnd, err := parseDate(req.ContractEnd)
	if err != nil {
		return err
	}
	now := time.Now()
	fields := map[string]any{
		"name":           strings.TrimSpace(req.Name),
		"short_name":     req.ShortName,
		"credit_code":    req.CreditCode,
		"license_no":     req.LicenseNo,
		"level":          req.Level,
		"legal_person":   req.LegalPerson,
		"contact_name":   req.ContactName,
		"contact_phone":  req.ContactPhone,
		"email":          req.Email,
		"province":       req.Province,
		"city":           req.City,
		"district":       req.District,
		"address":        req.Address,
		"service_scope":  req.ServiceScope,
		"contract_no":    req.ContractNo,
		"contract_start": contractStart,
		"contract_end":   contractEnd,
		"status":         pmDefault(req.Status, 1),
		"remark":         req.Remark,
		"update_by":      p.UserID,
		"update_time":    now,
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Company{}).Where("id = ?", companyID).Updates(fields).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	// 同步租户基础信息（名称/联系人/上限/有效期/启用状态）
	tenantFields := map[string]any{
		"name":          strings.TrimSpace(req.Name),
		"contact_name":  req.ContactName,
		"contact_phone": req.ContactPhone,
		"max_users":     req.MaxUsers,
		"status":        pmDefault(req.Status, 1),
		"update_by":     p.UserID,
		"update_time":   now,
	}
	if req.ExpireAt != "" {
		if t, err := parseDateTime(req.ExpireAt); err == nil && t != nil {
			tenantFields["expire_at"] = t.Format("2006-01-02 15:04:05")
		}
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Tenant{}).Where("id = ?", co.TenantID).Updates(tenantFields).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	s.InvalidateAll(ctx)
	return nil
}

// DeleteCompany 删除物业公司（租户）：仍有小区时拒绝。
func (s *Service) DeleteCompany(ctx context.Context, companyID uint64, p *principal.Info) error {
	if !p.IsSuper {
		return errs.ErrDataForbidden
	}
	var co entity.Company
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", companyID).First(&co).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errs.ErrNotFound
		}
		return errs.ErrDB.WithErr(err)
	}
	var n int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Community{}).Where("tenant_id = ?", co.TenantID).Count(&n).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if n > 0 {
		return errs.Invalid("该物业公司下仍有小区，不能删除")
	}
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", companyID).Delete(&entity.Company{}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if err := s.Repo.DB.WithContext(ctx).Where("id = ?", co.TenantID).Delete(&entity.Tenant{}).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	return nil
}

// CompanyOptions 物业公司下拉（平台侧选择归属用）。
func (s *Service) CompanyOptions(ctx context.Context, p *principal.Info) ([]OptionVO, error) {
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Company{})
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	var list []entity.Company
	if err := tx.Order("name ASC").Limit(2000).Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	out := make([]OptionVO, 0, len(list))
	for _, c := range list {
		out = append(out, OptionVO{ID: c.ID, Name: c.Name, Code: c.ShortName})
	}
	return out, nil
}

// TenantListForCompany 供平台侧建小区时选择物业公司（直接读 sys_tenant）。
// 平台超管可见全部租户；物业公司账号只返回自己（避免泄露其它公司信息）。
func (s *Service) TenantListForCompany(ctx context.Context, p *principal.Info) ([]OptionVO, error) {
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Tenant{}).Where("status = 1")
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("id = ?", t)
	}
	var list []entity.Tenant
	if err := tx.Order("id ASC").Limit(2000).Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	out := make([]OptionVO, 0, len(list))
	for _, t := range list {
		out = append(out, OptionVO{ID: t.ID, Name: t.Name, Code: t.Code})
	}
	return out, nil
}

func pmDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func pmDefaultStr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
