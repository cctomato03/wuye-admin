-- =============================================================================
-- 000021_property_people : 人员域（住户 / 住户房屋关系与历史 / 人脸 / 访客 / 其他人员 / 单位商铺）
-- 人员数据隔离原则：
--   pm_resident 是"租户级人员档案"（一个证件一条），跨小区唯一；
--   pm_resident_house 才是"小区级关系"，物业人员只能看到被授权小区内的关系与房屋明细。
-- =============================================================================

SET NAMES utf8mb4;

-- ---------------------------- 住户档案 ----------------------------
CREATE TABLE IF NOT EXISTS pm_resident (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所属物业公司',
  name         VARCHAR(64)  NOT NULL COMMENT '姓名',
  id_type      TINYINT      NOT NULL DEFAULT 1 COMMENT '证件类型 1身份证 2护照 3港澳通行证 4台胞证 5军官证 9其他',
  id_no        VARCHAR(64)  NOT NULL COMMENT '证件号码(租户内唯一标识)',
  gender       TINYINT      NOT NULL DEFAULT 0 COMMENT '0未知 1男 2女',
  birthday     DATE         NULL COMMENT '出生日期',
  phone        VARCHAR(20)  NOT NULL DEFAULT '' COMMENT '手机号',
  mobile2      VARCHAR(20)  NOT NULL DEFAULT '' COMMENT '备用电话',
  nation       VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '民族',
  native_place VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '籍贯',
  household_addr VARCHAR(255) NOT NULL DEFAULT '' COMMENT '户籍地址',
  live_addr    VARCHAR(255) NOT NULL DEFAULT '' COMMENT '居住地址',
  photo_url    VARCHAR(500) NOT NULL DEFAULT '' COMMENT '主照片(人脸)',
  emergency_contact VARCHAR(64) NOT NULL DEFAULT '' COMMENT '紧急联系人',
  emergency_phone   VARCHAR(20) NOT NULL DEFAULT '' COMMENT '紧急联系电话',
  first_community_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '首次登记小区(统计用)',
  status       TINYINT      NOT NULL DEFAULT 1 COMMENT '1正常 2迁出 0停用',
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_tenant_idno (tenant_id, id_type, id_no, deleted),
  KEY idx_tenant_phone (tenant_id, phone),
  KEY idx_tenant_name (tenant_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='住户档案';

-- ---------------------------- 住户-房屋关系 ----------------------------
CREATE TABLE IF NOT EXISTS pm_resident_house (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id BIGINT UNSIGNED NOT NULL,
  resident_id  BIGINT UNSIGNED NOT NULL,
  house_id     BIGINT UNSIGNED NOT NULL,
  rel_role     TINYINT      NOT NULL DEFAULT 1 COMMENT '1业主 2家人 3租户',
  relation     VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '与业主关系:本人/配偶/子女/父母/其他',
  is_main      TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '是否主住户',
  start_date   DATE         NULL COMMENT '生效日期',
  end_date     DATE         NULL COMMENT '到期日期(租户必填,业主可空=长期)',
  status       TINYINT      NOT NULL DEFAULT 1 COMMENT '1有效 2已到期 3已解除',
  bind_time    DATETIME     NULL COMMENT '登记时间',
  unbind_time  DATETIME     NULL COMMENT '解除/到期时间',
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_comm_house_resident (community_id, house_id, resident_id, deleted),
  KEY idx_comm_resident (community_id, resident_id, deleted),
  KEY idx_comm_house (community_id, house_id, status, deleted),
  KEY idx_comm_status_expire (tenant_id, community_id, status, end_date),
  KEY idx_resident (resident_id, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='住户-房屋关系';

-- ---------------------------- 关系变更历史 ----------------------------
CREATE TABLE IF NOT EXISTS pm_resident_house_log (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id BIGINT UNSIGNED NOT NULL,
  resident_id  BIGINT UNSIGNED NOT NULL,
  house_id     BIGINT UNSIGNED NOT NULL,
  rel_id       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '关系记录ID',
  action       TINYINT      NOT NULL DEFAULT 1 COMMENT '1登记 2续期 3到期 4解除 5删除 6还原',
  before_end   DATE         NULL COMMENT '变更前到期日',
  after_end    DATE         NULL COMMENT '变更后到期日',
  before_role  TINYINT      NOT NULL DEFAULT 0,
  after_role   TINYINT      NOT NULL DEFAULT 0,
  operator_id  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  operator_name VARCHAR(64) NOT NULL DEFAULT '',
  operate_time DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_rel (rel_id),
  KEY idx_comm_time (community_id, operate_time),
  KEY idx_resident_time (resident_id, operate_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='住户房屋关系变更历史';

-- ---------------------------- 住户人脸 ----------------------------
CREATE TABLE IF NOT EXISTS pm_resident_face (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id BIGINT UNSIGNED NOT NULL,
  resident_id  BIGINT UNSIGNED NOT NULL,
  file_id      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  file_url     VARCHAR(500) NOT NULL DEFAULT '',
  face_quality INT          NOT NULL DEFAULT 0 COMMENT '人脸质量分(第三方返回)',
  status       TINYINT      NOT NULL DEFAULT 1 COMMENT '1待下发 2已下发 3下发失败 4已解除',
  is_primary   TINYINT(1)   NOT NULL DEFAULT 0,
  issue_msg    VARCHAR(500) NOT NULL DEFAULT '',
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT       NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_comm_resident (tenant_id, community_id, resident_id, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='住户人脸照片';

-- ---------------------------- 访客 ----------------------------
CREATE TABLE IF NOT EXISTS pm_visitor (
  id             BIGINT UNSIGNED NOT NULL,
  tenant_id      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id   BIGINT UNSIGNED NOT NULL,
  visit_no       VARCHAR(32)  NOT NULL COMMENT '访客码/登记单号',
  name           VARCHAR(64)  NOT NULL COMMENT '访客姓名',
  phone          VARCHAR(20)  NOT NULL DEFAULT '',
  id_no          VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '证件号(实名登记)',
  company        VARCHAR(100) NOT NULL DEFAULT '' COMMENT '来访单位',
  visit_type     TINYINT      NOT NULL DEFAULT 1 COMMENT '1访客 2快递 3家政 4维修 5其他',
  visit_reason   VARCHAR(200) NOT NULL DEFAULT '' COMMENT '来访事由',
  visit_house_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '受访房屋',
  visit_house_name VARCHAR(200) NOT NULL DEFAULT '',
  resident_id    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '受访住户',
  resident_name  VARCHAR(64)  NOT NULL DEFAULT '',
  vehicle_no     VARCHAR(20)  NOT NULL DEFAULT '' COMMENT '随行车牌',
  people_count   INT          NOT NULL DEFAULT 1 COMMENT '来访人数(含随行)',
  start_time     DATETIME     NULL COMMENT '预约/到访开始',
  end_time       DATETIME     NULL COMMENT '预约结束(超时自动失效)',
  enter_time     DATETIME     NULL COMMENT '实际进入时间',
  leave_time     DATETIME     NULL COMMENT '实际离开时间',
  status         TINYINT      NOT NULL DEFAULT 1 COMMENT '1待到访 2在访 3已离场 4已过期 5已取消',
  operator_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  operator_name  VARCHAR(64)  NOT NULL DEFAULT '',
  face_url       VARCHAR(500) NOT NULL DEFAULT '',
  create_by      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time    DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted        BIGINT       NOT NULL DEFAULT 0,
  remark         VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_visit_no (visit_no, deleted),
  KEY idx_comm_status (tenant_id, community_id, status, create_time),
  KEY idx_comm_house (community_id, visit_house_id, create_time),
  KEY idx_comm_phone (community_id, phone),
  KEY idx_expire (status, end_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='访客登记';

CREATE TABLE IF NOT EXISTS pm_visitor_accompany (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id BIGINT UNSIGNED NOT NULL,
  visitor_id   BIGINT UNSIGNED NOT NULL,
  name         VARCHAR(64) NOT NULL DEFAULT '',
  id_no        VARCHAR(64) NOT NULL DEFAULT '',
  phone        VARCHAR(20) NOT NULL DEFAULT '',
  relation     VARCHAR(32) NOT NULL DEFAULT '',
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME    NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT      NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_visitor (visitor_id, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='访客随行人员';

-- ---------------------------- 其他人员类别 ----------------------------
CREATE TABLE IF NOT EXISTS pm_person_category (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0表示公司级共享',
  name         VARCHAR(64)  NOT NULL COMMENT '类别名称,如 保洁/维修',
  code         VARCHAR(64)  NOT NULL DEFAULT '',
  need_face    TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '默认需要人脸',
  need_card    TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '默认需要门禁卡',
  need_phone   TINYINT(1)   NOT NULL DEFAULT 0 COMMENT '默认需要手机拨号',
  default_days INT          NOT NULL DEFAULT 30 COMMENT '默认有效天数',
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
  UNIQUE KEY uk_tenant_name (tenant_id, community_id, name, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='其他人员类别';

-- ---------------------------- 其他人员 ----------------------------
CREATE TABLE IF NOT EXISTS pm_other_person (
  id            BIGINT UNSIGNED NOT NULL,
  tenant_id     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id  BIGINT UNSIGNED NOT NULL,
  category_id   BIGINT UNSIGNED NOT NULL DEFAULT 0,
  name          VARCHAR(64)  NOT NULL,
  phone         VARCHAR(20)  NOT NULL DEFAULT '',
  id_no         VARCHAR(64)  NOT NULL DEFAULT '',
  company_name  VARCHAR(100) NOT NULL DEFAULT '' COMMENT '所属单位/施工方',
  work_content  VARCHAR(200) NOT NULL DEFAULT '' COMMENT '作业内容',
  valid_from    DATE         NULL COMMENT '有效期开始',
  valid_to      DATE         NULL COMMENT '有效期结束(到期自动解除门禁)',
  face_url      VARCHAR(500) NOT NULL DEFAULT '',
  card_no       VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '门禁卡号',
  status        TINYINT      NOT NULL DEFAULT 1 COMMENT '1有效 2停用 3已过期',
  create_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by     BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time   DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted       BIGINT       NOT NULL DEFAULT 0,
  remark        VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_comm_cat (tenant_id, community_id, category_id, deleted),
  KEY idx_comm_valid (community_id, status, valid_to),
  KEY idx_comm_phone (community_id, phone)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='其他人员(保洁/维修/施工等)';

CREATE TABLE IF NOT EXISTS pm_other_person_device (
  id           BIGINT UNSIGNED NOT NULL,
  tenant_id    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id BIGINT UNSIGNED NOT NULL,
  person_id    BIGINT UNSIGNED NOT NULL,
  device_id    BIGINT UNSIGNED NOT NULL,
  create_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time  DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by    BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time  DATETIME    NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted      BIGINT      NOT NULL DEFAULT 0,
  remark       VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  UNIQUE KEY uk_person_device (person_id, device_id, deleted),
  KEY idx_device (device_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='其他人员-门禁设备';

-- ---------------------------- 单位/商铺 ----------------------------
CREATE TABLE IF NOT EXISTS pm_merchant (
  id             BIGINT UNSIGNED NOT NULL,
  tenant_id      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  community_id   BIGINT UNSIGNED NOT NULL,
  name           VARCHAR(100) NOT NULL COMMENT '单位名称',
  merchant_type  TINYINT      NOT NULL DEFAULT 1 COMMENT '1办公 2商铺 3学校 4医疗 5其他',
  building_id    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所在商业楼栋',
  house_id       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '所在房屋/铺位',
  house_name     VARCHAR(200) NOT NULL DEFAULT '',
  contact_name   VARCHAR(64)  NOT NULL DEFAULT '',
  contact_phone  VARCHAR(20)  NOT NULL DEFAULT '',
  id_no          VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '证件号(负责人)',
  industry       VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '所属行业',
  business_license VARCHAR(64) NOT NULL DEFAULT '' COMMENT '营业执照号',
  area           DECIMAL(10,2) NOT NULL DEFAULT 0 COMMENT '经营面积(㎡)',
  employee_count INT          NOT NULL DEFAULT 0,
  start_date     DATE         NULL COMMENT '入驻日期',
  end_date       DATE         NULL COMMENT '到期日期',
  status         TINYINT      NOT NULL DEFAULT 1 COMMENT '1在营 2停业 3已退场',
  create_by      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  create_time    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_by      BIGINT UNSIGNED NOT NULL DEFAULT 0,
  update_time    DATETIME     NULL ON UPDATE CURRENT_TIMESTAMP,
  deleted        BIGINT       NOT NULL DEFAULT 0,
  remark         VARCHAR(500) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_comm_type (tenant_id, community_id, merchant_type, deleted),
  KEY idx_comm_house (community_id, house_id),
  KEY idx_comm_name (community_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='单位/商铺';
