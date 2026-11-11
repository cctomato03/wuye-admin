<!-- 车辆管理：业主车/工作车/临时车 + 车位管理 -->
<template>
  <div class="art-full-height">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      <CommunitySelector @change="handleCommunityChange" />
      <ElInput v-model="query.keyword" placeholder="车牌/品牌/房屋" clearable class="!w-52" @keyup.enter="handleSearch" />
      <ElRadioGroup v-model="tab" size="default">
        <ElRadioButton label="车辆" value="vehicle" />
        <ElRadioButton label="车位" value="space" />
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

    <ElCard v-show="tab === 'space'" class="art-table-card">
      <ArtTableHeader v-model:columns="spaceColumnsChecks" :loading="spaceLoading" @refresh="refreshSpaces">
        <template #left>
          <ElButton v-auth="'property:vehicle:space'" type="primary" @click="openSpaceDialog()">新增车位</ElButton>
        </template>
      </ArtTableHeader>
      <ArtTable
        :loading="spaceLoading"
        :data="spaceData"
        :columns="spaceColumns"
        :pagination="spacePagination"
        @pagination:size-change="spaceHandleSizeChange"
        @pagination:current-change="spaceHandleCurrentChange"
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
            <ElFormItem label="绑定车位">
              <ElSelect v-model="vehicleForm.parkingSpaceId" clearable class="w-full">
                <ElOption v-for="s in spaceOptions" :key="s.id" :label="`${s.spaceNo || s.code}`" :value="s.id" />
              </ElSelect>
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

    <ElDialog v-model="spaceVisible" title="新增车位" width="520px" align-center destroy-on-close>
      <ElForm :model="spaceForm" label-width="90px">
        <ElFormItem label="车位编码" required><ElInput v-model.trim="spaceForm.code" maxlength="64" /></ElFormItem>
        <ElFormItem label="车位号"><ElInput v-model.trim="spaceForm.spaceNo" maxlength="64" /></ElFormItem>
        <ElFormItem label="类型">
          <ElSelect v-model="spaceForm.spaceType" class="w-full">
            <ElOption label="地下" :value="1" />
            <ElOption label="地上" :value="2" />
            <ElOption label="机械" :value="3" />
            <ElOption label="临时" :value="4" />
            <ElOption label="充电" :value="5" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="面积(㎡)"><ElInputNumber v-model="spaceForm.area" :min="0" :precision="2" class="!w-full" /></ElFormItem>
        <ElFormItem label="月租金"><ElInputNumber v-model="spaceForm.feeStandard" :min="0" :precision="2" class="!w-full" /></ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="spaceVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitSpace">提交</ElButton>
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
    fetchParkingSpacePage,
    fetchCreateParkingSpace,
    fetchDeleteParkingSpace,
    fetchVehicleLogPage
  } from '@/api/property'
  import { formatTime } from '@/utils/web-basic'

  defineOptions({ name: 'PropertyVehicle' })

  const { currentId } = usePropertyCommunity()
  const query = reactive<{ keyword?: string }>({})
  const tab = ref<'vehicle' | 'space' | 'log'>('vehicle')
  const submitting = ref(false)
  const vehicleVisible = ref(false)
  const spaceVisible = ref(false)
  const editingVehicle = ref<Property.Vehicle | null>(null)
  const spaceOptions = ref<Property.ParkingSpace[]>([])

  const vehicleForm = reactive<Record<string, any>>({
    plateNo: '',
    vehicleType: 1,
    houseId: '',
    parkingSpaceId: '',
    brand: '',
    model: '',
    validFrom: '',
    validTo: ''
  })
  const spaceForm = reactive<Record<string, any>>({ code: '', spaceNo: '', spaceType: 1, area: 0, feeStandard: 0 })

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
        { prop: 'parkingSpaceNo', label: '车位', width: 110 },
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
    columns: spaceColumns,
    columnChecks: spaceColumnsChecks,
    data: spaceData,
    loading: spaceLoading,
    pagination: spacePagination,
    getData: getSpaceData,
    replaceSearchParams: replaceSpaceParams,
    refreshData: refreshSpaces,
    handleSizeChange: spaceHandleSizeChange,
    handleCurrentChange: spaceHandleCurrentChange
  } = useTable({
    core: {
      apiFn: fetchParkingSpacePage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', label: '序号', width: 70 },
        { prop: 'code', label: '编码', width: 140 },
        { prop: 'spaceNo', label: '车位号', width: 120 },
        { prop: 'spaceType', label: '类型', width: 90, formatter: (row: Property.ParkingSpace) => ({ 1: '地下', 2: '地上', 3: '机械', 4: '临时', 5: '充电' })[row.spaceType] },
        { prop: 'feeStandard', label: '月租金', width: 100 },
        {
          prop: 'status',
          label: '状态',
          width: 100,
          formatter: (row: Property.ParkingSpace) =>
            h(ElTag, { size: 'small', type: row.status === 1 ? 'success' : 'warning' }, () =>
              ({ 1: '空闲', 2: '已售', 3: '已租', 4: '占用', 5: '停用' } as Record<number, string>)[row.status]
            )
        },
        {
          prop: 'operation',
          label: '操作',
          width: 90,
          formatter: (row: Property.ParkingSpace) =>
            h(ArtButtonMore, {
              list: [
                { key: 'delete', label: '删除', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: 'property:vehicle:space' }
              ],
              onClick: async (item: ButtonMoreItem) => {
                if (item.key !== 'delete') return
                try {
                  await ElMessageBox.confirm('确定删除该车位吗？', '删除确认', { type: 'warning' })
                } catch {
                  return
                }
                await fetchDeleteParkingSpace(row.id)
                refreshSpaces()
              }
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
    replaceSpaceParams({ communityId: id })
    replaceLogParams({ communityId: id })
    refreshData()
    refreshSpaces()
    refreshLogs()
    loadSpaceOptions()
  }
  const handleSearch = () => {
    replaceSearchParams({ keyword: query.keyword || undefined })
    getData()
  }

  const loadSpaceOptions = async () => {
    if (!currentId.value) return
    const res = await fetchParkingSpacePage({ communityId: currentId.value, current: 1, size: 100 })
    spaceOptions.value = res.records as any
  }


  const openVehicleDialog = (row?: Property.Vehicle) => {
    editingVehicle.value = row || null
    Object.assign(vehicleForm, {
      plateNo: row?.plateNo || '',
      vehicleType: row?.vehicleType || 1,
      houseId: row?.houseId || '',
      parkingSpaceId: row?.parkingSpaceId || '',
      brand: row?.brand || '',
      model: row?.model || '',
      validFrom: row?.validFrom?.slice(0, 10) || '',
      validTo: row?.validTo?.slice(0, 10) || ''
    })
    loadSpaceOptions()
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
      refreshSpaces()
    } finally {
      submitting.value = false
    }
  }

  const openSpaceDialog = () => {
    Object.assign(spaceForm, { code: '', spaceNo: '', spaceType: 1, area: 0, feeStandard: 0 })
    spaceVisible.value = true
  }
  const submitSpace = async () => {
    if (!spaceForm.code) return ElMessage.warning('请填写车位编码')
    submitting.value = true
    try {
      await fetchCreateParkingSpace({ ...spaceForm, communityId: currentId.value })
      spaceVisible.value = false
      refreshSpaces()
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
    if (v === 'space') refreshSpaces()
    if (v === 'log') refreshLogs()
  })
</script>
