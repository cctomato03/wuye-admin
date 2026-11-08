-- =============================================================================
-- 000027_district_five_levels : 行政区划五级（省/市/区县/街道/社区）+ 小区地址扩展
--
-- 说明：
--   * 全量行政区划数据（约 66 万条）不写在 SQL 里，由服务启动时从内嵌数据集导入
--     （internal/service/data/districts.json.gz），也可调用 POST /property/districts/sync 重新导入；
--   * 这里只做表结构扩展与索引优化，并清掉 000020 里的示例区划，避免与真实编码混用。
-- =============================================================================

SET NAMES utf8mb4;

-- 小区：补齐到街道与社区
ALTER TABLE pm_community
  ADD COLUMN town         VARCHAR(64) NOT NULL DEFAULT '' COMMENT '街道/乡镇' AFTER district,
  ADD COLUMN village      VARCHAR(64) NOT NULL DEFAULT '' COMMENT '社区/村' AFTER town,
  ADD COLUMN street_code  VARCHAR(20) NOT NULL DEFAULT '' COMMENT '街道编码(9位)' AFTER district_code,
  ADD COLUMN village_code VARCHAR(20) NOT NULL DEFAULT '' COMMENT '社区编码(12位)' AFTER street_code;

-- 行政区划：级联查询与搜索索引
ALTER TABLE pm_district
  ADD KEY idx_parent_level (parent_id, level),
  ADD KEY idx_level_name (level, name);

-- 清理 000020 中的示例区划（真实数据由服务启动时导入）
DELETE FROM pm_district WHERE id BETWEEN 1 AND 14;
