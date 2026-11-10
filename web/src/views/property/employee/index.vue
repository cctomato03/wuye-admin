<!-- 员工与组织：左侧组织架构（部门树），右侧员工列表；新增员工可同时创建登录账号与小区授权 -->
<template>
  <div class="art-full-height flex gap-3">
    <ElCard class="!w-64 shrink-0">
      <div class="mb-2 font-medium">组织架构</div>
      <ElTree :data="deptTree" node-key="id" :props="{ label: 'name', children: 'children' }" default-expand-all @node-click="handleDeptClick">
        <template #default="{ data }">
          <span class="flex items-center justify-between gap-1">
            <span>{{ data.name }}</span>
            <ElTag size="small" type="info">{{ data.staffing }}人</ElTag>
          </span>
        </template>
      </ElTree>
    </ElCard>

    <ElCard class="art-table-card flex-1">
      <OrgTenantSelector class="mb-2" @change="handleTenantChange" />
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElSpace wrap>
            <ElButton v-auth="'property:employee:create'" type="primary" @click="openDialog()" v-ripple>新增员工</ElButton>
            <ElButton @click="clearDeptFilter" v-if="query.deptId">清除部门筛选</ElButton>
          </ElSpace>
        </template>
      </ArtTableHeader>
      <ArtTable
        :loading="loading"
        :data="data"
        :columns="columns"
        :pagination="pagination"
        @pagination:size-change="handleSizeChange"
        @pagination:current-change="handleCurrentChange"
      />
    </ElCard>

    <ElDialog v-model="dialogVisible" :title="editing ? '编辑员工' : '新增员工'" width="720px" align-center destroy-on-close>
      <ElForm ref="formRef" :model="form" :rules="rules" label-width="110px">
        <ElRow :gutter="12">
          <ElCol :span="12">
            <ElFormItem label="姓名" prop="name"><ElInput v-model.trim="form.name" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="工号"><ElInput v-model.trim="form.employeeNo" placeholder="留空自动生成" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="所属部门">
              <ElTreeSelect v-model="form.deptId" :data="deptTree" :props="{ label: 'name', value: 'id', children: 'children' }" check-strictly class="w-full" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="主责小区">
              <ElSelect v-model="form.communityId" filterable class="w-full">
                <ElOption v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="岗位"><ElInput v-model.trim="form.position" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="手机号"><ElInput v-model.trim="form.phone" maxlength="20" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="用工类型">
              <ElSelect v-model="form.empType" class="w-full">
                <ElOption label="正式" :value="1" />
                <ElOption label="外包" :value="2" />
                <ElOption label="临时" :value="3" />
                <ElOption label="实习" :value="4" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="入职日期">
              <ElDatePicker v-model="form.entryDate" type="date" value-format="YYYY-MM-DD" class="!w-full" />
            </ElFormItem>
          </ElCol>
          <template v-if="!editing">
            <ElCol :span="12">
              <ElFormItem label="登录账号"><ElInput v-model.trim="form.username" placeholder="留空则不建账号" maxlength="32" /></ElFormItem>
            </ElCol>
            <ElCol :span="12">
              <ElFormItem label="初始密码"><ElInput v-model.trim="form.password" placeholder="留空随机生成" maxlength="32" /></ElFormItem>
            </ElCol>
            <ElCol :span="12">
              <ElFormItem label="角色">
                <ElSelect v-model="form.roleIds" multiple class="w-full">
                  <ElOption v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
                </ElSelect>
              </ElFormItem>
            </ElCol>
            <ElCol :span="12">
              <ElFormItem label="授权小区">
                <ElSelect v-model="form.communityIds" multiple filterable class="w-full">
                  <ElOption v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
                </ElSelect>
              </ElFormItem>
            </ElCol>
          </template>
          <ElCol :span="12">
            <ElFormItem label="状态">
              <ElSelect v-model="form.status" class="w-full">
                <ElOption label="在职" :value="1" />
                <ElOption label="离职" :value="2" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
        </ElRow>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">提交</ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="grantVisible" title="小区授权" width="520px" align-center destroy-on-close>
      <ElAlert
        type="warning"
        :closable="false"
        class="mb-3"
        title="物业人员只能看到被授权小区的数据；撤销授权后立即生效（无需重新登录）。"
      />
      <ElSelect v-model="grantIds" multiple filterable class="w-full" placeholder="选择可管理的小区">
        <ElOption v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
      </ElSelect>
      <template #footer>
        <ElButton @click="grantVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitGrant">保存</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import OrgTenantSelector from '@/views/system/components/org-tenant-selector.vue'
  import { ElMessage, ElMessageBox, ElTag, type FormInstance, type FormRules } from 'element-plus'
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTable } from '@/hooks/core/useTable'
  import { usePropertyCommunity } from '@/utils/property-community'
  import {
    fetchEmployeePage,
    fetchCreateEmployee,
    fetchUpdateEmployee,
    fetchDeleteEmployee,
    fetchGrantEmployeeCommunities,
    fetchGetEmployeeDeptTree,
    fetchGetEmployeeRoles
  } from '@/api/property'

  defineOptions({ name: 'PropertyEmployee' })

  const { options: communities, loadCommunities } = usePropertyCommunity()
  const query = reactive<{ deptId?: string }>({})
  const deptTree = ref<Property.DeptNode[]>([])
  const roles = ref<Property.Option[]>([])
  const dialogVisible = ref(false)
  const grantVisible = ref(false)
  const editing = ref<Property.Employee | null>(null)
  const granting = ref<Property.Employee | null>(null)
  const grantIds = ref<string[]>([])
  const submitting = ref(false)
  const formRef = ref<FormInstance>()

  const form = reactive<Record<string, any>>({
    name: '',
    employeeNo: '',
    deptId: '',
    communityId: '',
    position: '',
    phone: '',
    empType: 1,
    entryDate: '',
    username: '',
    password: '',
    roleIds: [],
    communityIds: [],
    status: 1
  })
  const rules: FormRules = { name: [{ required: true, message: '请输入姓名', trigger: 'blur' }] }

  const handleTenantChange = () => {
    refreshData()
    fetchGetEmployeeDeptTree().then((list) => (deptTree.value = list || []))
    fetchGetEmployeeRoles().then((list) => (roles.value = list || []))
  }

  const { columns, columnChecks, data, loading, pagination, getData, replaceSearchParams, handleSizeChange, handleCurrentChange, refreshData } = useTable({
    core: {
      apiFn: fetchEmployeePage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', label: '序号', width: 70 },
        { prop: 'name', label: '姓名', width: 110 },
        { prop: 'employeeNo', label: '工号', width: 140 },
        { prop: 'deptName', label: '部门', width: 130 },
        { prop: 'position', label: '岗位', width: 120 },
        { prop: 'phone', label: '手机号', width: 130 },
        { prop: 'username', label: '登录账号', width: 130 },
        {
          prop: 'communityNames',
          label: '授权小区',
          minWidth: 160,
          formatter: (row: Property.Employee) =>
            row.communityNames?.length ? row.communityNames.join('、') : h('span', { class: 'text-red-500' }, '未授权')
        },
        {
          prop: 'status',
          label: '状态',
          width: 90,
          formatter: (row: Property.Employee) =>
            h(ElTag, { type: row.status === 1 ? 'success' : 'info', size: 'small' }, () => (row.status === 1 ? '在职' : '离职'))
        },
        {
          prop: 'operation',
          label: '操作',
          width: 90,
          fixed: 'right',
          formatter: (row: Property.Employee) =>
            h(ArtButtonMore, {
              list: [
                { key: 'edit', label: '编辑', icon: 'ri:edit-2-line', auth: 'property:employee:update' },
                { key: 'grant', label: '小区授权', icon: 'ri:shield-check-line', auth: 'property:employee:grant' },
                { key: 'delete', label: '删除', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: 'property:employee:delete' }
              ],
              onClick: (item: ButtonMoreItem) => handleMore(item, row)
            })
        }
      ]
    }
  })

  const handleDeptClick = (node: Property.DeptNode) => {
    query.deptId = node.id
    replaceSearchParams({ deptId: node.id })
    getData()
  }
  const clearDeptFilter = () => {
    query.deptId = undefined
    replaceSearchParams({ deptId: undefined })
    getData()
  }

  const openDialog = (row?: Property.Employee) => {
    editing.value = row || null
    Object.assign(form, {
      name: row?.name || '',
      employeeNo: row?.employeeNo || '',
      deptId: row?.deptId || '',
      communityId: row?.communityId || '',
      position: row?.position || '',
      phone: row?.phone || '',
      empType: row?.empType || 1,
      entryDate: row?.entryDate?.slice(0, 10) || '',
      username: '',
      password: '',
      roleIds: [],
      communityIds: row?.communityIds || [],
      status: row?.status || 1
    })
    dialogVisible.value = true
  }

  const handleSubmit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    // 新员工要么不建账号（只建档），要么必须选角色：权限来自角色，没角色登录后什么都看不到
    if (!editing.value && form.username && !form.roleIds.length) {
      ElMessage.warning('请为新员工选择角色（可在「组织与权限 → 角色管理」先创建）')
      return
    }
    submitting.value = true
    try {
      if (editing.value) await fetchUpdateEmployee(editing.value.id, { ...form })
      else await fetchCreateEmployee({ ...form })
      dialogVisible.value = false
      refreshData()
    } finally {
      submitting.value = false
    }
  }

  const handleMore = async (item: ButtonMoreItem, row: Property.Employee) => {
    if (item.key === 'edit') return openDialog(row)
    if (item.key === 'grant') {
      granting.value = row
      grantIds.value = row.communityIds || []
      grantVisible.value = true
      return
    }
    try {
      await ElMessageBox.confirm(`确定删除员工「${row.name}」吗？会同时停用其登录账号与门禁权限。`, '删除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchDeleteEmployee(row.id)
    refreshData()
  }

  const submitGrant = async () => {
    if (!granting.value) return
    submitting.value = true
    try {
      await fetchGrantEmployeeCommunities(granting.value.id, {
        communityIds: grantIds.value,
        defaultId: grantIds.value[0]
      })
      grantVisible.value = false
      refreshData()
    } finally {
      submitting.value = false
    }
  }

  onMounted(async () => {
    await loadCommunities()
    deptTree.value = (await fetchGetEmployeeDeptTree()) || []
    roles.value = (await fetchGetEmployeeRoles()) || []
  })
</script>
