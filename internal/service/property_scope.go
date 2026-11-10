// 物业管理通用能力：小区级数据隔离、房屋/楼栋作用域校验、通用小工具。
//
// 硬约定（防越权四件套）：
//  1. 列表查询：Model(...) + tenantIDOf + scope.Community(all, ids, "community_id")
//  2. 详情/改删：先用带作用域的条件查出来，查不到统一返回 ErrNotFound（不暴露存在性）
//  3. 写入：先 MustCommunity 校验目标小区在授权范围内
//  4. 跨表引用（房屋/楼栋/设备）：必须校验与被引用对象同小区，否则拒绝
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"web-basic/internal/model/entity"
	"web-basic/internal/model/scope"
	"web-basic/internal/principal"
	"web-basic/pkg/errs"
)

// communityScope 当前登录用户的小区可见范围。
// all=true 表示不限制（超管 / 物业公司管理员）；否则返回被授权小区 ID 列表。
func (s *Service) communityScope(p *principal.Info) (bool, []uint64) {
	if p == nil {
		return false, nil
	}
	if p.IsSuper || p.CommunityAll {
		return true, p.CommunityIDs
	}
	return false, p.CommunityIDs
}

// tenantIDOf 数据过滤用的租户：平台超管返回 0（不按租户过滤，可跨租户运维）。
func (s *Service) tenantIDOf(p *principal.Info) uint64 {
	if p == nil || p.IsSuper {
		return 0
	}
	return p.TenantID
}

// scoped 业务表通用作用域：租户 + 小区。
func (s *Service) scoped(ctx context.Context, m any, p *principal.Info, alias string) *gorm.DB {
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(m)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	col := "community_id"
	if alias != "" {
		col = alias + ".community_id"
	}
	return tx.Scopes(scope.Community(all, ids, col))
}

// loadUserCommunities 读取用户被授权的小区（并返回默认小区）。
func (s *Service) loadUserCommunities(ctx context.Context, tenantID, userID uint64) ([]uint64, uint64, error) {
	var rows []entity.UserCommunity
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.UserCommunity{}).Where("user_id = ?", userID)
	if tenantID != 0 {
		tx = tx.Where("tenant_id = ?", tenantID)
	}
	if err := tx.Order("is_default DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	ids := make([]uint64, 0, len(rows))
	var def uint64
	for _, r := range rows {
		ids = append(ids, r.CommunityID)
		if def == 0 || r.IsDefault {
			def = r.CommunityID
		}
	}
	return ids, def, nil
}

// communityTableScope 小区表自身的作用域（过滤列是 id 而不是 community_id）。
func (s *Service) communityTableScope(ctx context.Context, p *principal.Info) *gorm.DB {
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Community{})
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	all, ids := s.communityScope(p)
	if !all {
		if len(ids) == 0 {
			return tx.Where("1 = 0")
		}
		tx = tx.Where("id IN ?", ids)
	}
	return tx
}

// MustCommunity 校验目标小区在授权范围内；不允许时返回"没有数据权限"而不是"不存在"。
//
// 三层判定：
//  1. 平台超管：放行（跨租户运维）
//  2. 物业公司管理员（CommunityAll）：小区必须属于本租户，防止拿着别的公司小区 ID 越界
//  3. 普通物业员工：必须在 pm_user_community 授权列表内
func (s *Service) MustCommunity(ctx context.Context, p *principal.Info, communityID uint64) error {
	if communityID == 0 {
		return errs.Invalid("请选择小区")
	}
	all, ids := s.communityScope(p)
	if all {
		if p != nil && p.IsSuper {
			return nil
		}
		tenantID := uint64(0)
		if p != nil {
			tenantID = p.TenantID
		}
		var n int64
		if err := s.Repo.DB.WithContext(ctx).Model(&entity.Community{}).
			Where("id = ? AND tenant_id = ?", communityID, tenantID).Count(&n).Error; err != nil {
			return errs.ErrDB.WithErr(err)
		}
		if n == 0 {
			return errs.ErrDataForbidden
		}
		return nil
	}
	for _, id := range ids {
		if id == communityID {
			return nil
		}
	}
	return errs.ErrDataForbidden
}

// findCommunity 带作用域读取小区（同时校验租户与授权）。
func (s *Service) findCommunity(ctx context.Context, p *principal.Info, communityID uint64) (*entity.Community, error) {
	if err := s.MustCommunity(ctx, p, communityID); err != nil {
		return nil, err
	}
	var c entity.Community
	tx := s.Repo.DB.WithContext(ctx).Where("id = ?", communityID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if err := tx.First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &c, nil
}

// findHouse 带作用域读取房屋（小区授权 + 租户）。
func (s *Service) findHouse(ctx context.Context, p *principal.Info, houseID uint64) (*entity.House, error) {
	var h entity.House
	all, ids := s.communityScope(p)
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.House{}).Where("id = ?", houseID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	tx = tx.Scopes(scope.Community(all, ids, "community_id"))
	if err := tx.First(&h).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &h, nil
}

// houseBelongsTo 校验房屋属于指定小区（跨表引用时用，避免把 A 小区数据挂到 B 小区）。
func (s *Service) houseBelongsTo(ctx context.Context, communityID, houseID uint64) (*entity.House, error) {
	if houseID == 0 {
		return nil, nil
	}
	var h entity.House
	if err := s.Repo.DB.WithContext(ctx).Where("id = ? AND community_id = ?", houseID, communityID).First(&h).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.Invalid("房屋不存在或不属于当前小区")
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &h, nil
}

// buildingBelongsTo 校验楼栋/空间节点属于指定小区。
func (s *Service) buildingBelongsTo(ctx context.Context, communityID, buildingID uint64) (*entity.StructureNode, error) {
	if buildingID == 0 {
		return nil, nil
	}
	var b entity.StructureNode
	if err := s.Repo.DB.WithContext(ctx).Where("id = ? AND community_id = ?", buildingID, communityID).First(&b).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.Invalid("楼栋不存在或不属于当前小区")
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &b, nil
}

// deviceBelongsTo 校验设备属于指定小区。
func (s *Service) deviceBelongsTo(ctx context.Context, communityID, deviceID uint64) (*entity.Device, error) {
	var d entity.Device
	if err := s.Repo.DB.WithContext(ctx).Where("id = ? AND community_id = ?", deviceID, communityID).First(&d).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.Invalid("设备不存在或不属于当前小区")
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &d, nil
}

// ---------- 通用小工具 ----------

// parseDate 解析 YYYY-MM-DD（空串返回 nil）。
func parseDate(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	if d, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
		return &d, nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", v, time.Local); err == nil {
		return &t, nil
	}
	return nil, errs.Invalid("日期格式应为 YYYY-MM-DD")
}

// parseDateTime 解析 YYYY-MM-DD HH:mm:ss（空串返回 nil）。
func parseDateTime(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return &t, nil
		}
	}
	return nil, errs.Invalid("时间格式应为 YYYY-MM-DD HH:mm:ss")
}

// today 当天 0 点。
func today() time.Time {
	n := time.Now()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.Local)
}

// endOfDay 指定日期 23:59:59。
func endOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.Local)
}

// monthRange 账期 YYYYMM → 当月首日/末日。
func monthRange(period string) (time.Time, time.Time, error) {
	t, err := time.ParseInLocation("200601", period, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, errs.Invalid("账期格式应为 YYYYMM")
	}
	start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
	return start, start.AddDate(0, 1, -1), nil
}

// likePrefix 统一前缀模糊查询（走索引，避免 %x% 全表扫描）。
func likePrefix(v string) string { return strings.TrimSpace(v) + "%" }

// chunk 分批回调：把大集合切成每批 size 条处理（账单生成/授权展开/导入用）。
func chunk[T any](items []T, size int, fn func(batch []T) error) error {
	if size <= 0 {
		size = 500
	}
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		if err := fn(items[start:end]); err != nil {
			return err
		}
	}
	return nil
}

// joinHouseName 拼接"楼栋单元房号"形式的房屋全称（导入/批量生成时用）。
func joinHouseName(building, unit, houseNo string) string {
	parts := make([]string, 0, 3)
	if building != "" {
		parts = append(parts, building)
	}
	if unit != "" {
		parts = append(parts, unit)
	}
	if houseNo != "" {
		parts = append(parts, houseNo)
	}
	return strings.Join(parts, "")
}

// idsToSet 切片转集合（批量校验授权用）。
func idsToSet(ids []uint64) map[uint64]struct{} {
	m := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}

// wrapTxErr 事务/写库错误归一化：
// 已经是业务错误（AppError，如"余额不足""编码重复"）时原样返回，
// 只有真正的数据库错误才包装成 50001，避免把"规则不满足"报成"数据库异常"。
func wrapTxErr(err error) error {
	if err == nil {
		return nil
	}
	var ae *errs.AppError
	if errors.As(err, &ae) {
		return err
	}
	return errs.Normalize(errs.ErrDB.WithErr(err))
}

// fmtErr 把底层错误转成业务可读错误（导入明细等场景）。
func fmtErr(err error) string {
	var ae *errs.AppError
	if errors.As(err, &ae) {
		if ae.Err != nil {
			return ae.Err.Error()
		}
		return ae.Msg
	}
	return err.Error()
}

// newBatchNo 生成批次号：前缀 + 时间 + 6 位随机。
func newBatchNo(prefix string) string {
	return fmt.Sprintf("%s%s%06d", prefix, time.Now().Format("20060102150405"), time.Now().Nanosecond()%1000000)
}
