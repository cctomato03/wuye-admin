-- =============================================================================
-- 000030_property_menu_rename : 分组内的页面名称更精确（避免与分组同名）
-- =============================================================================

SET NAMES utf8mb4;

UPDATE sys_menu SET name = '小区信息', name_en = 'Communities'       WHERE id = 3010;
UPDATE sys_menu SET name = '公司信息', name_en = 'Company Profile'   WHERE id = 3150;
UPDATE sys_menu SET name = '设备信息', name_en = 'Devices'           WHERE id = 3130;
UPDATE sys_menu SET name = '组织与员工', name_en = 'Org & Employees' WHERE id = 3060;
