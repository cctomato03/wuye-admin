-- =============================================================================
-- 000029_property_menu_grant_backfill : 为已存在的物业角色补齐新分组授权
--
-- 背景：000028 把页面挂到了新的分组目录（小区管理/人员管理/物业公司/设备管理），
-- 但**在 000028 之前就已创建**的物业公司，其角色是从模板克隆的旧授权集合，
-- 缺少新分组目录的 menu_id，前端会把这些页面当成"孤立菜单"而不显示。
-- 这里按"角色已拥有该分组下任意页面"来补齐父目录授权，并对全部物业角色兜底补齐。
-- =============================================================================

SET NAMES utf8mb4;

-- 1) 小区管理（3200）
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT DISTINCT rm.role_id, 3200
FROM sys_role_menu rm
JOIN sys_role r ON r.id = rm.role_id AND r.deleted = 0
WHERE rm.menu_id IN (3010, 3020, 3070, 3080);

-- 2) 人员管理（3210）
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT DISTINCT rm.role_id, 3210
FROM sys_role_menu rm
JOIN sys_role r ON r.id = rm.role_id AND r.deleted = 0
WHERE rm.menu_id IN (3030, 3040, 3050);

-- 3) 物业公司（3220）
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT DISTINCT rm.role_id, 3220
FROM sys_role_menu rm
JOIN sys_role r ON r.id = rm.role_id AND r.deleted = 0
WHERE rm.menu_id IN (3150, 3060, 3250);

-- 4) 设备管理（3230）
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT DISTINCT rm.role_id, 3230
FROM sys_role_menu rm
JOIN sys_role r ON r.id = rm.role_id AND r.deleted = 0
WHERE rm.menu_id IN (3130, 3140);

-- 5) 全部内置物业角色兜底：至少能看到四个分组（子页面仍按各自授权过滤）
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id
FROM sys_role r
JOIN sys_menu m ON m.id IN (3200, 3210, 3220, 3230)
WHERE r.deleted = 0
  AND r.code IN ('pm_company_admin', 'pm_project_manager', 'pm_customer_service', 'pm_cashier', 'pm_access_admin');
