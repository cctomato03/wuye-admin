-- =============================================================================
-- 000025_property_job_config : 物业相关参数与内置定时任务
-- =============================================================================

SET NAMES utf8mb4;

-- ---------- 参数 ----------
INSERT IGNORE INTO sys_config (id, name, config_key, value, value_type, is_builtin, is_public, tenant_id, create_time, remark) VALUES
  (1001, '门禁下发占位实现返回成功', 'property.access.stub_success', 'false', 'bool', 1, 0, 0, NOW(), '未对接第三方门禁平台时：false=下发记录标记失败（可重试），true=模拟成功'),
  (1002, '账单自动生成开关', 'property.bill.auto_generate', 'true', 'bool', 1, 0, 0, NOW(), '定时任务生成月度账单前先检查此开关'),
  (1003, '住户到期提前提醒天数', 'property.resident.expire_notice_days', '7', 'int', 1, 0, 0, NOW(), '住户/租户关系到期前提醒（预留）');

-- ---------- 内置定时任务（默认停用，需在「定时任务」页面手动启用） ----------
INSERT IGNORE INTO sys_job (id, name, handler, job_type, target, http_method, cron, params, concurrent, timeout_sec, retry_times, status, tenant_id, create_time, remark) VALUES
  (20, '住户关系到期处理', 'pmExpireResident',    1, '', 'POST', '0 10 1 * * *',    '{}', 1, 600, 1, 0, 0, NOW(), '把过期的住户/租户关系置为已到期，并撤销其门禁权限'),
  (21, '访客超时自动失效', 'pmExpireVisitor',     1, '', 'POST', '0 */10 * * * *',  '{}', 1, 300, 1, 0, 0, NOW(), '超过预约结束时间的访客置为已过期并撤销门禁'),
  (22, '其他人员有效期处理', 'pmExpireOther',     1, '', 'POST', '0 20 1 * * *',    '{}', 1, 600, 1, 0, 0, NOW(), '保洁/维修等临时人员到期后停用并撤销门禁'),
  (23, '门禁授权到期解除', 'pmExpireAccess',      1, '', 'POST', '0 */15 * * * *',  '{}', 1, 600, 1, 0, 0, NOW(), '调用第三方解除已到期的人脸/卡/手机拨号授权'),
  (24, '临时车辆到期处理', 'pmExpireVehicle',     1, '', 'POST', '0 30 1 * * *',    '{}', 1, 300, 1, 0, 0, NOW(), '临时车/工作车有效期结束后置为已过期'),
  (25, '月度物业费账单生成', 'pmGenerateBill',    1, '', 'POST', '0 0 2 1 * *',     '{}', 1, 1800, 1, 0, 0, NOW(), '每月 1 日凌晨为所有启用项目生成当月账单（按项目配置的生成日可手工触发）');
