<!-- 缴费记录与账户：收款流水、住户余额/押金账户与充值 -->
<template>
  <div class="art-full-height">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      <CommunitySelector @change="handleCommunityChange" />
      <ElRadioGroup v-model="tab">
        <ElRadioButton label="缴费流水" value="payment" />
        <ElRadioButton label="账户余额" value="account" />
      </ElRadioGroup>
      <template v-if="tab === 'payment'">
        <ElInput v-model="query.keyword" placeholder="流水号/账单号/房屋" clearable class="!w-52" />
        <ElSelect v-model="query.payMethod" clearable placeholder="支付方式" class="!w-36">
          <ElOption label="扫码盒子" :value="1" />
          <ElOption label="小程序" :value="2" />
          <ElOption label="现金" :value="3" />
          <ElOption label="公众号" :value="4" />
          <ElOption label="二维码" :value="5" />
          <ElOption label="余额划扣" :value="6" />
          <ElOption label="其他电子" :value="7" />
          <ElOption label="押金抵扣" :value="8" />
          <ElOption label="银行转账" :value="9" />
        </ElSelect>
        <ElButton type="primary" @click="handleSearch">查询</ElButton>
      </template>
      <template v-else>
        <ElInput v-model="accountKeyword" placeholder="住户/单位名称" clearable class="!w-48" @keyup.enter="loadAccounts" />
        <ElButton type="primary" @click="loadAccounts">查询</ElButton>
      </template>
    </div>

    <ElCard v-show="tab === 'payment'" class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData" />
      <ArtTable
        :loading="loading"
        :data="data"
        :columns="columns"
        :pagination="pagination"
        @pagination:size-change="handleSizeChange"
        @pagination:current-change="handleCurrentChange"
      />
    </ElCard>

    <ElCard v-show="tab === 'account'" class="art-table-card">
      <ArtTableHeader :columns="[]" :loading="accountLoading" @refresh="loadAccounts">
        <template #left>
          <ElButton v-auth="'property:fee:bill:pay'" type="primary" @click="openRecharge">账户充值 / 缴押金</ElButton>
        </template>
      </ArtTableHeader>
      <ElTable :data="accounts" v-loading="accountLoading" size="small" max-height="520">
        <ElTableColumn prop="ownerName" label="账户主体" min-width="150" />
        <ElTableColumn label="类型" width="100">
          <template #default="{ row }">{{ row.ownerType === 1 ? '住户' : '单位' }}</template>
        </ElTableColumn>
        <ElTableColumn prop="balance" label="可用余额" width="130" />
        <ElTableColumn prop="deposit" label="押金余额" width="130" />
        <ElTableColumn label="状态" width="90">
          <template #default="{ row }">
            <ElTag :type="row.status === 1 ? 'success' : 'info'" size="small">{{ row.status === 1 ? '正常' : '停用' }}</ElTag>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElCard>

    <ElDialog v-model="rechargeVisible" title="账户充值" width="520px" align-center destroy-on-close>
      <ElForm :model="rechargeForm" label-width="100px">
        <ElFormItem label="账户类型">
          <ElSelect v-model="rechargeForm.ownerType" class="w-full">
            <ElOption label="住户" :value="1" />
            <ElOption label="单位" :value="2" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="rechargeForm.ownerType === 1 ? '住户ID' : '单位ID'">
          <ElInput v-model.trim="rechargeForm.ownerId" placeholder="对应的住户/单位记录 ID" />
        </ElFormItem>
        <ElFormItem label="充值到">
          <ElSelect v-model="rechargeForm.targetType" class="w-full">
            <ElOption label="可用余额" :value="1" />
            <ElOption label="押金" :value="2" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="金额" required>
          <ElInputNumber v-model="rechargeForm.amount" :min="0.01" :precision="2" class="!w-full" />
        </ElFormItem>
        <ElFormItem label="备注"><ElInput v-model="rechargeForm.remark" maxlength="200" /></ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="rechargeVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitRecharge">确认</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import { ElMessage, ElTag } from 'element-plus'
  import CommunitySelector from '../../components/community-selector.vue'
  import { useTable } from '@/hooks/core/useTable'
  import { usePropertyCommunity } from '@/utils/property-community'
  import { fetchPaymentPage, fetchGetAccounts, fetchRechargeAccount } from '@/api/property'
  import { formatTime } from '@/utils/web-basic'

  defineOptions({ name: 'PropertyFeePayment' })

  const { currentId } = usePropertyCommunity()
  const tab = ref<'payment' | 'account'>('payment')
  const query = reactive<{ keyword?: string; payMethod?: number }>({})
  const accountKeyword = ref('')
  const accounts = ref<Property.Account[]>([])
  const accountLoading = ref(false)
  const rechargeVisible = ref(false)
  const submitting = ref(false)
  const rechargeForm = reactive<Record<string, any>>({ ownerType: 1, ownerId: '', targetType: 1, amount: 0, remark: '' })

  const payMethodText = (m: number) =>
    ({
      1: '扫码盒子',
      2: '小程序',
      3: '现金',
      4: '公众号',
      5: '二维码',
      6: '余额划扣',
      7: '其他电子',
      8: '押金抵扣',
      9: '银行转账'
    })[m] || '-'

  const { columns, columnChecks, data, loading, pagination, getData, replaceSearchParams, handleSizeChange, handleCurrentChange, refreshData } = useTable({
    core: {
      apiFn: fetchPaymentPage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', label: '序号', width: 70 },
        { prop: 'payNo', label: '流水号', width: 190 },
        { prop: 'billNo', label: '账单号', width: 190 },
        { prop: 'houseName', label: '房屋', minWidth: 140 },
        { prop: 'amount', label: '金额', width: 100 },
        { prop: 'payMethod', label: '支付方式', width: 110, formatter: (row: Property.Payment) => payMethodText(row.payMethod) },
        {
          prop: 'status',
          label: '状态',
          width: 90,
          formatter: (row: Property.Payment) =>
            h(ElTag, { size: 'small', type: row.status === 1 ? 'success' : row.status === 3 ? 'warning' : 'danger' }, () =>
              ({ 1: '成功', 2: '失败', 3: '待确认', 4: '已退款' } as Record<number, string>)[row.status]
            )
        },
        { prop: 'operatorName', label: '收款人', width: 110 },
        { prop: 'payTime', label: '收款时间', width: 170, formatter: (row: Property.Payment) => formatTime(row.payTime) }
      ]
    }
  })

  const handleCommunityChange = (id: string) => {
    replaceSearchParams({ communityId: id })
    refreshData()
    loadAccounts()
  }
  const handleSearch = () => {
    replaceSearchParams({ keyword: query.keyword || undefined, payMethod: query.payMethod })
    getData()
  }

  const loadAccounts = async () => {
    if (!currentId.value) return
    accountLoading.value = true
    try {
      accounts.value = (await fetchGetAccounts({ communityId: currentId.value, keyword: accountKeyword.value })) || []
    } finally {
      accountLoading.value = false
    }
  }

  const openRecharge = () => {
    Object.assign(rechargeForm, { ownerType: 1, ownerId: '', targetType: 1, amount: 0, remark: '' })
    rechargeVisible.value = true
  }

  const submitRecharge = async () => {
    if (!rechargeForm.ownerId) return ElMessage.warning('请填写住户/单位 ID')
    if (!rechargeForm.amount) return ElMessage.warning('请填写金额')
    submitting.value = true
    try {
      const payload: Record<string, any> = {
        communityId: currentId.value,
        ownerType: rechargeForm.ownerType,
        targetType: rechargeForm.targetType,
        amount: rechargeForm.amount,
        remark: rechargeForm.remark
      }
      if (rechargeForm.ownerType === 1) payload.residentId = rechargeForm.ownerId
      else payload.merchantId = rechargeForm.ownerId
      await fetchRechargeAccount(payload)
      rechargeVisible.value = false
      loadAccounts()
    } finally {
      submitting.value = false
    }
  }

  watch(tab, (v) => {
    if (v === 'account') loadAccounts()
  })
</script>
