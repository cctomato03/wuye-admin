-- =============================================================================
-- 000037_node_type_dict_and_address : 层级类型进字典 + 层级地址回填（数据部分）
--
-- 注意：DDL（full_address 列、同级名称索引）与地址回填都已移到应用启动时的幂等校验
-- （internal/bootstrap/schema.go），因为 MySQL 8 不支持 ADD COLUMN IF NOT EXISTS，
-- 而迁移脚本被拆成单条语句执行、无法安全使用会话变量/存储过程；
-- 这样即使上一次迁移执行到一半失败（列已存在），也能安全重跑本脚本。
-- =============================================================================

SET NAMES utf8mb4;

-- 1) 字典类型 + 字典项（幂等）
INSERT IGNORE INTO sys_dict_type (id, name, code, status, is_builtin, create_time, remark)
VALUES (9101, '小区层级类型', 'space_node_type', 1, 1, NOW(), '小区基础信息中层级可选类型：区域/楼栋/单元/楼层');

INSERT IGNORE INTO sys_dict_data (id, dict_code, label, value, sort, status, create_time)
VALUES
  (9111, 'space_node_type', '区域', '1', 1, 1, NOW()),
  (9112, 'space_node_type', '楼栋', '2', 2, 1, NOW()),
  (9113, 'space_node_type', '单元', '3', 3, 1, NOW()),
  (9114, 'space_node_type', '楼层', '4', 4, 1, NOW());

-- 2) 历史自定义/车位等类型统一归为楼栋
UPDATE pm_structure_node SET node_type = 2 WHERE node_type NOT IN (1, 2, 3, 4);

