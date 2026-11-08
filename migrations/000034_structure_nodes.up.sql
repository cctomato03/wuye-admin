-- =============================================================================
-- 000034_structure_nodes : 楼宇结构改为"自定义层级 + 房屋"
--
-- 背景：原来固定为 区域→楼栋→单元→楼层 四级，但很多小区没有区域/单元，
-- 甚至楼层划分方式也不同。改用通用层级树：
--   * pm_structure_node：小区为根（parent_id=0），任意层级，自行挂到上一级即可；
--   * pm_house.node_id：房屋挂在任意节点上（通常是叶子层级）；
--   * 历史数据（区域/楼栋/单元/楼层）按原 ID 迁移为节点，引用关系保持不变；
--   * 旧表保留（只读），新代码只写 pm_structure_node。
-- =============================================================================

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS pm_structure_node (
  id            BIGINT UNSIGNED NOT NULL,
  tenant_id     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id  BIGINT UNSIGNED NOT NULL,
  parent_id     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '上级节点，0=直接挂在小区下',
  code          VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '节点编码(小区内唯一)',
  name          VARCHAR(64)  NOT NULL COMMENT '节点名称，如 1栋 / 1单元 / 1层',
  node_type     TINYINT      NOT NULL DEFAULT 5 COMMENT '1区域 2楼栋 3单元 4楼层 5其他(自定义)',
  level_no      INT          NOT NULL DEFAULT 1 COMMENT '层级序号，1=小区下一级',
  path          VARCHAR(500) NOT NULL DEFAULT '0' COMMENT '祖级路径，如 0,100,101',
  building_type TINYINT      NOT NULL DEFAULT 1 COMMENT '楼栋类型(仅楼栋节点有意义) 1住宅2商业3配套4车位',
  total_floors  INT          NOT NULL DEFAULT 0 COMMENT '总层数(参考)',
  floor_no      INT          NOT NULL DEFAULT 0 COMMENT '楼层号(仅楼层节点)',
  house_count   INT          NOT NULL DEFAULT 0 COMMENT '节点下房屋数(含子节点，冗余)',
  sort          INT          NOT NULL DEFAULT 0,
  status        TINYINT      NOT NULL DEFAULT 1,
  create_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time   DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted       BIGINT       NOT NULL DEFAULT 0,
  remark        VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_comm_code (community_id, code, deleted),
  KEY idx_parent (community_id, parent_id, deleted),
  KEY idx_path (community_id, path(191)),
  KEY idx_level (community_id, level_no, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='小区层级节点(自定义)';

-- 房屋挂到节点
ALTER TABLE pm_house
  ADD COLUMN node_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属层级节点' AFTER floor_id,
  ADD KEY idx_comm_node (community_id, node_id, deleted);

-- ---------------------------- 历史数据迁移（保留原 ID，引用不变） ----------------------------
-- 1) 区域 → 节点（level 1）
INSERT IGNORE INTO pm_structure_node
  (id, tenant_id, community_id, parent_id, code, name, node_type, level_no, path, sort, status, create_by, create_time, remark)
SELECT r.id, r.tenant_id, r.community_id, 0,
       IF(r.code = '', CONCAT('RG', r.id), r.code), r.name, 1, 1, CONCAT('0,', r.id), r.sort, r.status, r.create_by, r.create_time, r.remark
FROM pm_region r WHERE r.deleted = 0;

-- 2) 楼栋 → 节点（挂在区域下；没有区域时为 level 1）
INSERT IGNORE INTO pm_structure_node
  (id, tenant_id, community_id, parent_id, code, name, node_type, level_no, path, building_type, total_floors, sort, status, create_by, create_time, remark)
SELECT b.id, b.tenant_id, b.community_id,
       IF(b.region_id = 0, 0, b.region_id),
       b.code, b.name, 2,
       IF(b.region_id = 0, 1, 2),
       IF(b.region_id = 0, CONCAT('0,', b.id), CONCAT('0,', b.region_id, ',', b.id)),
       b.building_type, b.total_floors, b.sort, b.status, b.create_by, b.create_time, b.remark
FROM pm_building b WHERE b.deleted = 0;

-- 3) 单元 → 节点（挂在楼栋下）
INSERT IGNORE INTO pm_structure_node
  (id, tenant_id, community_id, parent_id, code, name, node_type, level_no, path, total_floors, sort, status, create_by, create_time, remark)
SELECT u.id, u.tenant_id, u.community_id, u.building_id, u.code, u.name, 3,
       COALESCE(bn.level_no, 1) + 1,
       CONCAT(COALESCE(bn.path, CONCAT('0,', u.building_id)), ',', u.id),
       u.total_floors, u.sort, u.status, u.create_by, u.create_time, u.remark
FROM pm_unit u
LEFT JOIN pm_structure_node bn ON bn.id = u.building_id
WHERE u.deleted = 0;

-- 4) 楼层 → 节点（挂在单元下；无单元时挂楼栋）
INSERT IGNORE INTO pm_structure_node
  (id, tenant_id, community_id, parent_id, code, name, node_type, level_no, path, floor_no, sort, status, create_by, create_time)
SELECT f.id, f.tenant_id, f.community_id,
       IF(f.unit_id = 0, f.building_id, f.unit_id),
       CONCAT('FL', f.id), f.name, 4,
       COALESCE(pn.level_no, 1) + 1,
       CONCAT(COALESCE(pn.path, CONCAT('0,', IF(f.unit_id = 0, f.building_id, f.unit_id))), ',', f.id),
       f.floor_no, f.sort, 1, f.create_by, f.create_time
FROM pm_floor f
LEFT JOIN pm_structure_node pn ON pn.id = IF(f.unit_id = 0, f.building_id, f.unit_id)
WHERE f.deleted = 0;

-- 5) 房屋 → 挂到最末级节点（楼层优先，其次单元/楼栋/区域）
UPDATE pm_house h
SET h.node_id = COALESCE(
  (SELECT n.id FROM pm_structure_node n WHERE n.id = h.floor_id AND n.deleted = 0 LIMIT 1),
  (SELECT n.id FROM pm_structure_node n WHERE n.id = h.unit_id AND n.deleted = 0 LIMIT 1),
  (SELECT n.id FROM pm_structure_node n WHERE n.id = h.building_id AND n.deleted = 0 LIMIT 1),
  0
)
WHERE h.node_id = 0 AND h.deleted = 0;

-- 6) 设备支持绑定到自定义节点
ALTER TABLE pm_device
  ADD COLUMN bind_node_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '绑定的层级节点(优先于 bind_level/bind_id)' AFTER bind_name,
  ADD KEY idx_comm_node (community_id, bind_node_id, deleted);
