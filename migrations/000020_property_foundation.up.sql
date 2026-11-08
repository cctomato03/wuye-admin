-- =============================================================================
-- 000020_property_foundation : 物业管理基础域（小区 / 行政区划 / 楼宇结构 / 小区授权）
-- 约定：
--   1) 所有业务表包含 create_by/create_time/update_by/update_time/deleted/remark；
--   2) 不做物理删除，deleted=0 表示有效，非 0 为删除时间戳（毫秒）；
--   3) 唯一键一律带 deleted，软删除后可重建同编码数据；
--   4) 业务表索引以 (tenant_id, community_id) 打头，与"按租户+授权小区过滤"的查询形态对齐。
-- =============================================================================

SET NAMES utf8mb4;

-- ---------------------------- 行政区划 ----------------------------
CREATE TABLE IF NOT EXISTS pm_district (
  id          BIGINT UNSIGNED NOT NULL COMMENT '主键',
  parent_id   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '上级区划,0为顶级',
  code        VARCHAR(20)  NOT NULL COMMENT '行政区划代码(GB/T 2260)',
  name        VARCHAR(64)  NOT NULL COMMENT '名称',
  level       TINYINT      NOT NULL DEFAULT 1 COMMENT '层级 1省 2市 3区县 4街道',
  full_name   VARCHAR(200) NOT NULL DEFAULT '' COMMENT '全称,如 广东省深圳市南山区',
  sort        INT          NOT NULL DEFAULT 0,
  status      TINYINT      NOT NULL DEFAULT 1,
  create_time DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted     BIGINT       NOT NULL DEFAULT 0,
  remark      VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_code (code, deleted),
  KEY idx_parent (parent_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='行政区划';

-- ---------------------------- 小区 ----------------------------
CREATE TABLE IF NOT EXISTS pm_community (
  id             BIGINT UNSIGNED NOT NULL COMMENT '主键',
  tenant_id      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属物业公司(租户)',
  code           VARCHAR(64)  NOT NULL COMMENT '小区唯一编码',
  name           VARCHAR(100) NOT NULL COMMENT '小区名称',
  short_name     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '简称',
  community_type TINYINT      NOT NULL DEFAULT 1 COMMENT '类型 1住宅 2商业综合体 3公寓 4园区 5其他',
  province       VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '省',
  city           VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '市',
  district       VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '区县',
  district_code  VARCHAR(20)  NOT NULL DEFAULT '' COMMENT '行政区划代码',
  address        VARCHAR(255) NOT NULL DEFAULT '' COMMENT '详细地址',
  longitude      DECIMAL(10,6) NOT NULL DEFAULT 0 COMMENT '经度',
  latitude       DECIMAL(10,6) NOT NULL DEFAULT 0 COMMENT '纬度',
  total_area     DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '占地/建筑面积(㎡)',
  green_area     DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '绿化面积(㎡)',
  building_count INT          NOT NULL DEFAULT 0 COMMENT '楼栋数(冗余统计)',
  house_count    INT          NOT NULL DEFAULT 0 COMMENT '房屋数(冗余统计)',
  resident_count INT          NOT NULL DEFAULT 0 COMMENT '住户数(冗余统计)',
  developer      VARCHAR(100) NOT NULL DEFAULT '' COMMENT '开发商',
  property_company VARCHAR(100) NOT NULL DEFAULT '' COMMENT '物业公司名称(冗余展示)',
  manager_name   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '物业负责人',
  manager_phone  VARCHAR(20)  NOT NULL DEFAULT '' COMMENT '负责人电话',
  service_phone  VARCHAR(20)  NOT NULL DEFAULT '' COMMENT '客服电话',
  handover_date  DATE         NULL COMMENT '交付日期',
  status         TINYINT      NOT NULL DEFAULT 1 COMMENT '状态 1正常 0停用',
  sort           INT          NOT NULL DEFAULT 0,
  create_by      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time    DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted        BIGINT       NOT NULL DEFAULT 0,
  remark         VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_tenant_code (tenant_id, code, deleted),
  KEY idx_tenant_district (tenant_id, district_code, deleted),
  KEY idx_tenant_name (tenant_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='小区';

-- ---------------------------- 用户-小区授权 ----------------------------
-- 小区级数据权限的唯一入口：物业员工只能看到这里授权过的小区。
CREATE TABLE IF NOT EXISTS pm_user_community (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  user_id      BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  community_id BIGINT UNSIGNED NOT NULL COMMENT '小区ID',
  is_default   TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否默认小区',
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_user_comm (user_id, community_id, deleted),
  KEY idx_comm (community_id, deleted),
  KEY idx_tenant_user (tenant_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户-小区授权';

-- ---------------------------- 区域（期/片区） ----------------------------
CREATE TABLE IF NOT EXISTS pm_region (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id BIGINT UNSIGNED NOT NULL,
  name         VARCHAR(64)  NOT NULL COMMENT '区域名称/期数',
  code         VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '区域编码',
  region_type  TINYINT      NOT NULL DEFAULT 1 COMMENT '1住宅区 2商业区 3配套区',
  sort         INT          NOT NULL DEFAULT 0,
  status       TINYINT      NOT NULL DEFAULT 1,
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_comm_code (community_id, code, deleted),
  KEY idx_comm (tenant_id, community_id, deleted),
  KEY idx_comm_sort (community_id, sort)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='小区区域/期';

-- ---------------------------- 楼栋 ----------------------------
CREATE TABLE IF NOT EXISTS pm_building (
  id            BIGINT UNSIGNED NOT NULL,
  tenant_id     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id  BIGINT UNSIGNED NOT NULL,
  region_id     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属区域',
  code          VARCHAR(64)  NOT NULL COMMENT '楼栋编码(小区内唯一)',
  name          VARCHAR(64)  NOT NULL COMMENT '楼栋名称',
  building_type TINYINT      NOT NULL DEFAULT 1 COMMENT '1住宅 2商业 3配套 4车位',
  total_floors  INT          NOT NULL DEFAULT 0 COMMENT '总层数(含地下)',
  under_floors  INT          NOT NULL DEFAULT 0 COMMENT '地下层数',
  total_units   INT          NOT NULL DEFAULT 0 COMMENT '单元数(冗余)',
  total_houses  INT          NOT NULL DEFAULT 0 COMMENT '房屋数(冗余)',
  build_year    INT          NOT NULL DEFAULT 0 COMMENT '建成年份',
  address       VARCHAR(255) NOT NULL DEFAULT '' COMMENT '楼栋地址描述',
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
  KEY idx_comm_region (tenant_id, community_id, region_id, deleted),
  KEY idx_comm_type (community_id, building_type, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='楼栋';

-- ---------------------------- 单元 ----------------------------
CREATE TABLE IF NOT EXISTS pm_unit (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id BIGINT UNSIGNED NOT NULL,
  building_id  BIGINT UNSIGNED NOT NULL,
  code         VARCHAR(64)  NOT NULL COMMENT '单元编码(小区内唯一)',
  name         VARCHAR(64)  NOT NULL COMMENT '单元名称',
  total_floors INT          NOT NULL DEFAULT 0,
  houses_per_floor INT      NOT NULL DEFAULT 0 COMMENT '每层户数(参考)',
  elevator     TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否有电梯',
  sort         INT          NOT NULL DEFAULT 0,
  status       TINYINT      NOT NULL DEFAULT 1,
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_comm_code (community_id, code, deleted),
  KEY idx_comm_building (tenant_id, community_id, building_id, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='单元';

-- ---------------------------- 楼层 ----------------------------
CREATE TABLE IF NOT EXISTS pm_floor (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id BIGINT UNSIGNED NOT NULL,
  building_id  BIGINT UNSIGNED NOT NULL,
  unit_id      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0表示无单元',
  floor_no     INT          NOT NULL DEFAULT 1 COMMENT '楼层号,负数表示地下',
  name         VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '楼层名称',
  house_count  INT          NOT NULL DEFAULT 0 COMMENT '本层房屋数(冗余)',
  sort         INT          NOT NULL DEFAULT 0,
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_unit_floor (community_id, building_id, unit_id, floor_no, deleted),
  KEY idx_comm_building (tenant_id, community_id, building_id, floor_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='楼层';

-- ---------------------------- 房屋类型 ----------------------------
CREATE TABLE IF NOT EXISTS pm_house_type (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0表示公司级共享类型',
  code         VARCHAR(64)  NOT NULL DEFAULT '',
  name         VARCHAR(64)  NOT NULL COMMENT '类型名称,如 三室两厅',
  layout       VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '户型描述',
  usage_type   TINYINT      NOT NULL DEFAULT 1 COMMENT '1住宅 2商业 3办公',
  sort         INT          NOT NULL DEFAULT 0,
  status       TINYINT      NOT NULL DEFAULT 1,
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_tenant (tenant_id, community_id, deleted),
  KEY idx_name (tenant_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='房屋类型';

-- ---------------------------- 房屋 ----------------------------
CREATE TABLE IF NOT EXISTS pm_house (
  id             BIGINT UNSIGNED NOT NULL,
  tenant_id      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id   BIGINT UNSIGNED NOT NULL,
  region_id      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  building_id    BIGINT UNSIGNED NOT NULL,
  unit_id        BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0表示无单元',
  floor_id       BIGINT UNSIGNED NOT NULL DEFAULT 0,
  floor_no       INT          NOT NULL DEFAULT 1 COMMENT '楼层号(冗余,便于筛选)',
  house_type_id  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  code           VARCHAR(64)  NOT NULL COMMENT '房屋唯一编码(小区内)',
  house_no       VARCHAR(64)  NOT NULL COMMENT '房号,如 101',
  full_name      VARCHAR(200) NOT NULL DEFAULT '' COMMENT '全称,如 1栋1单元101',
  usage_type     TINYINT      NOT NULL DEFAULT 1 COMMENT '1住宅 2商业 3办公 4车位 5其他',
  area_build     DECIMAL(10,2) NOT NULL DEFAULT 0 COMMENT '建筑面积(㎡)',
  area_use       DECIMAL(10,2) NOT NULL DEFAULT 0 COMMENT '套内面积(㎡)',
  orientation    VARCHAR(20)  NOT NULL DEFAULT '' COMMENT '朝向',
  layout         VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '户型',
  decoration     TINYINT      NOT NULL DEFAULT 0 COMMENT '0未知 1毛坯 2简装 3精装',
  house_status   TINYINT      NOT NULL DEFAULT 1 COMMENT '1空置 2自住 3出租 4装修中 5停用',
  owner_count    INT          NOT NULL DEFAULT 0 COMMENT '在册业主数(冗余)',
  resident_count INT          NOT NULL DEFAULT 0 COMMENT '在册住户数(冗余)',
  delivery_date  DATE         NULL COMMENT '交付日期',
  sort           INT          NOT NULL DEFAULT 0,
  create_by      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time    DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted        BIGINT       NOT NULL DEFAULT 0,
  remark         VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_comm_code (community_id, code, deleted),
  KEY idx_comm_building (tenant_id, community_id, building_id, deleted),
  KEY idx_comm_unit (community_id, unit_id, deleted),
  KEY idx_comm_floor (community_id, floor_id, deleted),
  KEY idx_comm_usage (community_id, usage_type, house_status, deleted),
  KEY idx_fullname (community_id, full_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='房屋';

-- ---------------------------- 行政区划种子（精简：常用层级示例） ----------------------------
INSERT IGNORE INTO pm_district (id, parent_id, code, name, level, full_name, sort, status, create_time) VALUES
  (1, 0, '440000', '广东省', 1, '广东省', 1, 1, NOW()),
  (2, 1, '440300', '深圳市', 2, '广东省深圳市', 1, 1, NOW()),
  (3, 2, '440305', '南山区', 3, '广东省深圳市南山区', 1, 1, NOW()),
  (4, 2, '440304', '福田区', 3, '广东省深圳市福田区', 2, 1, NOW()),
  (5, 2, '440306', '宝安区', 3, '广东省深圳市宝安区', 3, 1, NOW()),
  (6, 2, '440307', '龙岗区', 3, '广东省深圳市龙岗区', 4, 1, NOW()),
  (7, 0, '110000', '北京市', 1, '北京市', 2, 1, NOW()),
  (8, 7, '110100', '北京市', 2, '北京市', 1, 1, NOW()),
  (9, 8, '110105', '朝阳区', 3, '北京市朝阳区', 1, 1, NOW()),
  (10, 8, '110108', '海淀区', 3, '北京市海淀区', 2, 1, NOW()),
  (11, 0, '310000', '上海市', 1, '上海市', 3, 1, NOW()),
  (12, 11, '310100', '上海市', 2, '上海市', 1, 1, NOW()),
  (13, 12, '310101', '黄浦区', 3, '上海市黄浦区', 1, 1, NOW()),
  (14, 12, '310115', '浦东新区', 3, '上海市浦东新区', 2, 1, NOW());
