// 物业公司组织与员工：组织架构（复用 sys_dept）+ 员工档案（pm_employee）+ 小区授权（pm_user_community）。
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"web-basic/internal/model/entity"
	"web-basic/internal/principal"
	"web-basic/pkg/crypto"
	"web-basic/pkg/errs"
	"web-basic/pkg/id"
	"web-basic/pkg/jsonx"
)

// EmployeeQuery 员工查询。
type EmployeeQuery struct {
	entity.PageQuery
	Keyword     string       `form:"keyword"`
	DeptID      jsonx.Uint64 `form:"deptId"`
	CommunityID jsonx.Uint64 `form:"communityId"`
	TenantID    jsonx.Uint64 `form:"tenantId"`
	Status      *int         `form:"status"`
	EmpType     *int         `form:"empType"`
}

// EmployeeVO 员工出参。
type EmployeeVO struct {
	entity.Employee
	DeptName       string            `json:"deptName"`
	CommunityName  string            `json:"communityName"`
	Username       string            `json:"username"`
	CommunityIDs   jsonx.Uint64Slice `json:"communityIds"`
	CommunityNames []string          `json:"communityNames"`
	RoleIDs        jsonx.Uint64Slice `json:"roleIds"`
}

// ListEmployees 员工分页列表（本租户内）。
func (s *Service) ListEmployees(ctx context.Context, q *EmployeeQuery, p *principal.Info) ([]EmployeeVO, int64, error) {
	q.Normalize()
	tid, err := s.OrgTenant(ctx, p, q.TenantID.Uint64())
	if err != nil {
		return nil, 0, err
	}
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Employee{}).Where("tenant_id = ?", tid)
	if q.Keyword != "" {
		kw := strings.TrimSpace(q.Keyword)
		tx = tx.Where("(name LIKE ? OR phone LIKE ? OR employee_no LIKE ?)", kw+"%", kw+"%", kw+"%")
	}
	if q.DeptID.Uint64() != 0 {
		tx = tx.Where("dept_id = ?", q.DeptID.Uint64())
	}
	if q.CommunityID.Uint64() != 0 {
		tx = tx.Where("community_id = ?", q.CommunityID.Uint64())
	}
	if q.Status != nil {
		tx = tx.Where("status = ?", *q.Status)
	}
	if q.EmpType != nil {
		tx = tx.Where("emp_type = ?", *q.EmpType)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	list := make([]entity.Employee, 0, q.PageSize)
	if err := tx.Order("id DESC").Offset(q.Offset()).Limit(q.PageSize).Find(&list).Error; err != nil {
		return nil, 0, errs.ErrDB.WithErr(err)
	}
	out := make([]EmployeeVO, 0, len(list))
	if len(list) == 0 {
		return out, total, nil
	}
	deptIDs := make([]uint64, 0, len(list))
	commIDs := make([]uint64, 0, len(list))
	userIDs := make([]uint64, 0, len(list))
	for _, e := range list {
		deptIDs = append(deptIDs, e.DeptID)
		commIDs = append(commIDs, e.CommunityID)
		userIDs = append(userIDs, e.UserID)
	}
	deptNames := s.nameMap(ctx, &entity.Dept{}, deptIDs)
	commNames := s.nameMap(ctx, &entity.Community{}, commIDs)
	var users []entity.User
	if err := s.Repo.DB.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error; err == nil {
		userMap := make(map[uint64]entity.User, len(users))
		for _, u := range users {
			userMap[u.ID] = u
		}
		for _, e := range list {
			vo := EmployeeVO{Employee: e, DeptName: deptNames[e.DeptID], CommunityName: commNames[e.CommunityID]}
			if u, ok := userMap[e.UserID]; ok {
				vo.Username = u.Username
			}
			ids, _, _ := s.loadUserCommunities(ctx, e.TenantID, e.UserID)
			vo.CommunityIDs = ids
			names := make([]string, 0, len(ids))
			nameMap := s.nameMap(ctx, &entity.Community{}, ids)
			for _, cid := range ids {
				names = append(names, nameMap[cid])
			}
			vo.CommunityNames = names
			out = append(out, vo)
		}
		return out, total, nil
	}
	for _, e := range list {
		out = append(out, EmployeeVO{Employee: e, DeptName: deptNames[e.DeptID], CommunityName: commNames[e.CommunityID]})
	}
	return out, total, nil
}

// EmployeeReq 员工入参。
type EmployeeReq struct {
	TenantID    jsonx.Uint64 `json:"tenantId"`
	EmployeeNo  string       `json:"employeeNo" binding:"max=64"`
	Name        string       `json:"name" binding:"required,max=64"`
	DeptID      jsonx.Uint64 `json:"deptId"`
	CommunityID jsonx.Uint64 `json:"communityId"` // 主责小区
	Position    string       `json:"position" binding:"max=64"`
	Phone       string       `json:"phone" binding:"max=20"`
	IDNo        string       `json:"idNo" binding:"max=64"`
	Gender      int          `json:"gender"`
	EmpType     int          `json:"empType"`
	EntryDate   string       `json:"entryDate"`
	LeaveDate   string       `json:"leaveDate"`
	FaceURL     string       `json:"faceUrl" binding:"max=500"`
	Status      int          `json:"status"`
	Remark      string       `json:"remark" binding:"max=500"`
	// 登录账号（不填则不创建账号，仅建档）
	Username string            `json:"username" binding:"omitempty,min=3,max=32"`
	Password string            `json:"password" binding:"omitempty,min=8,max=32"`
	RoleIDs  jsonx.Uint64Slice `json:"roleIds"`
	// 授权小区（不填则默认主责小区）
	CommunityIDs jsonx.Uint64Slice `json:"communityIds"`
}

// CreateEmployee 新增员工（可选同时创建登录账号与小区授权）。
func (s *Service) CreateEmployee(ctx context.Context, req *EmployeeReq, p *principal.Info) (uint64, error) {
	tid, err := s.OrgTenant(ctx, p, req.TenantID.Uint64())
	if err != nil {
		return 0, err
	}
	tenantID := tid
	if req.CommunityID.Uint64() != 0 {
		if err := s.MustCommunity(ctx, p, req.CommunityID.Uint64()); err != nil {
			return 0, err
		}
		com, err := s.findCommunity(ctx, p, req.CommunityID.Uint64())
		if err != nil {
			return 0, err
		}
		if com.TenantID != tenantID {
			if req.TenantID.Uint64() != 0 {
				return 0, errs.Invalid("所选小区不属于目标物业公司")
			}
			// 平台管理角色未显式指定公司时，按小区所属公司落库
			if err := s.AssertTenantAllowed(ctx, p, com.TenantID); err != nil {
				return 0, err
			}
			tenantID = com.TenantID
		}
	}
	entry, err := parseDate(req.EntryDate)
	if err != nil {
		return 0, err
	}
	leave, err := parseDate(req.LeaveDate)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	empID := id.Next()
	var userID uint64

	err = s.Repo.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 可选创建登录账号
		if strings.TrimSpace(req.Username) != "" {
			var dup int64
			if err := tx.Model(&entity.User{}).Where("username = ?", req.Username).Count(&dup).Error; err != nil {
				return err
			}
			if dup > 0 {
				return errs.Invalid("登录账号已存在")
			}
			pwd := req.Password
			if pwd == "" {
				pwd = crypto.RandomPassword(12)
			}
			if err := crypto.CheckStrength(pwd, s.Cfg.Auth.PwdMinLen); err != nil {
				return errs.InvalidErr(err)
			}
			hash, err := crypto.HashPassword(pwd)
			if err != nil {
				return err
			}
			userID = id.Next()
			u := &entity.User{
				Base:          entity.Base{ID: userID, CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now},
				Username:      strings.TrimSpace(req.Username),
				Nickname:      req.Name,
				Password:      hash,
				DeptID:        req.DeptID.Uint64(),
				Mobile:        req.Phone,
				Status:        1,
				PwdUpdateTime: &now,
				TenantID:      tenantID,
			}
			if err := tx.Create(u).Error; err != nil {
				return err
			}
			roleIDs := req.RoleIDs.ToUint64()
			if len(roleIDs) == 0 {
				// 新公司的内置角色只有「物业公司管理员」，不能默认把管理员角色发给普通员工；
				// 优先用公司里已有的普通角色（非管理员），没有就要求显式选择。
				var def entity.Role
				err := tx.Where("tenant_id = ? AND code <> ? AND status = 1", tenantID, "pm_company_admin").
					Order("sort ASC, id ASC").First(&def).Error
				if err != nil {
					return errs.Invalid("请为新员工选择角色（可在「组织与权限 → 角色管理」先创建角色）")
				}
				roleIDs = []uint64{def.ID}
			}
			if err := s.assertRolesOfTenant(ctx, roleIDs, tenantID); err != nil {
				return err
			}
			for _, rid := range roleIDs {
				if err := tx.Create(&entity.UserRole{UserID: userID, RoleID: rid}).Error; err != nil {
					return err
				}
			}
		}
		emp := &entity.Employee{
			Base:        entity.Base{ID: empID, CreateBy: p.UserID, CreateTime: now, UpdateBy: p.UserID, UpdateTime: &now, Remark: req.Remark},
			TenantID:    tenantID,
			UserID:      userID,
			EmployeeNo:  strings.TrimSpace(req.EmployeeNo),
			Name:        strings.TrimSpace(req.Name),
			DeptID:      req.DeptID.Uint64(),
			CommunityID: req.CommunityID.Uint64(),
			Position:    req.Position,
			Phone:       req.Phone,
			IDNo:        req.IDNo,
			Gender:      req.Gender,
			EmpType:     pmDefault(req.EmpType, 1),
			EntryDate:   entry,
			LeaveDate:   leave,
			FaceURL:     req.FaceURL,
			Status:      pmDefault(req.Status, 1),
		}
		if emp.EmployeeNo == "" {
			emp.EmployeeNo = "E" + now.Format("20060102") + id.NewString()[15:]
		}
		if err := tx.Create(emp).Error; err != nil {
			return err
		}
		// 小区授权
		communities := req.CommunityIDs.ToUint64()
		if len(communities) == 0 && req.CommunityID.Uint64() != 0 {
			communities = []uint64{req.CommunityID.Uint64()}
		}
		if userID != 0 && len(communities) > 0 {
			for _, cid := range communities {
				uc := &entity.UserCommunity{
					Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now},
					TenantID:    tenantID,
					UserID:      userID,
					CommunityID: cid,
					IsDefault:   cid == req.CommunityID.Uint64(),
				}
				if err := tx.Create(uc).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, wrapTxErr(err)
	}
	if userID != 0 {
		s.invalidatePrincipal(ctx, userID)
	}
	// 下发物业人员门禁（小区全部设备）
	if req.CommunityID.Uint64() != 0 {
		var emp entity.Employee
		if err := s.Repo.DB.WithContext(ctx).Where("id = ?", empID).First(&emp).Error; err == nil {
			s.syncEmployeeAccess(ctx, req.CommunityID.Uint64(), &emp, p)
		}
	}
	return empID, nil
}

// UpdateEmployee 修改员工。
func (s *Service) UpdateEmployee(ctx context.Context, empID uint64, req *EmployeeReq, p *principal.Info) error {
	emp, err := s.employeeScoped(ctx, p, empID)
	if err != nil {
		return err
	}
	entry, err := parseDate(req.EntryDate)
	if err != nil {
		return err
	}
	leave, err := parseDate(req.LeaveDate)
	if err != nil {
		return err
	}
	if err := s.Repo.DB.WithContext(ctx).Model(&entity.Employee{}).Where("id = ?", emp.ID).Updates(map[string]any{
		"employee_no":  strings.TrimSpace(req.EmployeeNo),
		"name":         strings.TrimSpace(req.Name),
		"dept_id":      req.DeptID.Uint64(),
		"community_id": req.CommunityID.Uint64(),
		"position":     req.Position,
		"phone":        req.Phone,
		"id_no":        req.IDNo,
		"gender":       req.Gender,
		"emp_type":     pmDefault(req.EmpType, 1),
		"entry_date":   entry,
		"leave_date":   leave,
		"face_url":     req.FaceURL,
		"status":       pmDefault(req.Status, 1),
		"remark":       req.Remark,
		"update_by":    p.UserID,
		"update_time":  time.Now(),
	}).Error; err != nil {
		return wrapTxErr(err)
	}
	if req.Status != 1 {
		// 离职/停用 → 撤销门禁（0 或 2 均视为不可用）
		s.revokePersonAccessByType(ctx, emp.TenantID, PersonTypeEmployee, emp.ID, p)
	}
	return nil
}

// DeleteEmployee 删除员工（同时软删除登录账号并撤销门禁）。
func (s *Service) DeleteEmployee(ctx context.Context, empID uint64, p *principal.Info) error {
	emp, err := s.employeeScoped(ctx, p, empID)
	if err != nil {
		return err
	}
	s.revokePersonAccessByType(ctx, emp.TenantID, PersonTypeEmployee, emp.ID, p)
	if err := s.Repo.DB.WithContext(ctx).Delete(&entity.Employee{}, emp.ID).Error; err != nil {
		return errs.ErrDB.WithErr(err)
	}
	if emp.UserID != 0 {
		if err := s.Repo.DB.WithContext(ctx).Delete(&entity.User{}, emp.UserID).Error; err != nil {
			return errs.ErrDB.WithErr(err)
		}
		_ = s.Repo.DB.WithContext(ctx).Where("user_id = ?", emp.UserID).Delete(&entity.UserCommunity{}).Error
		s.invalidatePrincipal(ctx, emp.UserID)
	}
	return nil
}

// employeeScoped 带作用域读取员工。
func (s *Service) employeeScoped(ctx context.Context, p *principal.Info, empID uint64) (*entity.Employee, error) {
	var e entity.Employee
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Employee{}).Where("id = ?", empID)
	if t := s.tenantIDOf(p); t != 0 {
		tx = tx.Where("tenant_id = ?", t)
	}
	if err := tx.First(&e).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, errs.ErrDB.WithErr(err)
	}
	return &e, nil
}

// revokePersonAccessByType 按人员类型撤销全部授权（离职/删除/停用）。
func (s *Service) revokePersonAccessByType(ctx context.Context, tenantID uint64, personType int, personID uint64, p *principal.Info) {
	var grants []entity.AccessGrant
	if err := s.Repo.DB.WithContext(ctx).
		Where("tenant_id = ? AND person_type = ? AND person_id = ? AND status IN ?",
			tenantID, personType, personID,
			[]int{GrantStatusPending, GrantStatusIssuing, GrantStatusSuccess, GrantStatusFailed}).
		Find(&grants).Error; err != nil || len(grants) == 0 {
		return
	}
	_ = s.revokeGrantsInternal(ctx, &grants, GrantStatusRevoked, p)
}

// GrantEmployeeCommunities 调整员工的小区授权。
func (s *Service) GrantEmployeeCommunities(ctx context.Context, empID uint64, communityIDs []uint64, defaultID uint64, p *principal.Info) error {
	emp, err := s.employeeScoped(ctx, p, empID)
	if err != nil {
		return err
	}
	if emp.UserID == 0 {
		return errs.Invalid("该员工没有登录账号，无法设置小区授权（请先为其创建账号）")
	}
	for _, cid := range communityIDs {
		com, err := s.findCommunity(ctx, p, cid)
		if err != nil {
			return err
		}
		if com.TenantID != emp.TenantID {
			return errs.Invalid("只能把本公司的小区授权给本公司员工")
		}
	}
	now := time.Now()
	err = s.Repo.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", emp.UserID).Delete(&entity.UserCommunity{}).Error; err != nil {
			return err
		}
		for _, cid := range communityIDs {
			uc := &entity.UserCommunity{
				Base:        entity.Base{ID: id.Next(), CreateBy: p.UserID, CreateTime: now},
				TenantID:    emp.TenantID,
				UserID:      emp.UserID,
				CommunityID: cid,
				IsDefault:   cid == defaultID,
			}
			if err := tx.Create(uc).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return wrapTxErr(err)
	}
	s.invalidatePrincipal(ctx, emp.UserID)
	// 授权后按新小区下发物业人员门禁
	for _, cid := range communityIDs {
		s.syncEmployeeAccess(ctx, cid, emp, p)
	}
	return nil
}

// EmployeeDeptNode 组织架构节点。
type EmployeeDeptNode struct {
	ID       uint64              `json:"id,string"`
	ParentID uint64              `json:"parentId,string"`
	Name     string              `json:"name"`
	Leader   string              `json:"leader"`
	Phone    string              `json:"phone"`
	Staffing int64               `json:"staffing"`
	Children []*EmployeeDeptNode `json:"children,omitempty"`
}

// EmployeeDeptTree 物业公司组织架构树（含各部门人数）。
func (s *Service) EmployeeDeptTree(ctx context.Context, p *principal.Info, tenantIDArg uint64) ([]*EmployeeDeptNode, error) {
	tenantID, err := s.OrgTenant(ctx, p, tenantIDArg)
	if err != nil {
		return nil, err
	}
	depts, err := s.Repo.ListDepts(ctx, tenantID)
	if err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	var counts []struct {
		DeptID uint64
		Count  int64
	}
	_ = s.Repo.DB.WithContext(ctx).Model(&entity.Employee{}).
		Select("dept_id, COUNT(*) AS count").Where("tenant_id = ?", tenantID).
		Group("dept_id").Scan(&counts).Error
	countMap := make(map[uint64]int64, len(counts))
	for _, c := range counts {
		countMap[c.DeptID] = c.Count
	}
	nodes := make(map[uint64]*EmployeeDeptNode, len(depts))
	for _, d := range depts {
		nodes[d.ID] = &EmployeeDeptNode{ID: d.ID, ParentID: d.ParentID, Name: d.Name, Leader: d.Leader, Phone: d.Phone, Staffing: countMap[d.ID]}
	}
	roots := make([]*EmployeeDeptNode, 0, 4)
	for _, d := range depts {
		n := nodes[d.ID]
		if d.ParentID == 0 {
			roots = append(roots, n)
			continue
		}
		if parent, ok := nodes[d.ParentID]; ok {
			parent.Children = append(parent.Children, n)
		} else {
			roots = append(roots, n)
		}
	}
	return roots, nil
}

// RoleOptions 可分配角色下拉（本租户）。
func (s *Service) RoleOptions(ctx context.Context, p *principal.Info, tenantIDArg uint64) ([]OptionVO, error) {
	tenantID, err := s.OrgTenant(ctx, p, tenantIDArg)
	if err != nil {
		return nil, err
	}
	var roles []entity.Role
	tx := s.Repo.DB.WithContext(ctx).Model(&entity.Role{}).Where("status = 1 AND tenant_id = ?", tenantID)
	if err := tx.Order("sort ASC, id ASC").Limit(200).Find(&roles).Error; err != nil {
		return nil, errs.ErrDB.WithErr(err)
	}
	out := make([]OptionVO, 0, len(roles))
	for _, r := range roles {
		out = append(out, OptionVO{ID: r.ID, Name: r.Name, Code: r.Code})
	}
	return out, nil
}
