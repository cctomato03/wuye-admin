-- =============================================================================
-- 000033_property_community_api_perm : 保留「小区列表接口」权限点
--
-- 000032 把物业模块里的「小区信息」页面并入租户管理后，页面级权限码 property:community:list
-- 随菜单一起失效，导致物业公司管理员调用小区列表接口时 403。
-- 这里补一个不可见的接口权限点（type=4），只用于接口鉴权，不出现在菜单里，
-- 并把原来拥有「小区信息」菜单的角色授权迁移过来。
-- =============================================================================

SET NAMES utf8mb4;

INSERT IGNORE INTO sys_menu
  (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  (3260, 3200, '小区数据接口', 'Community API', 4, 'property:community:list', '', '', '', 99, 0, 1, 1, 1, NOW(), '接口权限点：小区列表/详情（不显示在菜单中）');

INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT DISTINCT role_id, 3260 FROM sys_role_menu WHERE menu_id = 3010;

-- 内置物业公司管理员（含历史克隆）统一补上
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT r.id, 3260 FROM sys_role r WHERE r.code = 'pm_company_admin' AND r.deleted = 0;
