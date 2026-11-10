-- =============================================================================
-- 000038_org_roles_baseline : 组织与权限基线
--
-- 1) 系统默认租户只保留两个角色：超级管理员(super_admin) + 物业管理员(pm_company_admin)；
--    其余脚手架内置角色（common / pm_project_manager / pm_customer_service / pm_cashier / pm_access_admin）停用；
-- 2) admin 用户只保留超级管理员角色；
-- 3) 物业管理员权限 = 全部菜单 - 系统设置(1300) - 系统监控(1200) - 菜单管理(1030) 及其子菜单：
--    用户/角色/部门/岗位/员工可管，但看不到平台级配置与菜单权限，避免越界改平台结构。
-- =============================================================================

SET NAMES utf8mb4;

-- 1) 停用多余的内置角色
UPDATE sys_role SET deleted = UNIX_TIMESTAMP(NOW(3)) * 1000, status = 2
WHERE tenant_id = 1 AND code IN ('common', 'pm_project_manager', 'pm_customer_service', 'pm_cashier', 'pm_access_admin');

DELETE ur FROM sys_user_role ur JOIN sys_role r ON r.id = ur.role_id
WHERE r.deleted <> 0 AND r.code IN ('common', 'pm_project_manager', 'pm_customer_service', 'pm_cashier', 'pm_access_admin');

DELETE rm FROM sys_role_menu rm JOIN sys_role r ON r.id = rm.role_id
WHERE r.deleted <> 0 AND r.code IN ('common', 'pm_project_manager', 'pm_customer_service', 'pm_cashier', 'pm_access_admin');

-- 2) 只留一个“物业管理员”内置角色（统一命名）
UPDATE sys_role SET name = '物业管理员', remark = '物业公司内置管理员：除系统设置/系统监控/菜单管理外全部权限',
    is_builtin = 1, data_scope = 1, community_scope = 1, status = 1, deleted = 0
WHERE code = 'pm_company_admin';

-- 3) admin 用户只保留超级管理员角色
DELETE ur FROM sys_user_role ur
JOIN sys_role r ON r.id = ur.role_id
WHERE ur.user_id = (SELECT id FROM sys_user u WHERE u.username = 'admin' AND u.deleted = 0 LIMIT 1)
  AND r.code <> 'super_admin';

-- 4) 物业管理员：清空后重建为“除 1030/1200/1300 及其子菜单外的全部菜单”
DELETE rm FROM sys_role_menu rm JOIN sys_role r ON r.id = rm.role_id WHERE r.code = 'pm_company_admin';

INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id
FROM sys_role r
JOIN sys_menu m ON m.deleted = 0 AND m.status = 1
WHERE r.code = 'pm_company_admin' AND r.deleted = 0
  AND m.id NOT IN (1030, 1200, 1300)
  AND m.parent_id NOT IN (1030, 1200, 1300)
  AND m.parent_id NOT IN (SELECT id FROM (SELECT id FROM sys_menu WHERE parent_id IN (1030, 1200, 1300)) t);

-- 5) 超级管理员保持全权限（补齐可能缺失的授权）
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id FROM sys_role r JOIN sys_menu m ON m.deleted = 0
WHERE r.code = 'super_admin' AND r.deleted = 0;
