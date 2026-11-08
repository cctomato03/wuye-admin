<!-- 其他人员：保洁/维修/施工等临时人员，登记时直接勾选门禁设备，有效期到自动解除 -->
<template>
  <div class="art-full-height">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      <CommunitySelector @change="handleCommunityChange" />
      <ElInput v-model="query.keyword" placeholder="姓名/手机号/单位" clearable class="!w-52" @keyup.enter="handleSearch" />
      <ElSelect v-model="query.categoryId" clearable placeholder="人员类别" class="!w-36">
        <ElOption v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
      </ElSelect>
      <ElButton type="primary" @click="handleSearch">查询</ElButton>
      <ElButton @click="categoryVisible = true">类别管理</ElButton>
    </div>

    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElButton v-auth="'property:other:create'" type="primary" @click="openDialog()" v-ripple>登记人员</ElButton>
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

    <ElDialog v-model="dialogVisible" :title="editing ? '编辑人员' : '登记其他人员'" width="680px" align-center destroy-on-close>
      <ElForm ref="formRef" :model="form" :rules="rules" label-width="100px">
        <ElRow :gutter="12">
          <ElCol :span="12">
            <ElFormItem label="姓名" prop="name"><ElInput v-model.trim="form.name" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="人员类别">
              <ElSelect v-model="form.categoryId" clearable class="w-full">
                <ElOption v-for="c in categories" :key="c.id" :label="c.name" :value="c.id" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="手机号"><ElInput v-model.trim="form.phone" maxlength="20" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="证件号"><ElInput v-model.trim="form.idNo" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="所属单位"><ElInput v-model.trim="form.companyName" maxlength="100" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="门禁卡号"><ElInput v-model.trim="form.cardNo" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="有效期开始">
              <ElDatePicker v-model="form.validFrom" type="date" value-format="YYYY-MM-DD" class="!w-full" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="有效期结束">
              <ElDatePicker v-model="form.validTo" type="date" value-format="YYYY-MM-DD" class="!w-full" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="24">
            <ElFormItem label="作业内容"><ElInput v-model.trim="form.workContent" maxlength="200" /></ElFormItem>
          </ElCol>
          <ElCol :span="24">
            <ElFormItem label="门禁设备">
              <ElSelect v-model="form.deviceIds" multiple filterable class="w-full" placeholder="选择需要开通的门禁设备">
                <ElOption v-for="d in devices" :key="d.id" :label="`${d.name}${d.extra ? '（' + d.extra + '）' : ''}`" :value="d.id" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
        </ElRow>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">提交并下发</ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="categoryVisible" title="人员类别" width="560px" align-center destroy-on-close>
      <ElSpace class="mb-2">
        <ElInput v-model="newCategory" placeholder="类别名称，如 保洁/电梯维保" class="!w-56" maxlength="64" />
        <ElButton type="primary" @click="addCategory">新增</ElButton>
      </ElSpace>
      <ElTable :data="categories" size="small" max-height="320">
        <ElTableColumn prop="name" label="类别" />
        <ElTableColumn label="默认有效期(天)" width="140">
          <template #default="{ row }">{{ row.defaultDays }}</template>
        </ElTableColumn>
        <ElTableColumn label="操作" width="80">
          <template #default="{ row }">
            <ElButton link type="danger" @click="removeCategory(row)">删除</ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import { ElMessage, ElMessageBox, ElTag, type FormInstance, type FormRules } from 'element-plus'
  import CommunitySelector from '../components/community-selector.vue'
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTable } from '@/hooks/core/useTable'
  import { usePropertyCommunity } from '@/utils/property-community'
  import {
    fetchOtherPersonPage,
    fetchCreateOtherPerson,
    fetchUpdateOtherPerson,
    fetchDeleteOtherPerson,
    fetchGetPersonCategories,
    fetchCreatePersonCategory,
    fetchDeletePersonCategory,
    fetchGetDeviceOptions
  } from '@/api/property'

  defineOptions({ name: 'PropertyOtherPerson' })

  const { currentId } = usePropertyCommunity()
  const query = reactive<{ keyword?: string; categoryId?: string }>({})
  const categories = ref<Property.PersonCategory[]>([])
  const devices = ref<Property.Option[]>([])
  const dialogVisible = ref(false)
  const categoryVisible = ref(false)
  const editing = ref<Property.OtherPerson | null>(null)
  const submitting = ref(false)
  const formRef = ref<FormInstance>()
  const newCategory = ref('')

  const form = reactive<Record<string, any>>({
    name: '',
    categoryId: '',
    phone: '',
    idNo: '',
    companyName: '',
    cardNo: '',
    validFrom: '',
    validTo: '',
    workContent: '',
    deviceIds: []
  })
  const rules: FormRules = { name: [{ required: true, message: '请输入姓名', trigger: 'blur' }] }

  const { columns, columnChecks, data, loading, pagination, getData, replaceSearchParams, handleSizeChange, handleCurrentChange, refreshData } = useTable({
    core: {
      apiFn: fetchOtherPersonPage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', label: '序号', width: 70 },
        { prop: 'name', label: '姓名', width: 110 },
        { prop: 'categoryName', label: '类别', width: 110 },
        { prop: 'phone', label: '手机号', width: 130 },
        { prop: 'companyName', label: '所属单位', minWidth: 150 },
        { prop: 'workContent', label: '作业内容', minWidth: 140 },
        {
          prop: 'validTo',
          label: '有效期至',
          width: 130,
          formatter: (row: Property.OtherPerson) => row.validTo?.slice(0, 10) || '-'
        },
        {
          prop: 'deviceIds',
          label: '门禁设备',
          width: 100,
          formatter: (row: Property.OtherPerson) => `${row.deviceIds?.length || 0} 台`
        },
        {
          prop: 'status',
          label: '状态',
          width: 90,
          formatter: (row: Property.OtherPerson) => {
            const map: Record<number, { t: string; type: 'success' | 'warning' | 'danger' | 'info' | 'primary' }> = {
              1: { t: '有效', type: 'success' },
              2: { t: '停用', type: 'info' },
              3: { t: '已过期', type: 'danger' }
            }
            const s = map[row.status] || map[1]
            return h(ElTag, { size: 'small', type: s.type }, () => s.t)
          }
        },
        {
          prop: 'operation',
          label: '操作',
          width: 90,
          fixed: 'right',
          formatter: (row: Property.OtherPerson) =>
            h(ArtButtonMore, {
              list: [
                { key: 'edit', label: '编辑', icon: 'ri:edit-2-line', auth: 'property:other:update' },
                { key: 'delete', label: '删除', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: 'property:other:delete' }
              ],
              onClick: (item: ButtonMoreItem) => handleMore(item, row)
            })
        }
      ]
    }
  })

  const handleCommunityChange = async (id: string) => {
    replaceSearchParams({ communityId: id })
    refreshData()
    await loadRefs()
  }
  const handleSearch = () => {
    replaceSearchParams({ keyword: query.keyword || undefined, categoryId: query.categoryId })
    getData()
  }

  const loadRefs = async () => {
    if (!currentId.value) return
    categories.value = (await fetchGetPersonCategories(currentId.value)) || []
    devices.value = (await fetchGetDeviceOptions(currentId.value)) || []
  }

  const openDialog = (row?: Property.OtherPerson) => {
    editing.value = row || null
    Object.assign(form, {
      name: row?.name || '',
      categoryId: row?.categoryId || '',
      phone: row?.phone || '',
      idNo: row?.idNo || '',
      companyName: row?.companyName || '',
      cardNo: row?.cardNo || '',
      validFrom: row?.validFrom?.slice(0, 10) || '',
      validTo: row?.validTo?.slice(0, 10) || '',
      workContent: row?.workContent || '',
      deviceIds: row?.deviceIds || []
    })
    dialogVisible.value = true
  }

  const handleSubmit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    submitting.value = true
    try {
      const payload = { ...form, communityId: currentId.value }
      if (editing.value) await fetchUpdateOtherPerson(editing.value.id, payload)
      else await fetchCreateOtherPerson(payload)
      dialogVisible.value = false
      refreshData()
    } finally {
      submitting.value = false
    }
  }

  const handleMore = async (item: ButtonMoreItem, row: Property.OtherPerson) => {
    if (item.key === 'edit') return openDialog(row)
    try {
      await ElMessageBox.confirm(`确定删除「${row.name}」吗？会同时解除其门禁权限。`, '删除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchDeleteOtherPerson(row.id)
    refreshData()
  }

  const addCategory = async () => {
    if (!newCategory.value) return
    await fetchCreatePersonCategory({ communityId: currentId.value, name: newCategory.value })
    newCategory.value = ''
    loadRefs()
  }
  const removeCategory = async (row: Property.PersonCategory) => {
    await fetchDeletePersonCategory(row.id)
    loadRefs()
  }
</script>
