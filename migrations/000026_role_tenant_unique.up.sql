-- =============================================================================
-- 000026_role_tenant_unique : 角色编码在"租户内"唯一，而不是全表唯一
--
-- 背景：物业公司（租户）创建时会克隆内置物业角色（pm_company_admin 等），
-- 多个租户必然存在同编码角色，原 uniq_code(code, deleted) 会导致建租户失败。
-- 多租户系统的正确约束是 (code, tenant_id, deleted)。
-- =============================================================================

SET NAMES utf8mb4;

ALTER TABLE sys_role
  DROP INDEX uniq_code,
  ADD UNIQUE KEY uk_tenant_code (code, tenant_id, deleted);
