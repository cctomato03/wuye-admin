-- =============================================================================
-- 000031_role_community_scope : 角色绑定可管理小区 + 物业公司管理员角色权限重建
--
-- 1) sys_role 增加 community_scope：1=本物业公司全部小区（动态覆盖新增小区），2=指定小区；
-- 2) 新增 sys_role_community（角色可管理的小区）；
-- 3) 物业公司管理员角色（pm_company_admin）：拥有除「系统监控 / 系统设置 / 租户管理」外的全部菜单，
--    公司范围内可继续创建角色、用户、部门、岗位（这些都在「系统管理」下），但不能管理公司与小区；
--    菜单管理属于平台级配置，同样不下放（避免租户改到全平台菜单）。
-- 4) 历史物业公司角色统一置为「全部小区」，避免升级后管理员看不到数据。
-- =============================================================================

SET NAMES utf8mb4;

ALTER TABLE sys_role
  ADD COLUMN community_scope TINYINT NOT NULL DEFAULT 2 COMMENT '可管理小区 1本租户全部 2指定小区' AFTER data_scope;

CREATE TABLE IF NOT EXISTS sys_role_community (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  role_id      BIGINT UNSIGNED NOT NULL,
  community_id BIGINT UNSIGNED NOT NULL,
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_role_comm (role_id, community_id, deleted),
  KEY idx_comm (community_id, deleted),
  KEY idx_tenant_role (tenant_id, role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='角色可管理的小区';

-- 历史物业角色：默认管理本物业公司全部小区
UPDATE sys_role SET community_scope = 1 WHERE code LIKE 'pm\_%';

-- 物业公司管理员：重建菜单授权（排除 系统监控 / 系统设置 / 租户管理 三棵子树）
DELETE rm FROM sys_role_menu rm JOIN sys_role r ON r.id = rm.role_id WHERE r.code = 'pm_company_admin';

INSERT IGNORE INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id
FROM sys_role r
JOIN sys_menu m
  ON m.deleted = 0
 AND m.id NOT IN (1030, 1080, 1200, 1300)
 AND m.parent_id NOT IN (1030, 1080, 1200, 1300)
WHERE r.code = 'pm_company_admin';
