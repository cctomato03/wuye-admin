// Package scope GORM 查询作用域：租户隔离、数据权限、软删除过滤。
package scope

import (
	"gorm.io/gorm"

	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
)

// Tenant 租户字段隔离；tenantID=0 表示平台级数据（超管跨租户）。
func Tenant(tenantID uint64) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if tenantID == 0 {
			return db
		}
		return db.Where("tenant_id = ?", tenantID)
	}
}

// Community 小区级数据隔离（物业业务表的强制作用域）。
//
//	all=true  → 不追加条件（超管 / 物业公司管理员：本租户全部小区）
//	all=false → community_id IN (ids)；ids 为空时退化为 1=0（拒绝，绝不放开全表）
//
// column 为业务表的小区列名，默认 community_id。
func Community(all bool, ids []uint64, column string) func(*gorm.DB) *gorm.DB {
	if column == "" {
		column = "community_id"
	}
	return func(db *gorm.DB) *gorm.DB {
		if all {
			return db
		}
		if len(ids) == 0 {
			return db.Where("1 = 0")
		}
		return db.Where(column+" IN ?", ids)
	}
}

// DataScope 行级数据权限，取多角色最宽范围。
// deptColumn / userColumn 为目标表的部门列与创建人列名。
func DataScope(p *principal.Info, deptColumn, userColumn string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if p == nil || p.IsSuper {
			return db
		}
		switch p.DataScope {
		case entity.DataScopeAll:
			return db
		case entity.DataScopeDept:
			if deptColumn == "" {
				return db
			}
			return db.Where(deptColumn+" = ?", p.DeptID)
		case entity.DataScopeDeptAndSub:
			if deptColumn == "" || len(p.DeptIDs) == 0 {
				return db
			}
			return db.Where(deptColumn+" IN ?", p.DeptIDs)
		case entity.DataScopeCustom:
			if deptColumn == "" || len(p.CustomDeptIDs) == 0 {
				return db.Where("1 = 0")
			}
			return db.Where(deptColumn+" IN ?", p.CustomDeptIDs)
		case entity.DataScopeSelf:
			if userColumn == "" {
				return db
			}
			return db.Where(userColumn+" = ?", p.UserID)
		default:
			return db.Where("1 = 0") // 兜底拒绝，避免越权
		}
	}
}
