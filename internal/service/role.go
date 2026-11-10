package service

import (
	"context"
	"gorm.io/gorm"
	"strings"
	"time"

	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/internal/repository"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
	"web-basic/pkg/jsonx"
)

// RoleQuery 角色列表查询。
type RoleQuery struct {
	entity.PageQuery
	Name     string       `form:"name"`
	Status   *int         `form:"status"`
	TenantID jsonx.Uint64 `form:"tenantId"` // 平台超管按物业公司筛选
}

// RoleReq 角色新增/修改入参。
//
// CommunityScope/CommunityIDs 决定该角色能管理哪些小区：
//   - 1 全部：本物业公司所有小区（随公司新增自动覆盖；公司管理员默认此项）
//   - 2 指定：只能管理 CommunityIDs 里的小区，且必须是创建者自己可管理的小区子集
type RoleReq struct {
	Name           string            `json:"name" binding:"required,max=64"`
	Code           string            `json:"code" binding:"required,max=64"`
	Sort           int               `json:"sort"`
	DataScope      int               `json:"dataScope" binding:"required,oneof=1 2 3 4 5"`
	CommunityScope int               `json:"communityScope" binding:"omitempty,oneof=1 2"`
	CommunityIDs   jsonx.Uint64Slice `json:"communityIds"`
	TenantID       jsonx.Uint64      `json:"tenantId"` // 仅平台超管使用：为指定物业公司建角色
	Status         int               `json:"status"`
	Remark         string            `json:"remark"`
}

// ListRoles 分页查询角色（本租户）。
func (s *Service) ListRoles(ctx context.Context, q *RoleQuery, p *principal.Info) ([]entity.Role, int64, error) {
	q.Normalize()
	tid, err := s.OrgTenant(ctx, p, q.TenantID.Uint64())
	if err != nil {
		return nil, 0, err
	}
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Role{}).Where("tenant_id = ?", tid)
	if q.Name != "" {
		tx = tx.Where("name LIKE ? OR code LIKE ?", q.Name+"%", q.Name+"%")
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Role, 0, q.PageSize)
	if err := tx.Order("sort ASC, id ASC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	return list, total, nil
}

// AllRoles 全部启用角色（用户分配角色下拉，仅本租户）。
func (s *Service) AllRoles(ctx context.Context, p *principal.Info, tenantID uint64) ([]entity.Role, error) {
	tid, err := s.OrgTenant(ctx, p, tenantID)
	if err != nil {
		return nil, err
	}
	var list []entity.Role
	err = s.Repo.DB.WithContext(ctx).Model(&entity.Role{}).
		Where("tenant_id = ? AND status = 1", tid).Order("sort ASC").Find(&list).Error
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return list, nil
}

// CreateRole 新增角色。
//
// 两条硬约束（防止"造一个比我权限还大的角色"）：
//  1. 非超管只能勾选自己已拥有的菜单（菜单授权在 GrantRole 里校验，这里先约束小区范围）；
//  2. 指定小区必须是创建者可管理小区的子集。
func (s *Service) CreateRole(ctx context.Context, req *RoleReq, p *principal.Info) (uint64, error) {
	tenantID, err := s.OrgTenant(ctx, p, req.TenantID.Uint64())
	if err != nil {
		return 0, err
	}
	var n int64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Role{}).
		Where("tenant_id = ? AND code = ?", tenantID, req.Code).Count(&n).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	if n > 0 {
		return 0, errs.ErrDuplicate
	}
	cScope := req.CommunityScope
	if cScope == 0 {
		// 未指定时：有"全公司小区"数据权限的人默认建全公司角色，否则必须显式选择小区
		if s.canManageAllCommunities(p) {
			cScope = entity.RoleCommunityAll
		} else {
			cScope = entity.RoleCommunityAssigned
		}
	}
	if cScope == entity.RoleCommunityAll && !s.canManageAllCommunities(p) {
		return 0, errs.Invalid("你没有「全公司小区」的数据权限，不能创建管理全部小区的角色")
	}
	if cScope == entity.RoleCommunityAssigned {
		if err := s.assertCommunitiesManageable(ctx, p, tenantID, req.CommunityIDs.ToUint64()); err != nil {
			return 0, err
		}
	}
	now := time.Now()
	role := &entity.Role{
		Base: entity.Base{
			ID:         id.Next(),
			CreateBy:   p.UserID,
			CreateTime: now,
			UpdateBy:   p.UserID,
			UpdateTime: &now,
			Remark:     req.Remark,
		},
		Name:           req.Name,
		Code:           req.Code,
		Sort:           req.Sort,
		DataScope:      req.DataScope,
		CommunityScope: cScope,
		Status:         req.Status,
		TenantID:       tenantID,
	}
	if err := s.Repo.DB.WithContext(ctx).Create(role).Error; err != nil {
		return 0, errs.ErrDB.WithErr(err)
	}
	if cScope == entity.RoleCommunityAssigned {
		if err := s.Repo.SetRoleCommunities(ctx, tenantID, role.ID, req.CommunityIDs.ToUint64()); err != nil {
			return 0, errs.ErrDB.WithErr(err)
		}
	}
	return role.ID, nil
}

// UpdateRole 修改角色。
func (s *Service) UpdateRole(ctx context.Context, roleID uint64, req *RoleReq, p *principal.Info) error {
	role, err := s.roleOfTenant(ctx, p, roleID)
	if err != nil {
		return err
	}
	if role.IsBuiltin && role.Code == "super_admin" {
		return errs.ErrCannotOperateSup
	}
	cScope := req.CommunityScope
	if cScope == 0 {
		cScope = role.CommunityScope
	}
	if cScope == entity.RoleCommunityAll && !s.canManageAllCommunities(p) {
		return errs.Invalid("你没有「全公司小区」的数据权限，不能把角色设为管理全部小区")
	}
	if req.CommunityIDs != nil && cScope == entity.RoleCommunityAssigned {
		if err := s.assertCommunitiesManageable(ctx, p, role.TenantID, req.CommunityIDs.ToUint64()); err != nil {
			return err
		}
	}
	if err := repository.UpdateByID(ctx, s.Repo.DB, &entity.Role{}, roleID, map[string]any{
		"name":            req.Name,
		"code":            req.Code,
		"sort":            req.Sort,
		"data_scope":      req.DataScope,
		"community_scope": cScope,
		"status":          req.Status,
		"remark":          req.Remark,
		"update_by":       p.UserID,
		"update_time":     time.Now(),
	}); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if req.CommunityIDs != nil {
		var ids []uint64
		if cScope == entity.RoleCommunityAssigned {
			ids = req.CommunityIDs.ToUint64()
		}
		if err := s.Repo.SetRoleCommunities(ctx, role.TenantID, roleID, ids); err != nil {
			return errs.ErrDB.WithErr(err)
		}
	}
	s.InvalidateRoleUsers(ctx, roleID)
	return nil
}

// DeleteRole 删除角色。
func (s *Service) DeleteRole(ctx context.Context, roleID uint64, p *principal.Info) error {
	role, err := s.roleOfTenant(ctx, p, roleID)
	if err != nil {
		return err
	}
	if role.IsBuiltin {
		return errs.ErrCannotOperateSup
	}
	used, err := s.Repo.CountUsersOfRole(ctx, roleID)
	if err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if used > 0 {
		return errs.ErrRecordUsed
	}
	if err := repository.Result(s.Repo.DB.WithContext(ctx).Delete(&entity.Role{}, roleID)); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	_ = s.Repo.DB.WithContext(ctx).Where("role_id = ?", roleID).Delete(&entity.RoleMenu{}).Error
	_ = s.Repo.DB.WithContext(ctx).Where("role_id = ?", roleID).Delete(&entity.RoleDept{}).Error
	_ = s.Repo.DB.WithContext(ctx).Where("role_id = ?", roleID).Delete(&entity.RoleCommunity{}).Error
	s.InvalidateAll(ctx)
	return nil
}

// GrantReq 角色授权入参。
type GrantReq struct {
	MenuIDs        jsonx.Uint64Slice `json:"menuIds"`
	DeptIDs        jsonx.Uint64Slice `json:"deptIds"`
	CommunityScope *int              `json:"communityScope"`
	CommunityIDs   jsonx.Uint64Slice `json:"communityIds"`
}

// GrantRole 授权（菜单/按钮 + 自定义数据权限部门 + 可管理小区）。
func (s *Service) GrantRole(ctx context.Context, roleID uint64, req *GrantReq, p *principal.Info) error {
	role, err := s.roleOfTenant(ctx, p, roleID)
	if err != nil {
		return err
	}
	if role.Code == "super_admin" {
		return errs.ErrCannotOperateSup
	}
	// 子角色的菜单必须是"我"已有菜单的子集（超管不受限）
	if err := s.assertMenusAssignable(ctx, p, req.MenuIDs.ToUint64()); err != nil {
		return err
	}
	if err := s.Repo.SetRoleMenus(ctx, roleID, req.MenuIDs.ToUint64()); err != nil {
		return errs.ErrDB.WithErr(err)
	}
	// deptIds 为 nil 表示"本次只授权菜单"，不能覆盖已有数据权限部门
	if req.DeptIDs != nil {
		if err := s.Repo.SetRoleDepts(ctx, roleID, req.DeptIDs.ToUint64()); err != nil {
			return errs.ErrDB.WithErr(err)
		}
	}
	// 小区范围
	if req.CommunityScope != nil {
		cScope := *req.CommunityScope
		if cScope == entity.RoleCommunityAll && !s.canManageAllCommunities(p) {
			return errs.Invalid("你没有「全公司小区」的数据权限，不能把角色设为管理全部小区")
		}
		if cScope == entity.RoleCommunityAssigned {
			if err := s.assertCommunitiesManageable(ctx, p, role.TenantID, req.CommunityIDs.ToUint64()); err != nil {
				return err
			}
		}
		if err := repository.UpdateByID(ctx, s.Repo.DB, &entity.Role{}, roleID, map[string]any{
			"community_scope": cScope,
			"update_by":       p.UserID,
			"update_time":     time.Now(),
		}); err != nil {
			return errs.ErrDB.WithErr(err)
		}
		if err := s.Repo.SetRoleCommunities(ctx, role.TenantID, roleID, func() []uint64 {
			if cScope == entity.RoleCommunityAssigned {
				return req.CommunityIDs.ToUint64()
			}
			return nil
		}()); err != nil {
			return errs.ErrDB.WithErr(err)
		}
	}
	s.InvalidateRoleUsers(ctx, roleID)
	return nil
}

// RoleGranted 角色已授权菜单与部门。
type RoleGranted struct {
	MenuIDs        jsonx.Uint64Slice `json:"menuIds"`
	DeptIDs        jsonx.Uint64Slice `json:"deptIds"`
	CommunityScope int               `json:"communityScope"`
	CommunityIDs   jsonx.Uint64Slice `json:"communityIds"`
}

// RoleGrantedInfo 查询角色授权情况。
func (s *Service) RoleGrantedInfo(ctx context.Context, roleID uint64) (*RoleGranted, error) {
	menuIDs, err := s.Repo.MenuIDsOfRole(ctx, roleID)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	var deptIDs []uint64
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.RoleDept{}).
		Where("role_id = ?", roleID).Pluck("dept_id", &deptIDs).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	var cScope int
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Role{}).Where("id = ?", roleID).Pluck("community_scope", &cScope).Error
	commIDs, err := s.Repo.CommunityIDsOfRoles(ctx, []uint64{roleID})
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	return &RoleGranted{MenuIDs: menuIDs, DeptIDs: deptIDs, CommunityScope: cScope, CommunityIDs: commIDs}, nil
}

// ---------- 权限边界辅助 ----------

// roleOfTenant 读取本租户角色（超管可跨租户查看）。
func (s *Service) roleOfTenant(ctx context.Context, p *principal.Info, roleID uint64) (*entity.Role, error) {
	role, err := s.Repo.FindRoleByID(ctx, roleID)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	if role == nil {
		return nil, errs.ErrNotFound
	}
	if err := s.AssertTenantAllowed(ctx, p, role.TenantID); err != nil {
		return nil, errs.New(errs.ErrNotFound.Code, errs.ErrNotFound.Msg)
	}
	return role, nil
}

// canManageAllCommunities 当前用户是否具备"本租户全部小区"的管理范围。
func (s *Service) canManageAllCommunities(p *principal.Info) bool {
	if p == nil {
		return false
	}
	// 平台默认租户（系统管理角色）在「组织与权限」里切换公司时，按目标公司的全部小区处理
	return p.IsSuper || p.CommunityAll || p.TenantID == s.DefaultTenantID()
}

// assertCommunitiesManageable 校验目标小区都在当前用户可管理范围内。
func (s *Service) assertCommunitiesManageable(ctx context.Context, p *principal.Info, targetTenantID uint64, ids []uint64) error {
	if len(ids) == 0 {
		return errs.Invalid("请为该角色选择可管理的小区")
	}
	if s.canManageAllCommunities(p) {
		// 仍需校验小区属于目标租户，避免把别家公司的小区挂到本角色上
		tenantID := targetTenantID
		if tenantID == 0 && p != nil {
			tenantID = p.TenantID
		}
		var n int64
		if err := s.Repo.DB.WithContext(ctx).Model(&entity.Community{}).
			Where("tenant_id = ? AND id IN ?", tenantID, ids).Count(&n).Error; err != nil {
			return errs.ErrDB.WithErr(err)
		}
		if int(n) != len(ids) {
			return errs.Invalid("存在不属于本物业公司的小区")
		}
		return nil
	}
	allowed := idsToSet(p.CommunityIDs)
	for _, id := range ids {
		if _, ok := allowed[id]; !ok {
			return errs.Invalid("只能把角色分配给「你自己能管理的小区」")
		}
	}
	return nil
}

// callerMenuIDs 当前用户可下放的菜单 ID 集合（超管返回 all=true）。
func (s *Service) callerMenuIDs(ctx context.Context, p *principal.Info) (map[uint64]struct{}, bool, error) {
	if p == nil || p.IsSuper {
		return nil, true, nil
	}
	roles, err := s.Repo.RolesOfUser(ctx, p.UserID)
	if err != nil {
		return nil, false, errs.ErrDB.WithErr(err)
	}
	roleIDs := make([]uint64, 0, len(roles))
	for _, r := range roles {
		roleIDs = append(roleIDs, r.ID)
	}
	ids, err := s.Repo.MenuIDsOfRoles(ctx, roleIDs)
	if err != nil {
		return nil, false, errs.ErrDB.WithErr(err)
	}
	set := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set, false, nil
}

// assertMenusAssignable 校验要授出的菜单不超出当前用户自身权限。
func (s *Service) assertMenusAssignable(ctx context.Context, p *principal.Info, menuIDs []uint64) error {
	set, all, err := s.callerMenuIDs(ctx, p)
	if err != nil {
		return err
	}
	if all || len(menuIDs) == 0 {
		return nil
	}
	over := make([]string, 0)
	for _, id := range menuIDs {
		if _, ok := set[id]; !ok {
			over = append(over, itoa64(int64(id)))
		}
	}
	if len(over) > 0 {
		return errs.Invalid("不能授予超出自身权限的菜单（越权菜单数：" + itoa64(int64(len(over))) + "）")
	}
	return nil
}

// AssignableMenuTree 当前用户"可以授出"的菜单树（角色授权弹窗用）。
func (s *Service) AssignableMenuTree(ctx context.Context, p *principal.Info) ([]*MenuNode, error) {
	list, err := s.Repo.AllMenus(ctx, 0)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	set, all, err := s.callerMenuIDs(ctx, p)
	if err != nil {
		return nil, err
	}
	if all {
		return buildMenuTree(list), nil
	}
	filtered := make([]entity.Menu, 0, len(list))
	for _, m := range list {
		if _, ok := set[m.ID]; ok {
			filtered = append(filtered, m)
		}
	}
	return buildMenuTree(filtered), nil
}

// RoleCommunityOptions 角色可分配的小区下拉（受当前用户管理范围限制）。
func (s *Service) RoleCommunityOptions(ctx context.Context, p *principal.Info, tenantID uint64) ([]OptionVO, error) {
	tid, err := s.OrgTenant(ctx, p, tenantID)
	if err != nil {
		return nil, err
	}
	var tx *gorm.DB
	if s.IsPlatformTenant(p) {
		// 平台管理角色：按"当前切换的物业公司"列出全部小区
		tx = s.Repo.DB.WithContext(ctx).Model(&entity.Community{}).Where("tenant_id = ? AND status = 1", tid)
	} else {
		tx = s.communityTableScope(ctx, p).Where("status = 1")
	}
	var list []entity.Community
	if err := tx.Order("sort ASC, id ASC").Limit(5000).Find(&list).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	out := make([]OptionVO, 0, len(list))
	for _, c := range list {
		out = append(out, OptionVO{ID: c.ID, Name: strings.TrimSpace(c.Name), Code: c.Code})
	}
	return out, nil
}
