<!-- 收费项目：物业费/水/电/停车及自定义，支持按面积·月/按面积·天/按用量/固定金额/按套 -->
<template>
  <div class="art-full-height">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      <CommunitySelector @change="handleCommunityChange" />
      <ElInput v-model="query.keyword" placeholder="项目名称/编码" clearable class="!w-48" @keyup.enter="handleSearch" />
      <ElSelect v-model="query.itemType" clearable placeholder="费用类型" class="!w-36">
        <ElOption label="物业费" :value="1" />
        <ElOption label="水费" :value="2" />
        <ElOption label="电费" :value="3" />
        <ElOption label="停车费" :value="4" />
        <ElOption label="其他" :value="5" />
      </ElSelect>
      <ElButton type="primary" @click="handleSearch">查询</ElButton>
    </div>

    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElButton v-auth="'property:fee:item:create'" type="primary" @click="openDialog()" v-ripple>新增收费项目</ElButton>
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

    <ElDialog v-model="dialogVisible" :title="editing ? '编辑收费项目' : '新增收费项目'" width="680px" align-center destroy-on-close>
      <ElForm :model="form" label-width="110px">
        <ElRow :gutter="12">
          <ElCol :span="12">
            <ElFormItem label="项目编码" required><ElInput v-model.trim="form.code" placeholder="如 WYF" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="项目名称" required><ElInput v-model.trim="form.name" maxlength="100" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="费用类型">
              <ElSelect v-model="form.itemType" class="w-full">
                <ElOption label="物业费" :value="1" />
                <ElOption label="水费" :value="2" />
                <ElOption label="电费" :value="3" />
                <ElOption label="停车费" :value="4" />
                <ElOption label="其他费用" :value="5" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="计费方式">
              <ElSelect v-model="form.calcMode" class="w-full">
                <ElOption label="按面积·月" :value="1" />
                <ElOption label="按面积·天" :value="2" />
                <ElOption label="按用量（抄表）" :value="3" />
                <ElOption label="固定金额" :value="4" />
                <ElOption label="按套" :value="5" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="默认单价">
              <ElInputNumber v-model="form.unitPrice" :min="0" :precision="4" class="!w-full" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="计价单位">
              <ElInput v-model.trim="form.unit" placeholder="元/㎡·月" maxlength="20" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="适用房屋">
              <ElSelect v-model="form.houseUsage" class="w-full">
                <ElOption label="全部" :value="0" />
                <ElOption label="住宅" :value="1" />
                <ElOption label="商业" :value="2" />
                <ElOption label="办公" :value="3" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="收费对象">
              <ElSelect v-model="form.chargeObject" class="w-full">
                <ElOption label="按房屋" :value="1" />
                <ElOption label="按单位（商铺/办公）" :value="2" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="生成日">
              <ElInputNumber v-model="form.generateDay" :min="1" :max="28" class="!w-full" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="账期天数">
              <ElInputNumber v-model="form.dueDays" :min="1" :max="180" class="!w-full" />
            </ElFormItem>
          </ElCol>
        </ElRow>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">提交</ElButton>
      </template>
    </ElDialog>

    <!-- 收费标准：按房屋用途/类型/楼栋覆写单价（住宅与商业分开计价） -->
    <ElDialog v-model="standardVisible" :title="`收费标准 · ${currentItem?.name || ''}`" width="640px" align-center destroy-on-close>
      <ElAlert
        type="info"
        :closable="false"
        class="mb-3"
        title="按「楼栋 &gt; 房屋类型 &gt; 房屋用途」优先级匹配单价；未匹配时使用项目默认单价。"
      />
      <ElSpace class="mb-2" wrap>
        <ElSelect v-model="standardForm.houseUsage" class="!w-32">
          <ElOption label="全部用途" :value="0" />
          <ElOption label="住宅" :value="1" />
          <ElOption label="商业" :value="2" />
          <ElOption label="办公" :value="3" />
        </ElSelect>
        <ElInputNumber v-model="standardForm.unitPrice" :min="0" :precision="4" placeholder="单价" />
        <ElButton type="primary" @click="submitStandard">新增标准</ElButton>
      </ElSpace>
      <ElTable :data="standards" size="small" max-height="320">
        <ElTableColumn label="适用用途" width="120">
          <template #default="{ row }">
            {{ ({ 0: '全部', 1: '住宅', 2: '商业', 3: '办公' } as Record<number, string>)[row.houseUsage] }}
          </template>
        </ElTableColumn>
        <ElTableColumn prop="unitPrice" label="单价" width="120" />
        <ElTableColumn prop="effectiveFrom" label="生效日期" />
        <ElTableColumn label="操作" width="90">
          <template #default="{ row }">
            <ElButton link type="danger" @click="removeStandard(row)">删除</ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
      <ElEmpty v-if="!standards.length" description="暂无差异化标准，使用项目默认单价" :image-size="60" />
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
  import { fetchFeeItemPage, fetchCreateFeeItem, fetchUpdateFeeItem, fetchDeleteFeeItem, fetchGetFeeStandards, fetchCreateFeeStandard, fetchDeleteFeeStandard } from '@/api/property'

  defineOptions({ name: 'PropertyFeeItem' })

  const { currentId } = usePropertyCommunity()
  const query = reactive<{ keyword?: string; itemType?: number }>({})
  const dialogVisible = ref(false)
  const editing = ref<Property.FeeItem | null>(null)
  const submitting = ref(false)
  const standardVisible = ref(false)
  const currentItem = ref<Property.FeeItem | null>(null)
  const standards = ref<Property.FeeStandard[]>([])
  const standardForm = reactive<Record<string, any>>({ houseUsage: 0, unitPrice: 0, effectiveFrom: '' })

  const form = reactive<Record<string, any>>({
    code: '',
    name: '',
    itemType: 1,
    calcMode: 1,
    unitPrice: 0,
    unit: '元/㎡·月',
    houseUsage: 0,
    chargeObject: 1,
    generateDay: 1,
    dueDays: 30
  })

  const calcText = (m: number) => ({ 1: '按面积·月', 2: '按面积·天', 3: '按用量', 4: '固定金额', 5: '按套' })[m] || '-'

  const { columns, columnChecks, data, loading, pagination, getData, replaceSearchParams, handleSizeChange, handleCurrentChange, refreshData } = useTable({
    core: {
      apiFn: fetchFeeItemPage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', label: '序号', width: 70 },
        { prop: 'name', label: '项目名称', minWidth: 150 },
        { prop: 'code', label: '编码', width: 100 },
        {
          prop: 'itemType',
          label: '类型',
          width: 100,
          formatter: (row: Property.FeeItem) =>
            h(ElTag, { size: 'small' }, () => ({ 1: '物业费', 2: '水费', 3: '电费', 4: '停车费', 5: '其他' })[row.itemType] || '-')
        },
        { prop: 'calcMode', label: '计费方式', width: 120, formatter: (row: Property.FeeItem) => calcText(row.calcMode) },
        { prop: 'unitPrice', label: '单价', width: 100 },
        { prop: 'unit', label: '单位', width: 110 },
        {
          prop: 'houseUsage',
          label: '适用房屋',
          width: 100,
          formatter: (row: Property.FeeItem) => ({ 0: '全部', 1: '住宅', 2: '商业', 3: '办公' } as Record<number, string>)[row.houseUsage]
        },
        {
          prop: 'status',
          label: '状态',
          width: 90,
          formatter: (row: Property.FeeItem) =>
            h(ElTag, { size: 'small', type: row.status === 1 ? 'success' : 'info' }, () => (row.status === 1 ? '启用' : '停用'))
        },
        {
          prop: 'operation',
          label: '操作',
          width: 90,
          fixed: 'right',
          formatter: (row: Property.FeeItem) =>
            h(ArtButtonMore, {
              list: [
                { key: 'standard', label: '收费标准', icon: 'ri:price-tag-line', auth: 'property:fee:item:update' },
                { key: 'edit', label: '编辑', icon: 'ri:edit-2-line', auth: 'property:fee:item:update' },
                { key: 'delete', label: '删除', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: 'property:fee:item:delete' }
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
    replaceSearchParams({ keyword: query.keyword || undefined, itemType: query.itemType })
    getData()
  }

  const openDialog = (row?: Property.FeeItem) => {
    editing.value = row || null
    Object.assign(form, {
      code: row?.code || '',
      name: row?.name || '',
      itemType: row?.itemType || 1,
      calcMode: row?.calcMode || 1,
      unitPrice: row?.unitPrice || 0,
      unit: row?.unit || '元/㎡·月',
      houseUsage: row?.houseUsage ?? 0,
      chargeObject: row?.chargeObject || 1,
      generateDay: row?.generateDay || 1,
      dueDays: row?.dueDays || 30
    })
    dialogVisible.value = true
  }

  const handleSubmit = async () => {    if (!form.code || !form.name) return ElMessage.warning('请填写编码与名称')
    submitting.value = true
    try {
      const payload = { ...form, communityId: currentId.value }
      if (editing.value) await fetchUpdateFeeItem(editing.value.id, payload)
      else await fetchCreateFeeItem(payload)
      dialogVisible.value = false
      refreshData()
    } finally {
      submitting.value = false
    }
  }

  const handleMore = async (item: ButtonMoreItem, row: Property.FeeItem) => {
    if (item.key === 'standard') {
      currentItem.value = row
      standardVisible.value = true
      await loadStandards()
      return
    }
    if (item.key === 'edit') return openDialog(row)
    try {
      await ElMessageBox.confirm(`确定删除收费项目「${row.name}」吗？已产生账单时不允许删除。`, '删除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchDeleteFeeItem(row.id)
    refreshData()
  }

  const loadStandards = async () => {
    if (!currentItem.value) return
    standards.value = (await fetchGetFeeStandards(currentItem.value.id)) || []
    Object.assign(standardForm, { houseUsage: 0, unitPrice: currentItem.value.unitPrice || 0, effectiveFrom: '' })
  }

  const submitStandard = async () => {
    if (!currentItem.value) return
    if (!standardForm.unitPrice) return ElMessage.warning('请填写单价')
    await fetchCreateFeeStandard({ ...standardForm, feeItemId: currentItem.value.id })
    await loadStandards()
  }

  const removeStandard = async (row: Property.FeeStandard) => {
    await fetchDeleteFeeStandard(row.id)
    loadStandards()
  }
</script>
