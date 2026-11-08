<!-- 门禁授权：人 × 下发类型 × 设备 的明细，支持失败重试、批量解除、门禁卡登记 -->
<template>
  <div class="art-full-height">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      <CommunitySelector @change="handleCommunityChange" />
      <ElInput v-model="query.keyword" placeholder="人员/设备/卡号/手机号" clearable class="!w-56" @keyup.enter="handleSearch" />
      <ElSelect v-model="query.personType" clearable placeholder="人员类型" class="!w-32">
        <ElOption label="住户" :value="1" />
        <ElOption label="物业人员" :value="2" />
        <ElOption label="访客" :value="3" />
        <ElOption label="其他人员" :value="4" />
      </ElSelect>
      <ElSelect v-model="query.grantType" clearable placeholder="下发类型" class="!w-32">
        <ElOption label="人脸" :value="1" />
        <ElOption label="门禁卡" :value="2" />
        <ElOption label="手机拨号" :value="3" />
      </ElSelect>
      <ElSelect v-model="query.status" clearable placeholder="下发状态" class="!w-32">
        <ElOption label="待下发" :value="1" />
        <ElOption label="成功" :value="3" />
        <ElOption label="失败" :value="4" />
        <ElOption label="已解除" :value="5" />
        <ElOption label="过期解除" :value="6" />
      </ElSelect>
      <ElButton type="primary" @click="handleSearch">查询</ElButton>
    </div>

    <div class="mb-3 grid grid-cols-2 gap-3 md:grid-cols-5">
      <ElCard v-for="item in statCards" :key="item.label" shadow="never">
        <div class="text-xs text-g-500">{{ item.label }}</div>
        <div class="mt-1 text-2xl font-semibold" :class="item.color">{{ item.value }}</div>
      </ElCard>
    </div>

    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElSpace wrap>
            <ElButton v-auth="'property:access:retry'" type="primary" :disabled="!selectedRows.length" @click="retrySelected">重试所选</ElButton>
            <ElButton v-auth="'property:access:retry'" :disabled="!currentId" @click="retryAllFailed">重试本小区失败项</ElButton>
            <ElButton v-auth="'property:access:revoke'" type="danger" plain :disabled="!selectedRows.length" @click="revokeSelected">
              解除所选
            </ElButton>
            <ElButton v-auth="'property:access:card'" @click="openCardDialog">门禁卡登记</ElButton>
          </ElSpace>
        </template>
      </ArtTableHeader>
      <ArtTable
        :loading="loading"
        :data="data"
        :columns="columns"
        :pagination="pagination"
        @selection-change="handleSelectionChange"
        @pagination:size-change="handleSizeChange"
        @pagination:current-change="handleCurrentChange"
      />
    </ElCard>

    <ElDialog v-model="cardVisible" title="门禁卡登记" width="520px" align-center destroy-on-close>
      <ElAlert type="info" :closable="false" class="mb-3" title="登记后会按人员类型自动下发刷卡权限到对应门禁设备。" />
      <ElForm :model="cardForm" label-width="90px">
        <ElFormItem label="卡号" required><ElInput v-model.trim="cardForm.cardNo" maxlength="64" /></ElFormItem>
        <ElFormItem label="人员类型">
          <ElSelect v-model="cardForm.personType" class="w-full">
            <ElOption label="住户" :value="1" />
            <ElOption label="物业人员" :value="2" />
            <ElOption label="其他人员" :value="4" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="人员ID" required>
          <ElInput v-model.trim="cardForm.personId" placeholder="住户口令：住户/其他人员的记录 ID" />
        </ElFormItem>
        <ElFormItem label="人员姓名"><ElInput v-model.trim="cardForm.personName" maxlength="64" /></ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="cardVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitCard">提交</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import { ElMessage, ElMessageBox, ElTag } from 'element-plus'
  import CommunitySelector from '../components/community-selector.vue'
  import { useTable } from '@/hooks/core/useTable'
  import { usePropertyCommunity } from '@/utils/property-community'
  import {
    fetchAccessGrantPage,
    fetchGetAccessStats,
    fetchRetryAccessGrants,
    fetchRevokeAccessGrants,
    fetchCreateAccessCard
  } from '@/api/property'
  import { formatTime } from '@/utils/web-basic'

  defineOptions({ name: 'PropertyAccess' })

  const { currentId } = usePropertyCommunity()
  const query = reactive<{
    keyword?: string
    personType?: number
    grantType?: number
    status?: number
  }>({})
  const stats = ref<Property.AccessStats | null>(null)
  const selectedRows = ref<Property.AccessGrant[]>([])
  const cardVisible = ref(false)
  const submitting = ref(false)
  const cardForm = reactive({ cardNo: '', personType: 1, personId: '', personName: '' })

  const statCards = computed(() => [
    { label: '授权总数', value: stats.value?.total ?? 0, color: '' },
    { label: '成功', value: stats.value?.success ?? 0, color: 'text-green-600' },
    { label: '待下发', value: stats.value?.pending ?? 0, color: 'text-amber-500' },
    { label: '失败', value: stats.value?.failed ?? 0, color: 'text-red-500' },
    { label: '已解除', value: (stats.value?.revoked ?? 0) + (stats.value?.expired ?? 0), color: 'text-g-500' }
  ])

  const { columns, columnChecks, data, loading, pagination, getData, replaceSearchParams, handleSizeChange, handleCurrentChange, refreshData } = useTable({
    core: {
      apiFn: fetchAccessGrantPage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'selection' },
        { prop: 'personName', label: '人员', width: 120 },
        {
          prop: 'personType',
          label: '类型',
          width: 100,
          formatter: (row: Property.AccessGrant) =>
            ({ 1: '住户', 2: '物业人员', 3: '访客', 4: '其他人员' } as Record<number, string>)[row.personType]
        },
        { prop: 'houseName', label: '来源房屋', minWidth: 120 },
        {
          prop: 'grantType',
          label: '下发类型',
          width: 100,
          formatter: (row: Property.AccessGrant) =>
            h(ElTag, { size: 'small', type: row.grantType === 1 ? 'primary' : row.grantType === 2 ? 'success' : 'warning' }, () =>
              ({ 1: '人脸', 2: '门禁卡', 3: '手机拨号' } as Record<number, string>)[row.grantType]
            )
        },
        { prop: 'deviceName', label: '设备', minWidth: 140 },
        {
          prop: 'status',
          label: '状态',
          width: 100,
          formatter: (row: Property.AccessGrant) => {
            const map: Record<number, { t: string; type: 'success' | 'warning' | 'danger' | 'info' | 'primary' }> = {
              1: { t: '待下发', type: 'warning' },
              2: { t: '下发中', type: 'primary' },
              3: { t: '成功', type: 'success' },
              4: { t: '失败', type: 'danger' },
              5: { t: '已解除', type: 'info' },
              6: { t: '过期解除', type: 'info' }
            }
            const s = map[row.status] || map[1]
            return h(ElTag, { size: 'small', type: s.type }, () => s.t)
          }
        },
        { prop: 'retryCount', label: '重试', width: 70 },
        {
          prop: 'lastMsg',
          label: '下发结果',
          minWidth: 200,
          formatter: (row: Property.AccessGrant) => row.lastMsg || '-'
        },
        { prop: 'lastIssueTime', label: '最近下发', width: 170, formatter: (row: Property.AccessGrant) => formatTime(row.lastIssueTime) }
      ]
    }
  })

  const handleCommunityChange = async (id: string) => {
    replaceSearchParams({ communityId: id })
    refreshData()
    loadStats()
  }
  const handleSearch = () => {
    replaceSearchParams({
      keyword: query.keyword || undefined,
      personType: query.personType,
      grantType: query.grantType,
      status: query.status
    })
    getData()
  }

  const loadStats = async () => {
    if (!currentId.value) return
    try {
      stats.value = await fetchGetAccessStats(currentId.value)
    } catch {
      stats.value = null
    }
  }

  const handleSelectionChange = (rows: Property.AccessGrant[]) => {
    selectedRows.value = rows
  }

  const retrySelected = async () => {
    await fetchRetryAccessGrants({ ids: selectedRows.value.map((r) => r.id) })
    refreshData()
    loadStats()
  }

  const retryAllFailed = async () => {
    await fetchRetryAccessGrants({ communityId: currentId.value })
    refreshData()
    loadStats()
  }

  const revokeSelected = async () => {
    try {
      await ElMessageBox.confirm(`确定解除所选 ${selectedRows.value.length} 条授权吗？`, '解除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchRevokeAccessGrants(selectedRows.value.map((r) => r.id))
    refreshData()
    loadStats()
  }

  const openCardDialog = () => {
    Object.assign(cardForm, { cardNo: '', personType: 1, personId: '', personName: '' })
    cardVisible.value = true
  }

  const submitCard = async () => {
    if (!cardForm.cardNo || !cardForm.personId) return ElMessage.warning('请填写卡号与人员ID')
    submitting.value = true
    try {
      await fetchCreateAccessCard({ ...cardForm, communityId: currentId.value })
      cardVisible.value = false
      refreshData()
      loadStats()
    } finally {
      submitting.value = false
    }
  }
</script>
