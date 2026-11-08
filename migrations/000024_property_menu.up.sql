-- =============================================================================
-- 000024_property_menu : 物业管理菜单、按钮权限、字典、内置角色与授权
-- 全部 INSERT IGNORE，可重复执行。
-- =============================================================================

SET NAMES utf8mb4;

-- ---------- 1. 菜单与权限点 ----------
INSERT IGNORE INTO sys_menu
  (id, parent_id, name, name_en, type, permission, path, component, icon, sort, visible, keep_alive, status, tenant_id, create_time, remark)
VALUES
  (3000, 0,    '物业管理', 'Property',        1, '',                          '/property',  'Layout',                  'ri:organization-chart', 5,  1, 1, 1, 1, NOW(), '小区/楼宇/人员/收费/设备'),

  -- 小区
  (3010, 3000, '小区管理', 'Communities',     2, 'property:community:list',   'community',  'property/community/index', 'ri:home-line',         1,  1, 1, 1, 1, NOW(), ''),
  (3011, 3010, '新增',     'Create',          3, 'property:community:create', '',           '',                        '',                     1,  1, 1, 1, 1, NOW(), ''),
  (3012, 3010, '修改',     'Update',          3, 'property:community:update', '',           '',                        '',                     2,  1, 1, 1, 1, NOW(), ''),
  (3013, 3010, '删除',     'Delete',          3, 'property:community:delete', '',           '',                        '',                     3,  1, 1, 1, 1, NOW(), ''),
  (3014, 3010, '启停',     'Status',          3, 'property:community:status', '',           '',                        '',                     4,  1, 1, 1, 1, NOW(), ''),

  -- 楼宇结构
  (3020, 3000, '楼宇结构', 'Building Structure', 2, 'property:structure:list', 'structure', 'property/structure/index', 'ri:layout-grid-line',  2,  1, 1, 1, 1, NOW(), '区域/楼栋/单元/楼层/房屋'),
  (3021, 3020, '新增',     'Create',          3, 'property:structure:create', '',           '',                        '',                     1,  1, 1, 1, 1, NOW(), ''),
  (3022, 3020, '修改',     'Update',          3, 'property:structure:update', '',           '',                        '',                     2,  1, 1, 1, 1, NOW(), ''),
  (3023, 3020, '删除',     'Delete',          3, 'property:structure:delete', '',           '',                        '',                     3,  1, 1, 1, 1, NOW(), ''),
  (3024, 3020, '导入',     'Import',          3, 'property:structure:import', '',           '',                        '',                     4,  1, 1, 1, 1, NOW(), 'Excel 批量导入'),

  -- 住户
  (3030, 3000, '住户管理', 'Residents',       2, 'property:resident:list',    'resident',   'property/resident/index',  'ri:user-location-line', 3,  1, 1, 1, 1, NOW(), ''),
  (3031, 3030, '新增',     'Create',          3, 'property:resident:create',  '',           '',                        '',                     1,  1, 1, 1, 1, NOW(), ''),
  (3032, 3030, '修改',     'Update',          3, 'property:resident:update',  '',           '',                        '',                     2,  1, 1, 1, 1, NOW(), ''),
  (3033, 3030, '删除',     'Delete',          3, 'property:resident:delete',  '',           '',                        '',                     3,  1, 1, 1, 1, NOW(), ''),
  (3034, 3030, '房屋关系', 'House Relation',  3, 'property:resident:bind',    '',           '',                        '',                     4,  1, 1, 1, 1, NOW(), '登记/解除/续期/还原'),
  (3035, 3030, '人脸照片', 'Face',            3, 'property:resident:face',    '',           '',                        '',                     5,  1, 1, 1, 1, NOW(), ''),

  -- 访客
  (3040, 3000, '访客登记', 'Visitors',        2, 'property:visitor:list',     'visitor',    'property/visitor/index',   'ri:contacts-line',      4,  1, 1, 1, 1, NOW(), ''),
  (3041, 3040, '新增',     'Create',          3, 'property:visitor:create',   '',           '',                        '',                     1,  1, 1, 1, 1, NOW(), ''),
  (3042, 3040, '修改',     'Update',          3, 'property:visitor:update',   '',           '',                        '',                     2,  1, 1, 1, 1, NOW(), ''),
  (3043, 3040, '离场/取消', 'Leave',          3, 'property:visitor:leave',    '',           '',                        '',                     3,  1, 1, 1, 1, NOW(), ''),
  (3044, 3040, '删除',     'Delete',          3, 'property:visitor:delete',   '',           '',                        '',                     4,  1, 1, 1, 1, NOW(), ''),

  -- 其他人员
  (3050, 3000, '其他人员', 'Other Personnel', 2, 'property:other:list',       'other',      'property/other/index',     'ri:user-settings-line', 5,  1, 1, 1, 1, NOW(), '保洁/维修/施工等临时人员'),
  (3051, 3050, '新增',     'Create',          3, 'property:other:create',     '',           '',                        '',                     1,  1, 1, 1, 1, NOW(), ''),
  (3052, 3050, '修改',     'Update',          3, 'property:other:update',     '',           '',                        '',                     2,  1, 1, 1, 1, NOW(), ''),
  (3053, 3050, '删除',     'Delete',          3, 'property:other:delete',     '',           '',                        '',                     3,  1, 1, 1, 1, NOW(), ''),
  (3054, 3050, '人员类别', 'Category',        3, 'property:other:category',   '',           '',                        '',                     4,  1, 1, 1, 1, NOW(), ''),

  -- 员工
  (3060, 3000, '员工管理', 'Employees',       2, 'property:employee:list',    'employee',   'property/employee/index',  'ri:team-line',          6,  1, 1, 1, 1, NOW(), '组织架构与人员'),
  (3061, 3060, '新增',     'Create',          3, 'property:employee:create',  '',           '',                        '',                     1,  1, 1, 1, 1, NOW(), ''),
  (3062, 3060, '修改',     'Update',          3, 'property:employee:update',  '',           '',                        '',                     2,  1, 1, 1, 1, NOW(), ''),
  (3063, 3060, '删除',     'Delete',          3, 'property:employee:delete',  '',           '',                        '',                     3,  1, 1, 1, 1, NOW(), ''),
  (3064, 3060, '小区授权', 'Grant',           3, 'property:employee:grant',   '',           '',                        '',                     4,  1, 1, 1, 1, NOW(), ''),

  -- 车辆
  (3070, 3000, '车辆管理', 'Vehicles',        2, 'property:vehicle:list',     'vehicle',    'property/vehicle/index',   'ri:bus-2-line',         7,  1, 1, 1, 1, NOW(), ''),
  (3071, 3070, '新增',     'Create',          3, 'property:vehicle:create',   '',           '',                        '',                     1,  1, 1, 1, 1, NOW(), ''),
  (3072, 3070, '修改',     'Update',          3, 'property:vehicle:update',   '',           '',                        '',                     2,  1, 1, 1, 1, NOW(), ''),
  (3073, 3070, '删除',     'Delete',          3, 'property:vehicle:delete',   '',           '',                        '',                     3,  1, 1, 1, 1, NOW(), ''),
  (3074, 3070, '车位管理', 'Parking Space',   3, 'property:vehicle:space',    '',           '',                        '',                     4,  1, 1, 1, 1, NOW(), ''),

  -- 单位
  (3080, 3000, '单位管理', 'Merchants',       2, 'property:merchant:list',    'merchant',   'property/merchant/index',  'ri:shopping-bag-3-line', 8, 1, 1, 1, 1, NOW(), '办公单位/商铺'),
  (3081, 3080, '新增',     'Create',          3, 'property:merchant:create',  '',           '',                        '',                     1,  1, 1, 1, 1, NOW(), ''),
  (3082, 3080, '修改',     'Update',          3, 'property:merchant:update',  '',           '',                        '',                     2,  1, 1, 1, 1, NOW(), ''),
  (3083, 3080, '删除',     'Delete',          3, 'property:merchant:delete',  '',           '',                        '',                     3,  1, 1, 1, 1, NOW(), ''),

  -- 收费
  (3090, 3000, '收费管理', 'Billing',         1, '',                          '/property/fee',  'Layout',                'ri:money-cny-circle-line', 9, 1, 1, 1, 1, NOW(), '项目/账单/抄表/缴费'),
  (3091, 3090, '收费项目', 'Fee Items',       2, 'property:fee:item:list',    'item',       'property/fee/item/index',  'ri:price-tag-line',     1,  1, 1, 1, 1, NOW(), ''),
  (3092, 3091, '新增',     'Create',          3, 'property:fee:item:create',  '',           '',                        '',                     1,  1, 1, 1, 1, NOW(), ''),
  (3093, 3091, '修改',     'Update',          3, 'property:fee:item:update',  '',           '',                        '',                     2,  1, 1, 1, 1, NOW(), ''),
  (3094, 3091, '删除',     'Delete',          3, 'property:fee:item:delete',  '',           '',                        '',                     3,  1, 1, 1, 1, NOW(), ''),
  (3096, 3090, '账单管理', 'Bills',           2, 'property:fee:bill:list',    'bill',       'property/fee/bill/index',  'ri:bill-line',          2,  1, 1, 1, 1, NOW(), ''),
  (3097, 3096, '生成账单', 'Generate',        3, 'property:fee:bill:generate', '',          '',                        '',                     1,  1, 1, 1, 1, NOW(), ''),
  (3098, 3096, '收费',     'Pay',             3, 'property:fee:bill:pay',     '',           '',                        '',                     2,  1, 1, 1, 1, NOW(), ''),
  (3099, 3096, '作废',     'Cancel',          3, 'property:fee:bill:cancel',  '',           '',                        '',                     3,  1, 1, 1, 1, NOW(), ''),
  (3110, 3090, '抄表管理', 'Meter Reading',   2, 'property:fee:meter:list',   'meter',      'property/fee/meter/index', 'ri:water-flash-line',   3,  1, 1, 1, 1, NOW(), ''),
  (3111, 3110, '新增水表', 'Create',          3, 'property:fee:meter:create', '',           '',                        '',                     1,  1, 1, 1, 1, NOW(), ''),
  (3112, 3110, '录入读数', 'Reading',         3, 'property:fee:meter:reading', '',          '',                        '',                     2,  1, 1, 1, 1, NOW(), ''),
  (3113, 3110, '生成账单', 'Generate',        3, 'property:fee:meter:generate', '',         '',                        '',                     3,  1, 1, 1, 1, NOW(), ''),
  (3120, 3090, '缴费记录', 'Payments',        2, 'property:fee:payment:list', 'payment',    'property/fee/payment/index', 'ri:wallet-line',      4,  1, 1, 1, 1, NOW(), ''),

  -- 设备
  (3130, 3000, '设备管理', 'Devices',         2, 'property:device:list',      'device',     'property/device/index',    'ri:cpu-line',           10, 1, 1, 1, 1, NOW(), '门禁/道闸/监控'),
  (3131, 3130, '新增',     'Create',          3, 'property:device:create',    '',           '',                        '',                     1,  1, 1, 1, 1, NOW(), ''),
  (3132, 3130, '修改',     'Update',          3, 'property:device:update',    '',           '',                        '',                     2,  1, 1, 1, 1, NOW(), ''),
  (3133, 3130, '删除',     'Delete',          3, 'property:device:delete',    '',           '',                        '',                     3,  1, 1, 1, 1, NOW(), ''),
  (3134, 3130, '导入',     'Import',          3, 'property:device:import',    '',           '',                        '',                     4,  1, 1, 1, 1, NOW(), ''),

  -- 门禁授权
  (3140, 3000, '门禁授权', 'Access Control',  2, 'property:access:list',      'access',     'property/access/index',    'ri:shield-user-line',   11, 1, 1, 1, 1, NOW(), '人脸/门禁卡/手机拨号下发'),
  (3141, 3140, '下发',     'Issue',           3, 'property:access:issue',     '',           '',                        '',                     1,  1, 1, 1, 1, NOW(), ''),
  (3142, 3140, '解除',     'Revoke',          3, 'property:access:revoke',    '',           '',                        '',                     2,  1, 1, 1, 1, NOW(), ''),
  (3143, 3140, '重试',     'Retry',           3, 'property:access:retry',     '',           '',                        '',                     3,  1, 1, 1, 1, NOW(), ''),
  (3144, 3140, '门禁卡',   'Card',            3, 'property:access:card',      '',           '',                        '',                     4,  1, 1, 1, 1, NOW(), ''),

  -- 平台侧：物业公司
  (3150, 0,    '物业公司', 'Property Companies', 2, 'property:company:list',  '/property/company', 'property/company/index', 'ri:briefcase-4-line', 6, 1, 1, 1, 1, NOW(), '平台侧：物业公司(租户)管理'),
  (3151, 3150, '新增',     'Create',          3, 'property:company:create',   '',           '',                        '',                     1,  1, 1, 1, 1, NOW(), ''),
  (3152, 3150, '修改',     'Update',          3, 'property:company:update',   '',           '',                        '',                     2,  1, 1, 1, 1, NOW(), ''),
  (3153, 3150, '删除',     'Delete',          3, 'property:company:delete',   '',           '',                        '',                     3,  1, 1, 1, 1, NOW(), ''),

  -- 物业概览
  (3160, 3000, '物业概览', 'Property Overview', 2, 'property:dashboard:view',  'dashboard',  'property/dashboard/index', 'ri:bar-chart-2-line',   12, 1, 1, 1, 1, NOW(), '');

-- ---------- 2. 字典 ----------
INSERT IGNORE INTO sys_dict_type (id, name, code, status, is_builtin, create_time, remark) VALUES
  (20, '房屋用途', 'pm_house_usage', 1, 1, NOW(), ''),
  (21, '房屋状态', 'pm_house_status', 1, 1, NOW(), ''),
  (22, '住户角色', 'pm_rel_role', 1, 1, NOW(), ''),
  (23, '证件类型', 'pm_id_type', 1, 1, NOW(), ''),
  (24, '访客状态', 'pm_visitor_status', 1, 1, NOW(), ''),
  (25, '车辆类型', 'pm_vehicle_type', 1, 1, NOW(), ''),
  (26, '支付方式', 'pm_pay_method', 1, 1, NOW(), ''),
  (27, '账单状态', 'pm_bill_status', 1, 1, NOW(), ''),
  (28, '收费项目类型', 'pm_fee_item_type', 1, 1, NOW(), ''),
  (29, '设备类型', 'pm_device_type', 1, 1, NOW(), ''),
  (30, '设备绑定层级', 'pm_bind_level', 1, 1, NOW(), ''),
  (31, '门禁下发类型', 'pm_grant_type', 1, 1, NOW(), ''),
  (32, '门禁下发状态', 'pm_grant_status', 1, 1, NOW(), ''),
  (33, '单位类型', 'pm_merchant_type', 1, 1, NOW(), ''),
  (34, '人员类型', 'pm_person_type', 1, 1, NOW(), ''),
  (35, '小区类型', 'pm_community_type', 1, 1, NOW(), ''),
  (36, '员工类型', 'pm_emp_type', 1, 1, NOW(), ''),
  (37, '楼栋类型', 'pm_building_type', 1, 1, NOW(), '');

INSERT IGNORE INTO sys_dict_data (id, dict_code, label, value, sort, tag_type, is_default, status, create_time) VALUES
  (2001, 'pm_house_usage', '住宅', '1', 1, 'primary', 1, 1, NOW()),
  (2002, 'pm_house_usage', '商业', '2', 2, 'warning', 0, 1, NOW()),
  (2003, 'pm_house_usage', '办公', '3', 3, 'success', 0, 1, NOW()),
  (2004, 'pm_house_usage', '车位', '4', 4, 'info', 0, 1, NOW()),
  (2005, 'pm_house_usage', '其他', '5', 5, 'info', 0, 1, NOW()),

  (2011, 'pm_house_status', '空置', '1', 1, 'info', 1, 1, NOW()),
  (2012, 'pm_house_status', '自住', '2', 2, 'success', 0, 1, NOW()),
  (2013, 'pm_house_status', '出租', '3', 3, 'warning', 0, 1, NOW()),
  (2014, 'pm_house_status', '装修中', '4', 4, 'primary', 0, 1, NOW()),
  (2015, 'pm_house_status', '停用', '5', 5, 'danger', 0, 1, NOW()),

  (2021, 'pm_rel_role', '业主', '1', 1, 'success', 1, 1, NOW()),
  (2022, 'pm_rel_role', '家人', '2', 2, 'primary', 0, 1, NOW()),
  (2023, 'pm_rel_role', '租户', '3', 3, 'warning', 0, 1, NOW()),

  (2031, 'pm_id_type', '身份证', '1', 1, '', 1, 1, NOW()),
  (2032, 'pm_id_type', '护照', '2', 2, '', 0, 1, NOW()),
  (2033, 'pm_id_type', '港澳通行证', '3', 3, '', 0, 1, NOW()),
  (2034, 'pm_id_type', '台胞证', '4', 4, '', 0, 1, NOW()),
  (2035, 'pm_id_type', '军官证', '5', 5, '', 0, 1, NOW()),
  (2036, 'pm_id_type', '其他', '9', 6, '', 0, 1, NOW()),

  (2041, 'pm_visitor_status', '待到访', '1', 1, 'warning', 1, 1, NOW()),
  (2042, 'pm_visitor_status', '在访', '2', 2, 'success', 0, 1, NOW()),
  (2043, 'pm_visitor_status', '已离场', '3', 3, 'info', 0, 1, NOW()),
  (2044, 'pm_visitor_status', '已过期', '4', 4, 'danger', 0, 1, NOW()),
  (2045, 'pm_visitor_status', '已取消', '5', 5, 'info', 0, 1, NOW()),

  (2051, 'pm_vehicle_type', '业主车', '1', 1, 'success', 1, 1, NOW()),
  (2052, 'pm_vehicle_type', '工作车', '2', 2, 'primary', 0, 1, NOW()),
  (2053, 'pm_vehicle_type', '临时车', '3', 3, 'warning', 0, 1, NOW()),
  (2054, 'pm_vehicle_type', '访客车', '4', 4, 'info', 0, 1, NOW()),

  (2061, 'pm_pay_method', '扫码盒子', '1', 1, 'primary', 0, 1, NOW()),
  (2062, 'pm_pay_method', '小程序支付', '2', 2, 'primary', 0, 1, NOW()),
  (2063, 'pm_pay_method', '现金', '3', 3, 'info', 1, 1, NOW()),
  (2064, 'pm_pay_method', '公众号支付', '4', 4, 'primary', 0, 1, NOW()),
  (2065, 'pm_pay_method', '二维码支付', '5', 5, 'primary', 0, 1, NOW()),
  (2066, 'pm_pay_method', '余额划扣', '6', 6, 'success', 0, 1, NOW()),
  (2067, 'pm_pay_method', '其他电子支付', '7', 7, 'info', 0, 1, NOW()),
  (2068, 'pm_pay_method', '押金抵扣', '8', 8, 'warning', 0, 1, NOW()),
  (2069, 'pm_pay_method', '银行转账', '9', 9, 'info', 0, 1, NOW()),

  (2071, 'pm_bill_status', '未缴', '1', 1, 'danger', 1, 1, NOW()),
  (2072, 'pm_bill_status', '部分缴', '2', 2, 'warning', 0, 1, NOW()),
  (2073, 'pm_bill_status', '已缴', '3', 3, 'success', 0, 1, NOW()),
  (2074, 'pm_bill_status', '已减免', '4', 4, 'info', 0, 1, NOW()),
  (2075, 'pm_bill_status', '已作废', '5', 5, 'info', 0, 1, NOW()),

  (2081, 'pm_fee_item_type', '物业费', '1', 1, 'primary', 1, 1, NOW()),
  (2082, 'pm_fee_item_type', '水费', '2', 2, 'success', 0, 1, NOW()),
  (2083, 'pm_fee_item_type', '电费', '3', 3, 'warning', 0, 1, NOW()),
  (2084, 'pm_fee_item_type', '停车费', '4', 4, 'info', 0, 1, NOW()),
  (2085, 'pm_fee_item_type', '其他费用', '5', 5, 'info', 0, 1, NOW()),

  (2091, 'pm_device_type', '门禁机', '1', 1, 'primary', 1, 1, NOW()),
  (2092, 'pm_device_type', '车辆道闸', '2', 2, 'success', 0, 1, NOW()),
  (2093, 'pm_device_type', '视频监控', '3', 3, 'warning', 0, 1, NOW()),
  (2094, 'pm_device_type', '人脸终端', '4', 4, 'primary', 0, 1, NOW()),
  (2095, 'pm_device_type', '可视对讲', '5', 5, 'info', 0, 1, NOW()),
  (2096, 'pm_device_type', '其他', '9', 6, 'info', 0, 1, NOW()),

  (2101, 'pm_bind_level', '小区', '1', 1, 'danger', 1, 1, NOW()),
  (2102, 'pm_bind_level', '区域', '2', 2, 'warning', 0, 1, NOW()),
  (2103, 'pm_bind_level', '楼栋', '3', 3, 'primary', 0, 1, NOW()),
  (2104, 'pm_bind_level', '单元', '4', 4, 'success', 0, 1, NOW()),
  (2105, 'pm_bind_level', '楼层', '5', 5, 'info', 0, 1, NOW()),

  (2111, 'pm_grant_type', '人脸', '1', 1, 'primary', 1, 1, NOW()),
  (2112, 'pm_grant_type', '门禁卡', '2', 2, 'success', 0, 1, NOW()),
  (2113, 'pm_grant_type', '手机拨号', '3', 3, 'warning', 0, 1, NOW()),

  (2121, 'pm_grant_status', '待下发', '1', 1, 'warning', 1, 1, NOW()),
  (2122, 'pm_grant_status', '下发中', '2', 2, 'primary', 0, 1, NOW()),
  (2123, 'pm_grant_status', '成功', '3', 3, 'success', 0, 1, NOW()),
  (2124, 'pm_grant_status', '失败', '4', 4, 'danger', 0, 1, NOW()),
  (2125, 'pm_grant_status', '已解除', '5', 5, 'info', 0, 1, NOW()),
  (2126, 'pm_grant_status', '过期解除', '6', 6, 'info', 0, 1, NOW()),

  (2131, 'pm_merchant_type', '办公', '1', 1, 'primary', 1, 1, NOW()),
  (2132, 'pm_merchant_type', '商铺', '2', 2, 'success', 0, 1, NOW()),
  (2133, 'pm_merchant_type', '学校', '3', 3, 'warning', 0, 1, NOW()),
  (2134, 'pm_merchant_type', '医疗', '4', 4, 'danger', 0, 1, NOW()),
  (2135, 'pm_merchant_type', '其他', '5', 5, 'info', 0, 1, NOW()),

  (2141, 'pm_person_type', '住户', '1', 1, 'primary', 1, 1, NOW()),
  (2142, 'pm_person_type', '物业人员', '2', 2, 'success', 0, 1, NOW()),
  (2143, 'pm_person_type', '访客', '3', 3, 'warning', 0, 1, NOW()),
  (2144, 'pm_person_type', '其他人员', '4', 4, 'info', 0, 1, NOW()),

  (2151, 'pm_community_type', '住宅', '1', 1, 'primary', 1, 1, NOW()),
  (2152, 'pm_community_type', '商业综合体', '2', 2, 'warning', 0, 1, NOW()),
  (2153, 'pm_community_type', '公寓', '3', 3, 'success', 0, 1, NOW()),
  (2154, 'pm_community_type', '园区', '4', 4, 'info', 0, 1, NOW()),
  (2155, 'pm_community_type', '其他', '5', 5, 'info', 0, 1, NOW()),

  (2161, 'pm_emp_type', '正式', '1', 1, 'success', 1, 1, NOW()),
  (2162, 'pm_emp_type', '外包', '2', 2, 'warning', 0, 1, NOW()),
  (2163, 'pm_emp_type', '临时', '3', 3, 'info', 0, 1, NOW()),
  (2164, 'pm_emp_type', '实习', '4', 4, 'info', 0, 1, NOW()),

  (2171, 'pm_building_type', '住宅', '1', 1, 'primary', 1, 1, NOW()),
  (2172, 'pm_building_type', '商业', '2', 2, 'warning', 0, 1, NOW()),
  (2173, 'pm_building_type', '配套', '3', 3, 'info', 0, 1, NOW()),
  (2174, 'pm_building_type', '车位', '4', 4, 'info', 0, 1, NOW());

-- ---------- 3. 内置物业角色 ----------
INSERT IGNORE INTO sys_role (id, name, code, sort, data_scope, status, is_builtin, tenant_id, create_time, remark) VALUES
  (310, '物业公司管理员', 'pm_company_admin',   10, 1, 1, 1, 1, NOW(), '物业公司租户的管理员：本租户全部小区'),
  (311, '项目经理',       'pm_project_manager', 11, 3, 1, 1, 1, NOW(), '本部门及以下数据'),
  (312, '客服管家',       'pm_customer_service', 12, 4, 1, 1, 1, NOW(), '仅本人数据'),
  (313, '收费员',         'pm_cashier',          13, 4, 1, 1, 1, NOW(), '仅本人数据'),
  (314, '门禁管理员',     'pm_access_admin',     14, 4, 1, 1, 1, NOW(), '仅本人数据');

-- 物业公司管理员：物业管理全量
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT 310, id FROM sys_menu WHERE id BETWEEN 3000 AND 3169 AND deleted = 0;

-- 项目经理：楼宇/住户/访客/其他人员/员工/车辆/单位/设备/门禁 + 概览
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT 311, id FROM sys_menu WHERE id BETWEEN 3000 AND 3000 AND deleted = 0;
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT 311, id FROM sys_menu WHERE id IN (
  3010,3011,3012,3013,3014,
  3020,3021,3022,3023,3024,
  3030,3031,3032,3033,3034,3035,
  3040,3041,3042,3043,3044,
  3050,3051,3052,3053,3054,
  3060,3061,3062,3063,3064,
  3070,3071,3072,3073,3074,
  3080,3081,3082,3083,
  3130,3131,3132,3133,3134,
  3140,3141,3142,3143,3144,
  3160
) AND deleted = 0;

-- 客服管家：住户/访客/其他人员
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT 312, id FROM sys_menu WHERE id IN (
  3000,3010,3030,3031,3032,3034,3035,3040,3041,3042,3043,3050,3051,3052,3070,3080,3160
) AND deleted = 0;

-- 收费员：收费项目/账单/抄表/缴费 + 住户只读
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT 313, id FROM sys_menu WHERE id IN (
  3000,3010,3030,3090,3091,3092,3093,3096,3097,3098,3099,3110,3111,3112,3113,3120,3160
) AND deleted = 0;

-- 门禁管理员：设备/门禁授权 + 人员只读
INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT 314, id FROM sys_menu WHERE id IN (
  3000,3010,3030,3040,3050,3130,3131,3132,3133,3134,3140,3141,3142,3143,3144,3160
) AND deleted = 0;
