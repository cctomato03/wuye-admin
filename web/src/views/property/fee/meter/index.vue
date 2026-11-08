<!-- 抄表管理：水电表档案、逐表录入读数、按用量生成账单 -->
<template>
  <div class="art-full-height">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      <CommunitySelector @change="handleCommunityChange" />
      <ElRadioGroup v-model="tab">
        <ElRadioButton label="水表/电表" value="meter" />
        <ElRadioButton label="抄表记录" value="reading" />
      </ElRadioGroup>
      <template v-if="tab === 'reading'">
        <ElDatePicker v-model="period" type="month" value-format="YYYYMM" class="!w-36" placeholder="账期" />
        <ElButton type="primary" @click="refreshReadings">查询</ElButton>
        <ElSelect v-model="waterItemId" placeholder="选择水/电费项目" class="!w-52" filterable>
          <ElOption v-for="i in feeItems" :key="i.id" :label="i.name" :value="i.id" />
        </ElSelect>
        <ElButton v-auth="'property:fee:meter:generate'" type="success" @click="generateBills">生成账单</ElButton>
      </template>
    </div>

    <ElCard v-show="tab === 'meter'" class="art-table-card">
      <ArtTableHeader v-model:columns="meterColumnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElSpace>
            <ElButton v-auth="'property:fee:meter:create'" type="primary" @click="openMeterDialog">新增水表/电表</ElButton>
          </ElSpace>
        </template>
      </ArtTableHeader>
      <ArtTable
        :loading="loading"
        :data="data"
        :columns="meterColumns"
        :pagination="pagination"
        @pagination:size-change="handleSizeChange"
        @pagination:current-change="handleCurrentChange"
      />
    </ElCard>

    <ElCard v-show="tab === 'reading'" class="art-table-card">
      <ArtTableHeader v-model:columns="readingColumnChecks" :loading="readingLoading" @refresh="refreshReadings" />
      <ArtTable
        :loading="readingLoading"
        :data="readingData"
        :columns="readingColumns"
        :pagination="readingPagination"
        @pagination:size-change="readingHandleSizeChange"
        @pagination:current-change="readingHandleCurrentChange"
      />
    </ElCard>

    <ElDialog v-model="meterVisible" title="新增水表/电表" width="540px" align-center destroy-on-close>
      <ElForm :model="meterForm" label-width="100px">
        <ElFormItem label="表类型">
          <ElSelect v-model="meterForm.meterType" class="w-full">
            <ElOption label="水表" :value="1" />
            <ElOption label="电表" :value="2" />
            <ElOption label="燃气表" :value="3" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="表号" required><ElInput v-model.trim="meterForm.meterNo" maxlength="64" /></ElFormItem>
        <ElFormItem label="安装房屋">
          <ElSelect v-model="meterForm.houseId" filterable remote :remote-method="searchHouses" class="w-full" clearable>
            <ElOption v-for="h in houseOptions" :key="h.id" :label="h.fullName" :value="h.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="倍率"><ElInputNumber v-model="meterForm.ratio" :min="0.01" :precision="2" class="!w-full" /></ElFormItem>
        <ElFormItem label="初始读数"><ElInputNumber v-model="meterForm.initReading" :min="0" :precision="2" class="!w-full" /></ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="meterVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitMeter">提交</ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="readingVisible" title="录入抄表" width="520px" align-center destroy-on-close>
      <ElForm :model="readingForm" label-width="100px">
        <ElFormItem label="表">

          <span>{{ currentMeter?.meterNo }}（上次读数 {{ currentMeter?.lastReading }}）</span>
        </ElFormItem>
        <ElFormItem label="账期" required><ElInput v-model.trim="readingForm.period" placeholder="YYYYMM" maxlength="6" /></ElFormItem>
        <ElFormItem label="本期读数" required>
          <ElInputNumber v-model="readingForm.currReading" :min="0" :precision="2" class="!w-full" />
        </ElFormItem>
        <ElFormItem label="单价">
          <ElInputNumber v-model="readingForm.unitPrice" :min="0" :precision="4" class="!w-full" />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="readingVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitReading">保存</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import { ElMessage, ElMessageBox, ElTag } from 'element-plus'
  import CommunitySelector from '../../components/community-selector.vue'
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTable } from '@/hooks/core/useTable'
  import { usePropertyCommunity } from '@/utils/property-community'
  import {
    fetchMeterPage,
    fetchCreateMeter,
    fetchDeleteMeter,
    fetchMeterReadingPage,
    fetchCreateMeterReading,
    fetchGenerateMeterBills,
    fetchGetFeeItems,
    fetchHousePage
  } from '@/api/property'
  import { formatTime } from '@/utils/web-basic'

  defineOptions({ name: 'PropertyFeeMeter' })

  const { currentId } = usePropertyCommunity()
  const tab = ref<'meter' | 'reading'>('meter')
  const period = ref(new Date().toISOString().slice(0, 7).replace('-', ''))
  const waterItemId = ref('')
  const feeItems = ref<Property.FeeItem[]>([])
  const houseOptions = ref<Property.House[]>([])
  const meterVisible = ref(false)
  const readingVisible = ref(false)
  const submitting = ref(false)
  const currentMeter = ref<Property.Meter | null>(null)
  const meterForm = reactive<Record<string, any>>({ meterType: 1, meterNo: '', houseId: '', ratio: 1, initReading: 0 })
  const readingForm = reactive<Record<string, any>>({ period: period.value, currReading: 0, unitPrice: 0 })

  const { columns: meterColumns, columnChecks: meterColumnChecks, data, loading, pagination, getData, replaceSearchParams, handleSizeChange, handleCurrentChange, refreshData } = useTable({
    core: {
      apiFn: fetchMeterPage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', label: '序号', width: 70 },
        { prop: 'meterNo', label: '表号', width: 150 },
        {
          prop: 'meterType',
          label: '类型',
          width: 90,
          formatter: (row: Property.Meter) => ({ 1: '水表', 2: '电表', 3: '燃气表' } as Record<number, string>)[row.meterType]
        },
        { prop: 'houseName', label: '安装房屋', minWidth: 160 },
        { prop: 'ratio', label: '倍率', width: 80 },
        { prop: 'lastReading', label: '最近读数', width: 110 },
        { prop: 'lastReadTime', label: '最近抄表', width: 170, formatter: (row: Property.Meter) => formatTime(row.lastReadTime) },
        {
          prop: 'operation',
          label: '操作',
          width: 110,
          fixed: 'right',
          formatter: (row: Property.Meter) =>
            h(ArtButtonMore, {
              list: [
                { key: 'read', label: '录入读数', icon: 'ri:edit-2-line', auth: 'property:fee:meter:reading' },
                { key: 'delete', label: '删除', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: 'property:fee:meter:create' }
              ],
              onClick: (item: ButtonMoreItem) => handleMeterMore(item, row)
            })
        }
      ]
    }
  })

  const {
    columns: readingColumns,
    columnChecks: readingColumnChecks,
    data: readingData,
    loading: readingLoading,
    pagination: readingPagination,
    getData: getReadingData,
    replaceSearchParams: replaceReadingParams,
    refreshData: refreshReadings,
    handleSizeChange: readingHandleSizeChange,
    handleCurrentChange: readingHandleCurrentChange
  } = useTable({
    core: {
      apiFn: fetchMeterReadingPage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { prop: 'period', label: '账期', width: 90 },
        { prop: 'houseName', label: '房屋', minWidth: 150 },
        { prop: 'prevReading', label: '上期', width: 100 },
        { prop: 'currReading', label: '本期', width: 100 },
        { prop: 'usageAmount', label: '用量', width: 100 },
        { prop: 'unitPrice', label: '单价', width: 90 },
        { prop: 'amount', label: '金额', width: 100 },
        {
          prop: 'status',
          label: '状态',
          width: 100,
          formatter: (row: Property.MeterReading) =>
            h(ElTag, { size: 'small', type: row.status === 2 ? 'success' : 'warning' }, () =>
              ({ 1: '已抄表', 2: '已出账', 3: '已作废' } as Record<number, string>)[row.status]
            )
        },
        { prop: 'readerName', label: '抄表人', width: 110 }
      ]
    }
  })

  const handleCommunityChange = async (id: string) => {
    replaceSearchParams({ communityId: id })
    replaceReadingParams({ communityId: id, period: period.value })
    refreshData()
    refreshReadings()
    loadFeeItems()
  }

  const loadFeeItems = async () => {
    if (!currentId.value) return
    const res = await fetchGetFeeItems(currentId.value)
    feeItems.value = (res?.list || []).filter((i) => i.calcMode === 3)
  }

  const searchHouses = async (kw: string) => {
    if (!currentId.value) return
    const res = await fetchHousePage({ communityId: currentId.value, keyword: kw, current: 1, size: 20 })
    houseOptions.value = res.records as any
  }

  const openMeterDialog = () => {
    Object.assign(meterForm, { meterType: 1, meterNo: '', houseId: '', ratio: 1, initReading: 0 })
    searchHouses('')
    meterVisible.value = true
  }

  const submitMeter = async () => {
    if (!meterForm.meterNo) return ElMessage.warning('请填写表号')
    submitting.value = true
    try {
      await fetchCreateMeter({ ...meterForm, communityId: currentId.value })
      meterVisible.value = false
      refreshData()
    } finally {
      submitting.value = false
    }
  }

  const handleMeterMore = async (item: ButtonMoreItem, row: Property.Meter) => {
    if (item.key === 'read') {
      currentMeter.value = row
      Object.assign(readingForm, { period: period.value, currReading: row.lastReading, unitPrice: 0 })
      readingVisible.value = true
      return
    }
    try {
      await ElMessageBox.confirm('确定删除该水表吗？', '删除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchDeleteMeter(row.id)
    refreshData()
  }

  const submitReading = async () => {
    if (!currentMeter.value) return
    if (!/^\d{6}$/.test(readingForm.period)) return ElMessage.warning('账期格式应为 YYYYMM')
    submitting.value = true
    try {
      await fetchCreateMeterReading({ ...readingForm, meterId: currentMeter.value.id })
      readingVisible.value = false
      refreshData()
      refreshReadings()
    } finally {
      submitting.value = false
    }
  }

  const generateBills = async () => {
    if (!waterItemId.value) return ElMessage.warning('请选择水/电费收费项目')
    const res = await fetchGenerateMeterBills({ communityId: currentId.value, feeItemId: waterItemId.value, period: period.value })
    ElMessage.success(`生成 ${res.created} 条水电账单，合计 ${res.amount} 元`)
    refreshReadings()
  }

  watch(tab, (v) => {
    if (v === 'reading') refreshReadings()
  })

  onMounted(loadFeeItems)
</script>
