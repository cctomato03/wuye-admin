-- =============================================================================
-- 000023_property_fee : 收费域（收费项目 / 收费标准 / 水电表 / 抄表 / 账单 / 缴费流水 / 账户）
-- 大表说明：
--   pm_bill 是增长最快的表（小区数 × 房屋数 × 项目数 × 月数），
--   因此以 (community_id, bill_month, status) 与 (community_id, house_id, status) 双索引支撑
--   "按月出账/按房屋查欠费/催缴统计"三类主查询；批量写入统一 CreateInBatches。
-- =============================================================================

SET NAMES utf8mb4;

-- ---------------------------- 收费项目 ----------------------------
CREATE TABLE IF NOT EXISTS pm_fee_item (
  id            BIGINT UNSIGNED NOT NULL,
  tenant_id     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id  BIGINT UNSIGNED NOT NULL,
  code          VARCHAR(64)  NOT NULL COMMENT '项目编码(小区内唯一)',
  name          VARCHAR(100) NOT NULL COMMENT '项目名称',
  item_type     TINYINT      NOT NULL DEFAULT 1 COMMENT '1物业费 2水费 3电费 4停车费 5其他',
  calc_mode     TINYINT      NOT NULL DEFAULT 1 COMMENT '1按面积·月 2按面积·天 3按用量 4固定金额 5按套',
  unit_price    DECIMAL(12,4) NOT NULL DEFAULT 0 COMMENT '默认单价',
  unit          VARCHAR(20)  NOT NULL DEFAULT '元/㎡·月' COMMENT '计价单位(展示)',
  billing_cycle TINYINT      NOT NULL DEFAULT 1 COMMENT '1月 2季 3年 4按次',
  generate_day  INT          NOT NULL DEFAULT 1 COMMENT '每月生成日(1-28)',
  house_usage   TINYINT      NOT NULL DEFAULT 0 COMMENT '适用房屋用途 0全部 1住宅 2商业 3办公',
  charge_object TINYINT      NOT NULL DEFAULT 1 COMMENT '1按房屋 2按单位(商铺/办公) 3按车位',
  late_fee_rate DECIMAL(8,4) NOT NULL DEFAULT 0 COMMENT '滞纳金日利率(0=不收)',
  due_days      INT          NOT NULL DEFAULT 30 COMMENT '账期天数(生成日+天数=到期日)',
  status        TINYINT      NOT NULL DEFAULT 1,
  sort          INT          NOT NULL DEFAULT 0,
  create_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time   DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted       BIGINT       NOT NULL DEFAULT 0,
  remark        VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_comm_code (community_id, code, deleted),
  KEY idx_comm_type (tenant_id, community_id, item_type, status, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='收费项目';

-- ---------------------------- 收费标准（差异化单价） ----------------------------
CREATE TABLE IF NOT EXISTS pm_fee_standard (
  id             BIGINT UNSIGNED NOT NULL,
  tenant_id      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id   BIGINT UNSIGNED NOT NULL,
  fee_item_id    BIGINT UNSIGNED NOT NULL,
  house_usage    TINYINT      NOT NULL DEFAULT 0 COMMENT '0全部 1住宅 2商业 3办公',
  house_type_id  BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0=不区分房屋类型',
  building_id    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0=不区分楼栋',
  unit_price     DECIMAL(12,4) NOT NULL DEFAULT 0,
  effective_from DATE         NULL,
  effective_to   DATE         NULL,
  status         TINYINT      NOT NULL DEFAULT 1,
  create_by      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time    DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted        BIGINT       NOT NULL DEFAULT 0,
  remark         VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_item (tenant_id, community_id, fee_item_id, house_usage, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='收费标准';

-- ---------------------------- 水电表 ----------------------------
CREATE TABLE IF NOT EXISTS pm_meter (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id BIGINT UNSIGNED NOT NULL,
  house_id     BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '安装房屋',
  house_name   VARCHAR(200) NOT NULL DEFAULT '',
  merchant_id  BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '单位表(可空)',
  meter_type   TINYINT      NOT NULL DEFAULT 1 COMMENT '1水表 2电表 3燃气表 4热水表',
  meter_no     VARCHAR(64)  NOT NULL COMMENT '表号(小区内唯一)',
  ratio        DECIMAL(8,2) NOT NULL DEFAULT 1 COMMENT '倍率',
  init_reading DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT '初始读数',
  last_reading DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT '最近读数(冗余,抄表时更新)',
  last_read_time DATETIME   NULL,
  install_date DATE         NULL,
  status       TINYINT      NOT NULL DEFAULT 1,
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_comm_no (community_id, meter_no, deleted),
  KEY idx_comm_house (tenant_id, community_id, house_id, deleted),
  KEY idx_comm_type (community_id, meter_type, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='水电表';

-- ---------------------------- 抄表记录 ----------------------------
CREATE TABLE IF NOT EXISTS pm_meter_reading (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id BIGINT UNSIGNED NOT NULL,
  meter_id     BIGINT UNSIGNED NOT NULL,
  house_id     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  house_name   VARCHAR(200) NOT NULL DEFAULT '',
  period       CHAR(6)      NOT NULL COMMENT '账期 YYYYMM',
  prev_reading DECIMAL(14,2) NOT NULL DEFAULT 0,
  curr_reading DECIMAL(14,2) NOT NULL DEFAULT 0,
  usage_amount DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT '用量=(本期-上期)×倍率',
  unit_price   DECIMAL(12,4) NOT NULL DEFAULT 0,
  amount       DECIMAL(12,2) NOT NULL DEFAULT 0,
  read_time    DATETIME     NULL,
  reader_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  reader_name  VARCHAR(64)  NOT NULL DEFAULT '',
  bill_id      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '已生成账单',
  status       TINYINT      NOT NULL DEFAULT 1 COMMENT '1已抄表 2已出账 3已作废',
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_meter_period (meter_id, period, deleted),
  KEY idx_comm_period (tenant_id, community_id, period),
  KEY idx_comm_house (community_id, house_id, period)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='抄表记录';

-- ---------------------------- 账单 ----------------------------
CREATE TABLE IF NOT EXISTS pm_bill (
  id            BIGINT UNSIGNED NOT NULL,
  tenant_id     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id  BIGINT UNSIGNED NOT NULL,
  bill_no       VARCHAR(40)  NOT NULL COMMENT '账单号',
  fee_item_id   BIGINT UNSIGNED NOT NULL,
  fee_item_name VARCHAR(100) NOT NULL DEFAULT '',
  item_type     TINYINT      NOT NULL DEFAULT 1 COMMENT '1物业费 2水费 3电费 4停车费 5其他',
  house_id      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '房屋(按房屋收费)',
  house_name    VARCHAR(200) NOT NULL DEFAULT '',
  building_id   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  merchant_id   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '单位(按单位收费)',
  merchant_name VARCHAR(100) NOT NULL DEFAULT '',
  resident_id   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '付款人(冗余,便于查询)',
  resident_name VARCHAR(64)  NOT NULL DEFAULT '',
  house_usage   TINYINT      NOT NULL DEFAULT 1 COMMENT '计费时的房屋用途',
  bill_month    CHAR(6)      NOT NULL COMMENT '账期 YYYYMM',
  period_start  DATE         NULL,
  period_end    DATE         NULL,
  quantity      DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT '计费数量(面积/用量/套数)',
  unit_price    DECIMAL(12,4) NOT NULL DEFAULT 0,
  amount        DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '应收金额',
  discount_amount DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '减免金额',
  late_fee      DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '滞纳金',
  paid_amount   DECIMAL(12,2) NOT NULL DEFAULT 0 COMMENT '已收金额',
  status        TINYINT      NOT NULL DEFAULT 1 COMMENT '1未缴 2部分缴 3已缴 4已减免 5已作废',
  due_date      DATE         NULL COMMENT '到期日',
  pay_time      DATETIME     NULL COMMENT '结清时间',
  source        TINYINT      NOT NULL DEFAULT 1 COMMENT '1批量生成 2抄表生成 3手工录入',
  meter_reading_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time   DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted       BIGINT       NOT NULL DEFAULT 0,
  remark        VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_bill_no (bill_no, deleted),
  KEY idx_comm_month_status (tenant_id, community_id, bill_month, status),
  KEY idx_comm_house (community_id, house_id, status),
  KEY idx_comm_resident (community_id, resident_id, status),
  KEY idx_comm_item (community_id, fee_item_id, bill_month),
  KEY idx_due (status, due_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='账单';

-- ---------------------------- 缴费流水 ----------------------------
CREATE TABLE IF NOT EXISTS pm_payment (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id BIGINT UNSIGNED NOT NULL,
  pay_no       VARCHAR(40)  NOT NULL COMMENT '支付流水号',
  bill_id      BIGINT UNSIGNED NOT NULL,
  bill_no      VARCHAR(40)  NOT NULL DEFAULT '',
  house_id     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  house_name   VARCHAR(200) NOT NULL DEFAULT '',
  resident_id  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  resident_name VARCHAR(64) NOT NULL DEFAULT '',
  pay_method   TINYINT      NOT NULL DEFAULT 3 COMMENT '1扫码盒子 2小程序 3现金 4公众号 5二维码 6余额划扣 7其他电子 8押金抵扣 9银行转账',
  amount       DECIMAL(12,2) NOT NULL DEFAULT 0,
  status       TINYINT      NOT NULL DEFAULT 1 COMMENT '1成功 2失败 3待确认 4已退款',
  pay_time     DATETIME     NULL,
  operator_id  BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '收款人',
  operator_name VARCHAR(64) NOT NULL DEFAULT '',
  trade_no     VARCHAR(100) NOT NULL DEFAULT '' COMMENT '第三方交易号',
  channel      VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '支付渠道标识(预留)',
  channel_resp VARCHAR(1000) NOT NULL DEFAULT '' COMMENT '渠道返回(预留)',
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_pay_no (pay_no, deleted),
  KEY idx_comm_time (tenant_id, community_id, pay_time),
  KEY idx_bill (bill_id),
  KEY idx_comm_method (community_id, pay_method, pay_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='缴费流水';

-- ---------------------------- 账户（余额 / 押金） ----------------------------
CREATE TABLE IF NOT EXISTS pm_account (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id BIGINT UNSIGNED NOT NULL,
  owner_type   TINYINT      NOT NULL DEFAULT 1 COMMENT '1住户 2单位',
  resident_id  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  merchant_id  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  owner_name   VARCHAR(100) NOT NULL DEFAULT '',
  balance      DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT '可用余额',
  deposit      DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT '押金余额',
  status       TINYINT      NOT NULL DEFAULT 1,
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_comm_owner (tenant_id, community_id, owner_type, resident_id, merchant_id, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='住户/单位账户';

-- ---------------------------- 账户流水 ----------------------------
CREATE TABLE IF NOT EXISTS pm_account_log (
  id            BIGINT UNSIGNED NOT NULL,
  tenant_id     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id  BIGINT UNSIGNED NOT NULL,
  account_id    BIGINT UNSIGNED NOT NULL,
  change_type   TINYINT      NOT NULL DEFAULT 1 COMMENT '1充值 2扣款 3退款 4押金抵扣 5调整',
  amount        DECIMAL(14,2) NOT NULL DEFAULT 0 COMMENT '正数增加,负数减少',
  balance_after DECIMAL(14,2) NOT NULL DEFAULT 0,
  target_type   TINYINT      NOT NULL DEFAULT 1 COMMENT '1余额 2押金',
  biz_type      VARCHAR(32)  NOT NULL DEFAULT '',
  biz_id        BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time   DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted       BIGINT       NOT NULL DEFAULT 0,
  remark        VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_account (account_id, create_time),
  KEY idx_comm_time (tenant_id, community_id, create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='账户流水';
