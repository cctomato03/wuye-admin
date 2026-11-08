<!-- 账单管理：按月批量生成、收费（9 种支付方式）、作废、催缴汇总 -->
<template>
  <div class="art-full-height">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      <CommunitySelector @change="handleCommunityChange" />
      <ElDatePicker v-model="query.billMonth" type="month" value-format="YYYYMM" placeholder="账期" class="!w-36" />
      <ElSelect v-model="query.status" clearable placeholder="状态" class="!w-32">
        <ElOption label="未缴" :value="1" />
        <ElOption label="部分缴" :value="2" />
        <ElOption label="已缴" :value="3" />
        <ElOption label="已作废" :value="5" />
      </ElSelect>
      <ElInput v-model="query.keyword" placeholder="房号/账单号/住户" clearable class="!w-48" />
      <ElButton type="primary" @click="handleSearch">查询</ElButton>
      <ElButton v-auth="'property:fee:bill:generate'" type="success" @click="generateVisible = true">生成账单</ElButton>
    </div>

    <div class="mb-3 grid grid-cols-2 gap-3 md:grid-cols-4">
      <ElCard v-for="item in summaryCards" :key="item.label" shadow="never">
        <div class="text-xs text-g-500">{{ item.label }}</div>
        <div class="mt-1 text-2xl font-semibold">{{ item.value }}</div>
      </ElCard>
    </div>

    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <span class="text-sm text-g-500">共 {{ pagination.total }} 条账单</span>
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

    <!-- 生成账单 -->
    <ElDialog v-model="generateVisible" title="生成账单" width="560px" align-center destroy-on-close>
      <ElAlert type="info" :closable="false" class="mb-3" title="按收费项目批量生成，已存在的账单会自动跳过（可重复执行）。按用量计费的项目请在「抄表管理」生成。" />
      <ElForm :model="genForm" label-width="100px">
        <ElFormItem label="收费项目" required>
          <ElSelect v-model="genForm.feeItemId" class="w-full" filterable>
            <ElOption v-for="i in feeItems" :key="i.id" :label="`${i.name}（${i.unitPrice} ${i.unit || ''}）`" :value="i.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="账期" required>
          <ElDatePicker v-model="genForm.billMonth" type="month" value-format="YYYYMM" class="!w-full" />
        </ElFormItem>
        <ElFormItem label="试算">
          <ElCheckbox v-model="genForm.dryRun">只试算不写库</ElCheckbox>
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="generateVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitGenerate">生成</ElButton>
      </template>
    </ElDialog>

    <!-- 收费 -->
    <ElDialog v-model="payVisible" title="账单收费" width="520px" align-center destroy-on-close>
      <ElDescriptions v-if="paying" :column="2" border size="small" class="mb-3">
        <ElDescriptionsItem label="账单号">{{ paying.billNo }}</ElDescriptionsItem>
        <ElDescriptionsItem label="房屋">{{ paying.houseName }}</ElDescriptionsItem>
        <ElDescriptionsItem label="应收">{{ paying.amount }}</ElDescriptionsItem>
        <ElDescriptionsItem label="已收">{{ paying.paidAmount }}</ElDescriptionsItem>
      </ElDescriptions>
      <ElForm :model="payForm" label-width="100px">
        <ElFormItem label="支付方式">
          <ElSelect v-model="payForm.payMethod" class="w-full">
            <ElOption label="扫码盒子" :value="1" />
            <ElOption label="小程序支付" :value="2" />
            <ElOption label="现金" :value="3" />
            <ElOption label="公众号支付" :value="4" />
            <ElOption label="二维码支付" :value="5" />
            <ElOption label="余额划扣" :value="6" />
            <ElOption label="其他电子支付" :value="7" />
            <ElOption label="押金抵扣" :value="8" />
            <ElOption label="银行转账" :value="9" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="收款金额">
          <ElInputNumber v-model="payForm.amount" :min="0" :precision="2" class="!w-full" placeholder="留空=全额" />
        </ElFormItem>
        <ElFormItem label="备注"><ElInput v-model="payForm.remark" maxlength="200" /></ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="payVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitPay">确认收款</ElButton>
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
    fetchBillPage,
    fetchGenerateBills,
    fetchPayBill,
    fetchCancelBill,
    fetchGetBillSummary,
    fetchGetFeeItems
  } from '@/api/property'

  defineOptions({ name: 'PropertyFeeBill' })

  const { currentId } = usePropertyCommunity()
  const query = reactive<{ billMonth?: string; status?: number; keyword?: string }>({
    billMonth: new Date().toISOString().slice(0, 7).replace('-', '')
  })
  const summary = ref<Property.BillSummary | null>(null)
  const feeItems = ref<Property.FeeItem[]>([])
  const generateVisible = ref(false)
  const payVisible = ref(false)
  const submitting = ref(false)
  const paying = ref<Property.Bill | null>(null)
  const genForm = reactive<Record<string, any>>({
    feeItemId: '',
    billMonth: new Date().toISOString().slice(0, 7).replace('-', ''),
    dryRun: false
  })
  const payForm = reactive<Record<string, any>>({ payMethod: 3, amount: undefined, remark: '' })

  const summaryCards = computed(() => [
    { label: '应收合计', value: summary.value?.receivable ?? 0 },
    { label: '已收合计', value: summary.value?.received ?? 0 },
    { label: '欠费合计', value: summary.value?.outstanding ?? 0 },
    { label: '未缴账单', value: summary.value?.unpaid ?? 0 }
  ])

  const { columns, columnChecks, data, loading, pagination, getData, replaceSearchParams, handleSizeChange, handleCurrentChange, refreshData } = useTable({
    core: {
      apiFn: fetchBillPage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', label: '序号', width: 70 },
        { prop: 'billNo', label: '账单号', width: 200 },
        { prop: 'billMonth', label: '账期', width: 90 },
        { prop: 'feeItemName', label: '收费项目', width: 110 },
        { prop: 'houseName', label: '房屋/单位', minWidth: 150, formatter: (row: Property.Bill) => row.houseName || row.merchantName || '-' },
        { prop: 'quantity', label: '数量', width: 90 },
        { prop: 'unitPrice', label: '单价', width: 90 },
        { prop: 'amount', label: '应收', width: 100 },
        { prop: 'paidAmount', label: '已收', width: 100 },
        {
          prop: 'status',
          label: '状态',
          width: 90,
          formatter: (row: Property.Bill) => {
            const map: Record<number, { t: string; type: 'success' | 'warning' | 'danger' | 'info' | 'primary' }> = {
              1: { t: '未缴', type: 'danger' },
              2: { t: '部分缴', type: 'warning' },
              3: { t: '已缴', type: 'success' },
              4: { t: '已减免', type: 'info' },
              5: { t: '已作废', type: 'info' }
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
          formatter: (row: Property.Bill) =>
            h(ArtButtonMore, {
              list: [
                { key: 'pay', label: '收费', icon: 'ri:money-cny-circle-line', auth: 'property:fee:bill:pay' },
                { key: 'cancel', label: '作废', icon: 'ri:close-circle-line', color: '#f56c6c', auth: 'property:fee:bill:cancel' }
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
    loadSummary()
    loadFeeItems()
  }
  const handleSearch = () => {
    replaceSearchParams({
      communityId: currentId.value,
      billMonth: query.billMonth || undefined,
      status: query.status,
      keyword: query.keyword || undefined
    })
    getData()
    loadSummary()
  }

  const loadSummary = async () => {
    if (!currentId.value) return
    try {
      summary.value = await fetchGetBillSummary({ communityId: currentId.value, billMonth: query.billMonth })
    } catch {
      summary.value = null
    }
  }

  const loadFeeItems = async () => {
    if (!currentId.value) return
    const res = await fetchGetFeeItems(currentId.value)
    feeItems.value = res?.list || []
  }

  const submitGenerate = async () => {
    if (!genForm.feeItemId || !genForm.billMonth) return ElMessage.warning('请选择收费项目与账期')
    submitting.value = true
    try {
      const res = await fetchGenerateBills({ ...genForm, communityId: currentId.value })
      generateVisible.value = false
      ElMessage.success(`生成 ${res.created} 条，跳过 ${res.skipped} 条，合计 ${res.amount} 元`)
      refreshData()
      loadSummary()
    } finally {
      submitting.value = false
    }
  }

  const handleMore = async (item: ButtonMoreItem, row: Property.Bill) => {
    if (item.key === 'pay') {
      paying.value = row
      Object.assign(payForm, { payMethod: 3, amount: undefined, remark: '' })
      payVisible.value = true
      return
    }
    try {
      await ElMessageBox.confirm(`确定作废账单「${row.billNo}」吗？`, '作废确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchCancelBill(row.id)
    refreshData()
    loadSummary()
  }

  const submitPay = async () => {
    if (!paying.value) return
    submitting.value = true
    try {
      await fetchPayBill(paying.value.id, { ...payForm } as any)
      payVisible.value = false
      refreshData()
      loadSummary()
    } finally {
      submitting.value = false
    }
  }

  onMounted(() => {
    loadFeeItems()
    loadSummary()
  })
</script>
