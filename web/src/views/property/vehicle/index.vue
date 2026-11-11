<!-- 车辆管理：业主车/工作车/临时车 + 车位管理 -->
<template>
  <div class="art-full-height">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      <CommunitySelector @change="handleCommunityChange" />
      <ElInput v-model="query.keyword" placeholder="车牌/品牌/房屋" clearable class="!w-52" @keyup.enter="handleSearch" />
      <ElRadioGroup v-model="tab" size="default">
        <ElRadioButton label="车辆" value="vehicle" />
        <ElRadioButton label="进出记录" value="log" />
      </ElRadioGroup>
    </div>

    <ElCard v-show="tab === 'vehicle'" class="art-table-card">
      <ArtTableHeader v-model:columns="vehicleColumnsChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElButton v-auth="'property:vehicle:create'" type="primary" @click="openVehicleDialog()" v-ripple>登记车辆</ElButton>
        </template>
      </ArtTableHeader>
      <ArtTable
        :loading="loading"
        :data="data"
        :columns="vehicleColumns"
        :pagination="pagination"
        @pagination:size-change="handleSizeChange"
        @pagination:current-change="handleCurrentChange"
      />
    </ElCard>

    <ElCard v-show="tab === 'log'" class="art-table-card">
      <ArtTableHeader :columns="[]" :loading="logLoading" @refresh="refreshLogs" />
      <ArtTable
        :loading="logLoading"
        :data="logData"
        :columns="logColumns"
        :pagination="logPagination"
        @pagination:size-change="logHandleSizeChange"
        @pagination:current-change="logHandleCurrentChange"
      />
    </ElCard>

    <ElDialog v-model="vehicleVisible" :title="editingVehicle ? '编辑车辆' : '登记车辆'" width="640px" align-center destroy-on-close>
      <ElForm :model="vehicleForm" label-width="100px">
        <ElRow :gutter="12">
          <ElCol :span="12">
            <ElFormItem label="车牌号" required><ElInput v-model.trim="vehicleForm.plateNo" maxlength="20" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="车辆类型">
              <ElSelect v-model="vehicleForm.vehicleType" class="w-full">
                <ElOption label="业主车" :value="1" />
                <ElOption label="工作车" :value="2" />
                <ElOption label="临时车" :value="3" />
                <ElOption label="访客车" :value="4" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="关联房屋">
              <HouseCascader v-model="vehicleForm.houseId" placeholder="按层级选择房屋" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="品牌"><ElInput v-model.trim="vehicleForm.brand" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="型号"><ElInput v-model.trim="vehicleForm.model" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="有效期开始">
              <ElDatePicker v-model="vehicleForm.validFrom" type="date" value-format="YYYY-MM-DD" class="!w-full" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="有效期结束">
              <ElDatePicker v-model="vehicleForm.validTo" type="date" value-format="YYYY-MM-DD" class="!w-full" />
            </ElFormItem>
          </ElCol>
        </ElRow>
      </ElForm>
      <template #footer>
        <ElButton @click="vehicleVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitVehicle">提交</ElButton>
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
  import {
    fetchVehiclePage,
    fetchCreateVehicle,
    fetchUpdateVehicle,
    fetchDeleteVehicle,
    fetchVehicleLogPage
  } from '@/api/property'
  import { formatTime } from '@/utils/web-basic'

  defineOptions({ name: 'PropertyVehicle' })

  const { currentId } = usePropertyCommunity()
  const query = reactive<{ keyword?: string }>({})
  const tab = ref<'vehicle' | 'log'>('vehicle')
  const submitting = ref(false)
  const vehicleVisible = ref(false)
  const editingVehicle = ref<Property.Vehicle | null>(null)

  const vehicleForm = reactive<Record<string, any>>({
    plateNo: '',
    vehicleType: 1,
    houseId: '',
    brand: '',
    model: '',
    validFrom: '',
    validTo: ''
  })

  const { columns: vehicleColumns, columnChecks: vehicleColumnsChecks, data, loading, pagination, getData, replaceSearchParams, handleSizeChange, handleCurrentChange, refreshData } = useTable({
    core: {
      apiFn: fetchVehiclePage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', label: '序号', width: 70 },
        { prop: 'plateNo', label: '车牌号', width: 130 },
        {
          prop: 'vehicleType',
          label: '类型',
          width: 100,
          formatter: (row: Property.Vehicle) =>
            h(ElTag, { size: 'small' }, () => ({ 1: '业主车', 2: '工作车', 3: '临时车', 4: '访客车' })[row.vehicleType] || '-')
        },
        { prop: 'houseName', label: '关联房屋', minWidth: 140 },
        { prop: 'brand', label: '品牌', width: 100 },
        {
          prop: 'validTo',
          label: '有效期至',
          width: 120,
          formatter: (row: Property.Vehicle) => row.validTo?.slice(0, 10) || '长期'
        },
        {
          prop: 'status',
          label: '状态',
          width: 90,
          formatter: (row: Property.Vehicle) =>
            h(ElTag, { size: 'small', type: row.status === 1 ? 'success' : 'info' }, () =>
              ({ 1: '有效', 2: '停用', 3: '已过期' } as Record<number, string>)[row.status]
            )
        },
        {
          prop: 'operation',
          label: '操作',
          width: 90,
          fixed: 'right',
          formatter: (row: Property.Vehicle) =>
            h(ArtButtonMore, {
              list: [
                { key: 'edit', label: '编辑', icon: 'ri:edit-2-line', auth: 'property:vehicle:update' },
                { key: 'delete', label: '删除', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: 'property:vehicle:delete' }
              ],
              onClick: (item: ButtonMoreItem) => handleVehicleMore(item, row)
            })
        }
      ]
    }
  })

  const {
    columns: logColumns,
    data: logData,
    loading: logLoading,
    pagination: logPagination,
    replaceSearchParams: replaceLogParams,
    refreshData: refreshLogs,
    handleSizeChange: logHandleSizeChange,
    handleCurrentChange: logHandleCurrentChange
  } = useTable({
    core: {
      apiFn: fetchVehicleLogPage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { prop: 'plateNo', label: '车牌号', width: 130 },
        { prop: 'direction', label: '方向', width: 80, formatter: (row: Property.VehicleLog) => (row.direction === 1 ? '入场' : '出场') },
        { prop: 'passTime', label: '时间', width: 180, formatter: (row: Property.VehicleLog) => formatTime(row.passTime) },
        { prop: 'gateName', label: '道闸', width: 140 },
        { prop: 'amount', label: '收费', width: 90 }
      ]
    }
  })

  const handleCommunityChange = (id: string) => {
    replaceSearchParams({ communityId: id })
    replaceLogParams({ communityId: id })
    refreshData()
    refreshLogs()
  }
  const handleSearch = () => {
    replaceSearchParams({ keyword: query.keyword || undefined })
    getData()
  }




  const openVehicleDialog = (row?: Property.Vehicle) => {
    editingVehicle.value = row || null
    Object.assign(vehicleForm, {
      plateNo: row?.plateNo || '',
      vehicleType: row?.vehicleType || 1,
      houseId: row?.houseId || '',
      brand: row?.brand || '',
      model: row?.model || '',
      validFrom: row?.validFrom?.slice(0, 10) || '',
      validTo: row?.validTo?.slice(0, 10) || ''
    })
    vehicleVisible.value = true
  }

  const submitVehicle = async () => {
    if (!vehicleForm.plateNo) return ElMessage.warning('请填写车牌号')
    submitting.value = true
    try {
      const payload = { ...vehicleForm, communityId: currentId.value }
      if (editingVehicle.value) await fetchUpdateVehicle(editingVehicle.value.id, payload)
      else await fetchCreateVehicle(payload)
      vehicleVisible.value = false
      refreshData()
    } finally {
      submitting.value = false
    }
  }




  const handleVehicleMore = async (item: ButtonMoreItem, row: Property.Vehicle) => {
    if (item.key === 'edit') return openVehicleDialog(row)
    try {
      await ElMessageBox.confirm(`确定删除车辆「${row.plateNo}」吗？`, '删除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchDeleteVehicle(row.id)
    refreshData()
  }

  watch(tab, (v) => {
    if (v === 'log') refreshLogs()
  })
</script>
