<!-- 访客登记：登记即按受访房屋下发门禁，离场/取消自动解除 -->
<template>
  <div class="art-full-height">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      <CommunitySelector @change="handleCommunityChange" />
      <ElInput v-model="query.keyword" placeholder="姓名/手机号/访客码" clearable class="!w-52" @keyup.enter="handleSearch" />
      <ElSelect v-model="query.status" clearable placeholder="状态" class="!w-32">
        <ElOption label="待到访" :value="1" />
        <ElOption label="在访" :value="2" />
        <ElOption label="已离场" :value="3" />
        <ElOption label="已过期" :value="4" />
        <ElOption label="已取消" :value="5" />
      </ElSelect>
      <ElButton type="primary" @click="handleSearch">查询</ElButton>
    </div>

    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElButton v-auth="'property:visitor:create'" type="primary" @click="openDialog" v-ripple>访客登记</ElButton>
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

    <ElDialog v-model="dialogVisible" title="访客登记" width="700px" align-center destroy-on-close>
      <ElForm ref="formRef" :model="form" :rules="rules" label-width="100px">
        <ElRow :gutter="12">
          <ElCol :span="8">
            <ElFormItem label="姓名" prop="name"><ElInput v-model.trim="form.name" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="8">
            <ElFormItem label="手机号"><ElInput v-model.trim="form.phone" maxlength="20" /></ElFormItem>
          </ElCol>
          <ElCol :span="8">
            <ElFormItem label="证件号"><ElInput v-model.trim="form.idNo" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="受访房屋" prop="visitHouseId">
              <ElSelect v-model="form.visitHouseId" filterable remote :remote-method="searchHouses" :loading="houseLoading" class="w-full" placeholder="搜索房屋">
                <ElOption v-for="h in houseOptions" :key="h.id" :label="h.fullName" :value="h.id" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="来访类型">
              <ElSelect v-model="form.visitType" class="w-full">
                <ElOption label="访客" :value="1" />
                <ElOption label="快递" :value="2" />
                <ElOption label="家政" :value="3" />
                <ElOption label="维修" :value="4" />
                <ElOption label="其他" :value="5" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="到访时间">
              <ElDatePicker v-model="form.startTime" type="datetime" value-format="YYYY-MM-DD HH:mm:ss" class="!w-full" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="离开时间">
              <ElDatePicker v-model="form.endTime" type="datetime" value-format="YYYY-MM-DD HH:mm:ss" class="!w-full" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="随行车牌"><ElInput v-model.trim="form.vehicleNo" maxlength="20" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="来访人数"><ElInputNumber v-model="form.peopleCount" :min="1" :max="50" class="!w-full" /></ElFormItem>
          </ElCol>
          <ElCol :span="24">
            <ElFormItem label="来访事由"><ElInput v-model.trim="form.visitReason" maxlength="200" /></ElFormItem>
          </ElCol>
        </ElRow>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">登记并下发权限</ElButton>
      </template>
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
  import { fetchVisitorPage, fetchCreateVisitor, fetchChangeVisitorStatus, fetchDeleteVisitor, fetchHousePage } from '@/api/property'
  import { formatTime } from '@/utils/web-basic'

  defineOptions({ name: 'PropertyVisitor' })

  const { currentId } = usePropertyCommunity()
  const query = reactive<{ keyword?: string; status?: number }>({})
  const dialogVisible = ref(false)
  const submitting = ref(false)
  const formRef = ref<FormInstance>()
  const houseOptions = ref<Property.House[]>([])
  const houseLoading = ref(false)

  const form = reactive<Record<string, any>>({
    name: '',
    phone: '',
    idNo: '',
    visitHouseId: '',
    visitType: 1,
    startTime: '',
    endTime: '',
    vehicleNo: '',
    peopleCount: 1,
    visitReason: ''
  })
  const rules: FormRules = { name: [{ required: true, message: '请输入访客姓名', trigger: 'blur' }] }

  const { columns, columnChecks, data, loading, pagination, getData, replaceSearchParams, handleSizeChange, handleCurrentChange, refreshData } = useTable({
    core: {
      apiFn: fetchVisitorPage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', label: '序号', width: 70 },
        { prop: 'visitNo', label: '访客码', width: 170 },
        { prop: 'name', label: '姓名', width: 110 },
        { prop: 'phone', label: '手机号', width: 130 },
        { prop: 'visitHouseName', label: '受访房屋', minWidth: 150 },
        { prop: 'peopleCount', label: '人数', width: 80 },
        {
          prop: 'endTime',
          label: '预约截止',
          width: 170,
          formatter: (row: Property.Visitor) => formatTime(row.endTime)
        },
        {
          prop: 'status',
          label: '状态',
          width: 90,
          formatter: (row: Property.Visitor) => {
            const map: Record<number, { t: string; type: 'success' | 'warning' | 'danger' | 'info' | 'primary' }> = {
              1: { t: '待到访', type: 'warning' },
              2: { t: '在访', type: 'success' },
              3: { t: '已离场', type: 'info' },
              4: { t: '已过期', type: 'danger' },
              5: { t: '已取消', type: 'info' }
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
          formatter: (row: Property.Visitor) =>
            h(ArtButtonMore, {
              list: [
                { key: 'enter', label: '登记进入', icon: 'ri:login-circle-line', auth: 'property:visitor:leave' },
                { key: 'leave', label: '登记离场', icon: 'ri:logout-circle-line', auth: 'property:visitor:leave' },
                { key: 'cancel', label: '取消预约', icon: 'ri:close-circle-line', auth: 'property:visitor:leave' },
                { key: 'delete', label: '删除记录', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: 'property:visitor:delete' }
              ],
              onClick: (item: ButtonMoreItem) => handleMore(item, row)
            })
        }
      ]
    }
  })

  const handleCommunityChange = (id: string) => {
    replaceSearchParams({ communityId: id })
    refreshData()
  }
  const handleSearch = () => {
    replaceSearchParams({ keyword: query.keyword || undefined, status: query.status })
    getData()
  }

  const searchHouses = async (kw: string) => {
    if (!currentId.value) return
    houseLoading.value = true
    try {
      const res = await fetchHousePage({ communityId: currentId.value, keyword: kw, current: 1, size: 20 })
      houseOptions.value = res.records as any
    } finally {
      houseLoading.value = false
    }
  }

  const openDialog = () => {
    Object.assign(form, {
      name: '',
      phone: '',
      idNo: '',
      visitHouseId: '',
      visitType: 1,
      startTime: '',
      endTime: '',
      vehicleNo: '',
      peopleCount: 1,
      visitReason: ''
    })
    searchHouses('')
    dialogVisible.value = true
  }

  const handleSubmit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    if (!form.visitHouseId) return ElMessage.warning('请选择受访房屋')
    submitting.value = true
    try {
      await fetchCreateVisitor({ ...form, communityId: currentId.value })
      dialogVisible.value = false
      refreshData()
    } finally {
      submitting.value = false
    }
  }

  const handleMore = async (item: ButtonMoreItem, row: Property.Visitor) => {
    if (item.key === 'delete') {
      try {
        await ElMessageBox.confirm(`确定删除访客「${row.name}」的登记记录吗？`, '删除确认', { type: 'warning' })
      } catch {
        return
      }
      await fetchDeleteVisitor(row.id)
      return refreshData()
    }
    await fetchChangeVisitorStatus(row.id, item.key as 'enter' | 'leave' | 'cancel')
    refreshData()
  }
</script>
