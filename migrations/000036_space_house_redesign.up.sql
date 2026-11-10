-- =============================================================================
-- 000036_space_house_redesign : 小区基础信息（层级 + 房屋）重新设计
--
-- 设计目标：
--   * 一套模型同时支持：普通住宅（楼栋→单元→楼层→房屋）、别墅（一栋一套）、
--     商业楼/商铺、办公楼、地下车库/车位、配套设施；
--   * 层级节点（pm_structure_node）承载"楼栋/单元/楼层/自定义"的完整属性，
--     房屋（pm_house）承载面积、用途、产权、临街等经营属性；
--   * 丢弃历史的 pm_region / pm_building / pm_unit / pm_floor（数据已在 000034 迁移为节点）；
--   * 菜单合并为「小区管理 → 基础信息管理」一个页面。
-- =============================================================================

SET NAMES utf8mb4;

-- ---------------------------- 层级节点：补齐楼栋/空间属性 ----------------------------
ALTER TABLE pm_structure_node
  ADD COLUMN category        TINYINT      NOT NULL DEFAULT 9 COMMENT '空间类别 1住宅楼 2别墅 3商业楼 4办公楼 5车库/车位 6配套设施 9其他' AFTER node_type,
  ADD COLUMN floors_above    INT          NOT NULL DEFAULT 0 COMMENT '地上层数' AFTER total_floors,
  ADD COLUMN floors_under    INT          NOT NULL DEFAULT 0 COMMENT '地下层数' AFTER floors_above,
  ADD COLUMN unit_count      INT          NOT NULL DEFAULT 0 COMMENT '单元数' AFTER floors_under,
  ADD COLUMN elevator_count  INT          NOT NULL DEFAULT 0 COMMENT '电梯数' AFTER unit_count,
  ADD COLUMN build_area      DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '建筑面积(㎡)' AFTER elevator_count,
  ADD COLUMN build_year      INT          NOT NULL DEFAULT 0 COMMENT '建成年份' AFTER build_area,
  ADD COLUMN structure_type  VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '建筑结构：框架/砖混/钢结构…' AFTER build_year,
  ADD COLUMN delivery_date   DATE         NULL COMMENT '交付日期' AFTER structure_type,
  ADD COLUMN address         VARCHAR(255) NOT NULL DEFAULT '' COMMENT '位置/门牌描述' AFTER delivery_date;

-- 历史节点回填类别
UPDATE pm_structure_node SET category = 1 WHERE node_type = 2 AND building_type = 1;
UPDATE pm_structure_node SET category = 3 WHERE node_type = 2 AND building_type = 2;
UPDATE pm_structure_node SET category = 6 WHERE node_type = 2 AND building_type = 3;
UPDATE pm_structure_node SET category = 5 WHERE node_type = 2 AND building_type = 4;
UPDATE pm_structure_node SET category = 9 WHERE category = 9 AND node_type = 1;
UPDATE pm_structure_node SET floors_above = total_floors WHERE total_floors > 0 AND floors_above = 0;

-- ---------------------------- 房屋：补齐经营属性 ----------------------------
ALTER TABLE pm_house
  ADD COLUMN area_share      DECIMAL(10,2) NOT NULL DEFAULT 0 COMMENT '公摊面积(㎡)' AFTER area_use,
  ADD COLUMN ceiling_height  DECIMAL(5,2)  NOT NULL DEFAULT 0 COMMENT '层高(m)' AFTER area_share,
  ADD COLUMN property_right  TINYINT       NOT NULL DEFAULT 1 COMMENT '产权性质 1商品房 2安置房 3公房 4自建 9其他' AFTER area_share,
  ADD COLUMN is_street       TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '是否临街(商铺)' AFTER property_right,
  ADD COLUMN shop_width      DECIMAL(8,2)  NOT NULL DEFAULT 0 COMMENT '门面宽(m)' AFTER is_street,
  ADD COLUMN is_sale         TINYINT(1)    NOT NULL DEFAULT 1 COMMENT '是否可售' AFTER shop_width,
  ADD COLUMN facilities      VARCHAR(500)  NOT NULL DEFAULT '' COMMENT '附属设施(水电燃气/空调/消防等)' AFTER is_sale,
  ADD COLUMN handover_status TINYINT       NOT NULL DEFAULT 1 COMMENT '交付状态 1已交付 2未交付 3自持' AFTER facilities;

-- ---------------------------- 菜单合并为一个页面 ----------------------------
UPDATE sys_menu SET name = '基础信息管理', name_en = 'Spaces & Houses', path = 'space',
       component = 'property/space/index', icon = 'ri:layout-grid-line', sort = 1,
       remark = '小区层级（楼栋/单元/楼层）与房屋一体化管理，支持导入导出'
WHERE id = 3020;

UPDATE sys_menu SET deleted = UNIX_TIMESTAMP(NOW(3)) * 1000 WHERE id = 3270;

-- ---------------------------- 丢弃旧结构表（数据已迁移到 pm_structure_node） ----------------------------
DROP TABLE IF EXISTS pm_region;
DROP TABLE IF EXISTS pm_building;
DROP TABLE IF EXISTS pm_unit;
DROP TABLE IF EXISTS pm_floor;
