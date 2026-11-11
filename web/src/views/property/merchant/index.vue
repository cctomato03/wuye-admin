<!-- 单位管理：办公单位 / 商铺，关联商业楼栋与房屋 -->
<template>
  <div class="art-full-height">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      <CommunitySelector @change="handleCommunityChange" />
      <ElInput v-model="query.keyword" placeholder="单位名称/联系人/电话" clearable class="!w-52" @keyup.enter="handleSearch" />
      <ElSelect v-model="query.merchantType" clearable placeholder="类型" class="!w-32">
        <ElOption label="办公" :value="1" />
        <ElOption label="商铺" :value="2" />
        <ElOption label="学校" :value="3" />
        <ElOption label="医疗" :value="4" />
        <ElOption label="其他" :value="5" />
      </ElSelect>
      <ElButton type="primary" @click="handleSearch">查询</ElButton>
    </div>

    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElButton v-auth="'property:merchant:create'" type="primary" @click="openDialog()" v-ripple>登记单位</ElButton>
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

    <ElDialog v-model="dialogVisible" :title="editing ? '编辑单位' : '登记单位'" width="680px" align-center destroy-on-close>
      <ElForm :model="form" label-width="100px">
        <ElRow :gutter="12">
          <ElCol :span="12">
            <ElFormItem label="单位名称" required><ElInput v-model.trim="form.name" maxlength="100" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="类型">
              <ElSelect v-model="form.merchantType" class="w-full">
                <ElOption label="办公" :value="1" />
                <ElOption label="商铺" :value="2" />
                <ElOption label="学校" :value="3" />
                <ElOption label="医疗" :value="4" />
                <ElOption label="其他" :value="5" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="所在房屋">
              <HouseCascader v-model="form.houseId" placeholder="按层级选择房屋" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="经营面积(㎡)"><ElInputNumber v-model="form.area" :min="0" :precision="2" class="!w-full" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="联系人"><ElInput v-model.trim="form.contactName" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="联系电话"><ElInput v-model.trim="form.contactPhone" maxlength="20" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="所属行业"><ElInput v-model.trim="form.industry" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="营业执照号"><ElInput v-model.trim="form.businessLicense" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="入驻日期">
              <ElDatePicker v-model="form.startDate" type="date" value-format="YYYY-MM-DD" class="!w-full" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="到期日期">
              <ElDatePicker v-model="form.endDate" type="date" value-format="YYYY-MM-DD" class="!w-full" />
            </ElFormItem>
          </ElCol>
        </ElRow>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">提交</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import HouseCascader from '../components/house-cascader.vue'
  import { ElMessage, ElMessageBox, ElTag } from 'element-plus'
  import CommunitySelector from '../components/community-selector.vue'
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTable } from '@/hooks/core/useTable'
  import { usePropertyCommunity } from '@/utils/property-community'
  import { fetchMerchantPage, fetchCreateMerchant, fetchUpdateMerchant, fetchDeleteMerchant } from '@/api/property'

  defineOptions({ name: 'PropertyMerchant' })

  const { currentId } = usePropertyCommunity()
  const query = reactive<{ keyword?: string; merchantType?: number }>({})
  const dialogVisible = ref(false)
  const editing = ref<Property.Merchant | null>(null)
  const submitting = ref(false)

  const form = reactive<Record<string, any>>({
    name: '',
    merchantType: 2,
    houseId: '',
    area: 0,
    contactName: '',
    contactPhone: '',
    industry: '',
    businessLicense: '',
    startDate: '',
    endDate: ''
  })

  const { columns, columnChecks, data, loading, pagination, getData, replaceSearchParams, handleSizeChange, handleCurrentChange, refreshData } = useTable({
    core: {
      apiFn: fetchMerchantPage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', label: '序号', width: 70 },
        { prop: 'name', label: '单位名称', minWidth: 170 },
        {
          prop: 'merchantType',
          label: '类型',
          width: 90,
          formatter: (row: Property.Merchant) =>
            h(ElTag, { size: 'small' }, () => ({ 1: '办公', 2: '商铺', 3: '学校', 4: '医疗', 5: '其他' })[row.merchantType] || '-')
        },
        { prop: 'houseName', label: '所在房屋', minWidth: 140 },
        { prop: 'contactName', label: '联系人', width: 110 },
        { prop: 'contactPhone', label: '联系电话', width: 130 },
        { prop: 'area', label: '面积(㎡)', width: 100 },
        {
          prop: 'status',
          label: '状态',
          width: 90,
          formatter: (row: Property.Merchant) =>
            h(ElTag, { size: 'small', type: row.status === 1 ? 'success' : 'info' }, () =>
              ({ 1: '在营', 2: '停业', 3: '已退场' } as Record<number, string>)[row.status]
            )
        },
        {
          prop: 'operation',
          label: '操作',
          width: 90,
          fixed: 'right',
          formatter: (row: Property.Merchant) =>
            h(ArtButtonMore, {
              list: [
                { key: 'edit', label: '编辑', icon: 'ri:edit-2-line', auth: 'property:merchant:update' },
                { key: 'delete', label: '删除', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: 'property:merchant:delete' }
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
    replaceSearchParams({ keyword: query.keyword || undefined, merchantType: query.merchantType })
    getData()
  }



  const openDialog = (row?: Property.Merchant) => {
    editing.value = row || null
    Object.assign(form, {
      name: row?.name || '',
      merchantType: row?.merchantType || 2,
      houseId: row?.houseId || '',
      area: row?.area || 0,
      contactName: row?.contactName || '',
      contactPhone: row?.contactPhone || '',
      industry: row?.industry || '',
      businessLicense: row?.businessLicense || '',
      startDate: row?.startDate?.slice(0, 10) || '',
      endDate: row?.endDate?.slice(0, 10) || ''
    })
    dialogVisible.value = true
  }

  const handleSubmit = async () => {
    if (!form.name) return ElMessage.warning('请填写单位名称')
    submitting.value = true
    try {
      const payload = { ...form, communityId: currentId.value }
      if (editing.value) await fetchUpdateMerchant(editing.value.id, payload)
      else await fetchCreateMerchant(payload)
      dialogVisible.value = false
      refreshData()
    } finally {
      submitting.value = false
    }
  }

  const handleMore = async (item: ButtonMoreItem, row: Property.Merchant) => {
    if (item.key === 'edit') return openDialog(row)
    try {
      await ElMessageBox.confirm(`确定删除单位「${row.name}」吗？`, '删除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchDeleteMerchant(row.id)
    refreshData()
  }
</script>
