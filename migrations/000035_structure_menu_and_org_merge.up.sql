-- =============================================================================
-- 000035_structure_menu_and_org_merge : 层级管理/房屋管理拆分 + 物业公司并入组织与权限
--
-- 1) 「楼宇结构」拆成两个页面：层级管理（自定义层级树）与房屋管理（挂在节点上的房屋）；
-- 2) 员工管理、岗位管理并入「组织与权限」（系统管理），去掉独立的「物业公司」分组；
-- 3) 岗位管理复用系统岗位页面（删掉重复入口）。
-- =============================================================================

SET NAMES utf8mb4;

-- 1) 层级管理
UPDATE sys_menu SET name = '层级管理', name_en = 'Hierarchy', path = 'hierarchy',
       component = 'property/hierarchy/index', icon = 'ri:organization-chart', sort = 1
WHERE id = 3020;

-- 2) 房屋管理（新增）
INSERT IGNORE INTO sys_menu
  (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  (3270, 3200, '房屋管理', 'Houses', 2, 'property:structure:list', 'house', 'property/house/index', 'ri:home-4-line', 2, 1, 1, 1, 1, NOW(), '房屋挂在层级节点上，支持批量生成与 Excel 导入');

-- 3) 员工管理并入组织与权限
UPDATE sys_menu SET parent_id = 1000, path = 'employee', name = '员工管理', name_en = 'Employees',
       component = 'property/employee/index', sort = 6
WHERE id = 3060;

-- 4) 岗位管理：系统里已有「岗位管理」(1050)，删掉物业下的重复入口
UPDATE sys_menu SET deleted = UNIX_TIMESTAMP(NOW(3)) * 1000 WHERE id = 3250;

-- 5) 取消独立「物业公司」分组
UPDATE sys_menu SET deleted = UNIX_TIMESTAMP(NOW(3)) * 1000 WHERE id = 3220;

-- 6) 授权：拥有层级管理的角色自动获得房屋管理；内置公司管理员补新菜单
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT DISTINCT role_id, 3270 FROM sys_role_menu WHERE menu_id = 3020;

INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT r.id, 3270 FROM sys_role r WHERE r.code = 'pm_company_admin' AND r.deleted = 0;
