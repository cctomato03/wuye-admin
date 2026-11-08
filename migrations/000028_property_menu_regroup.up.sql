-- =============================================================================
-- 000028_property_menu_regroup : 物业菜单按功能分组（小区 / 人员 / 物业公司 / 收费 / 设备）
--
-- 目标：
--   * 取消「物业管理」一个目录塞满全部页面；
--   * 按业务域拆成 5 个一级分组，每组下挂对应页面；
--   * 「物业概览」不再单独占菜单，数据合并进工作台（菜单软删除）；
--   * 新增「岗位管理」入口（复用系统岗位页面）。
--
-- 路由说明：分组的 path 即该组页面的 URL 前缀，子页面用相对 path
-- （如 小区管理 /property/site + community = /property/site/community）。
-- =============================================================================

SET NAMES utf8mb4;

-- ---------- 1. 新建一级分组 ----------
INSERT IGNORE INTO sys_menu
  (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  (3200, 0, '小区管理', 'Site Management', 1, '', '/property/site',   'Layout', 'ri:home-line',              4, 1, 1, 1, 1, NOW(), '小区信息 / 楼宇结构 / 单位 / 车辆'),
  (3210, 0, '人员管理', 'People',          1, '', '/property/people', 'Layout', 'ri:contacts-line',          5, 1, 1, 1, 1, NOW(), '住户 / 访客 / 其他人员'),
  (3220, 0, '物业公司', 'Property Company', 1, '', '/property/org',   'Layout', 'ri:briefcase-4-line',       6, 1, 1, 1, 1, NOW(), '公司信息 / 组织与员工 / 岗位'),
  (3230, 0, '设备管理', 'Devices',         1, '', '/property/devices', 'Layout', 'ri:cpu-line',               8, 1, 1, 1, 1, NOW(), '设备信息 / 门禁授权');

-- 收费管理：从「物业管理」子目录升级为一级分组
UPDATE sys_menu SET parent_id = 0, name = '收费管理', name_en = 'Billing', path = '/property/fee',
       icon = 'ri:money-cny-circle-line', sort = 7 WHERE id = 3090;

-- ---------- 2. 页面挂到分组下（相对路径） ----------
UPDATE sys_menu SET parent_id = 3200, path = 'community', sort = 1 WHERE id = 3010;  -- 小区信息
UPDATE sys_menu SET parent_id = 3200, path = 'structure', sort = 2 WHERE id = 3020;  -- 楼宇结构
UPDATE sys_menu SET parent_id = 3200, path = 'merchant',  sort = 3 WHERE id = 3080;  -- 单位管理
UPDATE sys_menu SET parent_id = 3200, path = 'vehicle',   sort = 4 WHERE id = 3070;  -- 车辆管理

UPDATE sys_menu SET parent_id = 3210, path = 'resident', sort = 1 WHERE id = 3030;   -- 住户
UPDATE sys_menu SET parent_id = 3210, path = 'visitor',  sort = 2 WHERE id = 3040;   -- 访客
UPDATE sys_menu SET parent_id = 3210, path = 'other',    sort = 3 WHERE id = 3050;   -- 其他人员

UPDATE sys_menu SET parent_id = 3220, path = 'company',  sort = 1 WHERE id = 3150;   -- 公司信息
UPDATE sys_menu SET parent_id = 3220, path = 'employee', sort = 2 WHERE id = 3060;   -- 组织与员工

UPDATE sys_menu SET parent_id = 3230, path = 'info',   sort = 1 WHERE id = 3130;     -- 设备信息
UPDATE sys_menu SET parent_id = 3230, path = 'access', sort = 2 WHERE id = 3140;     -- 门禁授权

-- 岗位管理（复用系统岗位页面，权限沿用 system:post:list）
INSERT IGNORE INTO sys_menu
  (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  (3250, 3220, '岗位管理', 'Positions', 2, 'system:post:list', 'post', 'system/post/index', 'ri:id-card-line', 3, 1, 1, 1, 1, NOW(), '物业公司岗位维护');

-- ---------- 3. 概览并入工作台 ----------
UPDATE sys_menu SET deleted = UNIX_TIMESTAMP(NOW(3)) * 1000 WHERE id IN (3160);
-- 「物业管理」总目录不再使用（页面已迁移到各分组）
UPDATE sys_menu SET deleted = UNIX_TIMESTAMP(NOW(3)) * 1000 WHERE id IN (3000);

-- ---------- 4. 分组授权 ----------
-- 物业公司管理员：全部
INSERT IGNORE INTO sys_role_menu (role_id, menu_id) VALUES
  (310, 3200), (310, 3210), (310, 3220), (310, 3230), (310, 3250);

-- 项目经理：小区 / 人员 / 公司 / 设备
INSERT IGNORE INTO sys_role_menu (role_id, menu_id) VALUES
  (311, 3200), (311, 3210), (311, 3220), (311, 3230), (311, 3250);

-- 客服管家：小区 / 人员
INSERT IGNORE INTO sys_role_menu (role_id, menu_id) VALUES
  (312, 3200), (312, 3210);

-- 收费员：人员（住户只读）/ 收费
INSERT IGNORE INTO sys_role_menu (role_id, menu_id) VALUES
  (313, 3210);

-- 门禁管理员：人员 / 设备
INSERT IGNORE INTO sys_role_menu (role_id, menu_id) VALUES
  (314, 3210), (314, 3230);
