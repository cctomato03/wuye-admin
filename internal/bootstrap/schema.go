// Package bootstrap：启动期的幂等结构校验。
//
// 背景：MySQL 8 不支持 `ADD COLUMN IF NOT EXISTS`，而迁移执行器把脚本按分号拆成单条语句、
// 且每条都可能是池里的不同连接，无法安全使用会话变量/存储过程。因此这类"加列/加索引"的
// DDL 放到这里：先查 information_schema，再决定是否执行，天然幂等，可修复"迁移执行到一半失败"。
package bootstrap

import (
	"fmt"

	"gorm.io/gorm"
)

type columnSpec struct {
	Table  string
	Column string
	DDL    string
}

type indexSpec struct {
	Table string
	Index string
	DDL   string
}

// schemaGuards 需要保证存在的列与索引（幂等）。
var schemaGuards = struct {
	Columns []columnSpec
	Indexes []indexSpec
}{
	Columns: []columnSpec{
		{
			Table:  "pm_structure_node",
			Column: "full_address",
			DDL: "ALTER TABLE pm_structure_node " +
				"ADD COLUMN full_address VARCHAR(500) NOT NULL DEFAULT '' " +
				"COMMENT '自动生成的层级地址（上级地址+层级名称）' AFTER address",
		},
		{
			Table:  "pm_structure_node",
			Column: "category",
			DDL: "ALTER TABLE pm_structure_node " +
				"ADD COLUMN category TINYINT NOT NULL DEFAULT 9 " +
				"COMMENT '空间类别 1住宅楼 2别墅 3商业楼 4办公楼 5车库/车位 6配套设施 9其他' AFTER node_type",
		},
	},
	Indexes: []indexSpec{
		{
			Table: "pm_structure_node",
			Index: "idx_comm_parent_name",
			DDL:   "ALTER TABLE pm_structure_node ADD KEY idx_comm_parent_name (community_id, parent_id, name)",
		},
	},
}

// EnsureSchema 幂等补齐关键列/索引，避免迁移脚本因"列已存在"而整体失败。
// 必须在 migrate.Run 之后调用：表由迁移创建，表不存在时跳过（全新库）。
func EnsureSchema(db *gorm.DB) error {
	for _, c := range schemaGuards.Columns {
		exists, err := hasTable(db, c.Table)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		ok, err := hasColumn(db, c.Table, c.Column)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		if err := db.Exec(c.DDL).Error; err != nil {
			return fmt.Errorf("补齐字段 %s.%s 失败: %w", c.Table, c.Column, err)
		}
	}
	for _, ix := range schemaGuards.Indexes {
		exists, err := hasTable(db, ix.Table)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		ok, err := hasIndex(db, ix.Table, ix.Index)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		if err := db.Exec(ix.DDL).Error; err != nil {
			return fmt.Errorf("补齐索引 %s.%s 失败: %w", ix.Table, ix.Index, err)
		}
	}
	return nil
}

func hasTable(db *gorm.DB, table string) (bool, error) {
	var n int64
	err := db.Raw(`SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name = ?`, table).Scan(&n).Error
	return n > 0, err
}

func hasColumn(db *gorm.DB, table, column string) (bool, error) {
	var n int64
	err := db.Raw(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`, table, column).Scan(&n).Error
	return n > 0, err
}

func hasIndex(db *gorm.DB, table, index string) (bool, error) {
	var n int64
	err := db.Raw(`SELECT COUNT(*) FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`, table, index).Scan(&n).Error
	return n > 0, err
}

// nodeRow 回填地址用的最小字段。
type nodeRow struct {
	ID          uint64
	CommunityID uint64
	ParentID    uint64
	Name        string
	LevelNo     int
	FullAddress string
}

// EnsureNodeAddresses 幂等回填层级地址：full_address = 上级地址 + 名称，
// 顶级 = 小区地址 + 名称。迁移只做数据规范化，地址回填放在这里，
// 这样"迁移执行到一半失败"或历史数据缺失时都能自愈。
func EnsureNodeAddresses(db *gorm.DB) error {
	exists, err := hasTable(db, "pm_structure_node")
	if err != nil || !exists {
		return err
	}
	var pending int64
	if err := db.Raw("SELECT COUNT(*) FROM pm_structure_node WHERE deleted = 0 AND (full_address = '' OR full_address IS NULL)").Scan(&pending).Error; err != nil {
		return err
	}
	if pending == 0 {
		return nil
	}
	// 小区基础地址
	var communities []struct {
		ID                       uint64
		Address                  string
		Province, City, District string
	}
	if err := db.Raw("SELECT id, address, province, city, district FROM pm_community WHERE deleted = 0").Scan(&communities).Error; err != nil {
		return err
	}
	base := make(map[uint64]string, len(communities))
	for _, c := range communities {
		addr := c.Address
		if addr == "" {
			addr = c.Province + c.City + c.District
		}
		base[c.ID] = addr
	}
	var nodes []nodeRow
	if err := db.Raw("SELECT id, community_id, parent_id, name, level_no, full_address FROM pm_structure_node WHERE deleted = 0 ORDER BY level_no ASC, id ASC").Scan(&nodes).Error; err != nil {
		return err
	}
	addrByID := make(map[uint64]string, len(nodes))
	for _, n := range nodes {
		parentAddr := base[n.CommunityID]
		if n.ParentID != 0 {
			if v, ok := addrByID[n.ParentID]; ok {
				parentAddr = v
			}
		}
		addr := parentAddr + n.Name
		addrByID[n.ID] = addr
		if n.FullAddress == addr {
			continue
		}
		if err := db.Exec("UPDATE pm_structure_node SET full_address = ? WHERE id = ?", addr, n.ID).Error; err != nil {
			return err
		}
	}
	return nil
}
