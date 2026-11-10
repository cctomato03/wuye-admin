package service

import (
	"context"

	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/pkg/errs"
)

// 组织与权限（用户/角色/部门/岗位/员工）的租户解析。
//
// 规则：
//   - 物业公司用户只能管理本公司（tenantId 参数会被强制为本公司，传别家直接拒绝）；
//   - 平台默认租户（系统管理角色，默认 tenant_id=1）可以在组织与权限里切换任意物业公司；
//   - 任何写操作都要落在目标公司租户下，避免数据越界。

// DefaultTenantID 平台默认租户 ID（config: tenant.default_id，兜底 1）。
func (s *Service) DefaultTenantID() uint64 {
	if s.Cfg != nil && s.Cfg.Tenant.DefaultID > 0 {
		return s.Cfg.Tenant.DefaultID
	}
	return 1
}

// IsPlatformTenant 当前登录是否属于平台默认租户（系统管理角色）。
func (s *Service) IsPlatformTenant(p *principal.Info) bool {
	return p != nil && p.TenantID == s.DefaultTenantID()
}

// OrgTenant 解析本次组织操作的目标租户。
// requested=0 表示不切换（使用登录用户所属公司）。
func (s *Service) OrgTenant(ctx context.Context, p *principal.Info, requested uint64) (uint64, error) {
	if p == nil {
		return 0, errs.ErrForbidden
	}
	if requested == 0 || requested == p.TenantID {
		return p.TenantID, nil
	}
	// 只有平台默认租户可以切换其它物业公司
	if !s.IsPlatformTenant(p) {
		return 0, errs.New(errs.ErrForbidden.Code, "只能管理本物业公司的组织与权限")
	}
	var n int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Tenant{}).Where("id = ?", requested).Count(&n).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	if n == 0 {
		return 0, errs.Invalid("目标物业公司不存在")
	}
	return requested, nil
}

// AssertTenantAllowed 校验"某个已存在对象"的租户是否允许当前用户操作。
func (s *Service) AssertTenantAllowed(ctx context.Context, p *principal.Info, tenantID uint64) error {
	_, err := s.OrgTenant(ctx, p, tenantID)
	return err
}
