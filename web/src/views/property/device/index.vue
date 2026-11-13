<!-- 设备管理：门禁机/道闸/监控/人脸终端，绑定到小区/区域/楼栋/单元/楼层，支持 Excel 导入 -->
<template>
  <div class="art-full-height">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      <CommunitySelector @change="handleCommunityChange" />
      <ElInput v-model="query.keyword" placeholder="编码/名称/序列号" clearable class="!w-52" @keyup.enter="handleSearch" />
      <ElSelect v-model="query.deviceType" clearable placeholder="设备类型" class="!w-36">
        <ElOption label="门禁机" :value="1" />
        <ElOption label="车辆道闸" :value="2" />
        <ElOption label="视频监控" :value="3" />
        <ElOption label="人脸终端" :value="4" />
        <ElOption label="可视对讲" :value="5" />
      </ElSelect>
      <ElButton type="primary" @click="handleSearch">查询</ElButton>
    </div>

    <div class="mb-3 grid grid-cols-2 gap-3 md:grid-cols-4">
      <ElCard v-for="item in statCards" :key="item.label" shadow="never">
        <div class="text-xs text-g-500">{{ item.label }}</div>
        <div class="mt-1 text-2xl font-semibold">{{ item.value }}</div>
      </ElCard>
    </div>

    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElSpace wrap>
            <ElButton v-auth="'property:device:create'" type="primary" @click="openDialog()" v-ripple>新增设备</ElButton>
            <ElButton v-auth="'property:device:import'" :loading="importing" @click="triggerImport">导入 Excel</ElButton>
            <ElButton v-auth="'property:device:import'" text @click="downloadTemplate">下载模板</ElButton>
            <input ref="fileInputRef" type="file" accept=".xlsx,.xls" class="hidden" @change="handleImport" />
          </ElSpace>
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

    <ElDialog v-model="dialogVisible" :title="editing ? '编辑设备' : '新增设备'" width="660px" align-center destroy-on-close>
      <ElForm :model="form" label-width="100px">
        <ElRow :gutter="12">
          <ElCol :span="12">
            <ElFormItem label="设备编码" required><ElInput v-model.trim="form.code" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="设备名称" required><ElInput v-model.trim="form.name" maxlength="100" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="设备类型">
              <ElSelect v-model="form.deviceType" class="w-full">
                <ElOption label="门禁机" :value="1" />
                <ElOption label="车辆道闸" :value="2" />
                <ElOption label="视频监控" :value="3" />
                <ElOption label="人脸终端" :value="4" />
                <ElOption label="可视对讲" :value="5" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="序列号"><ElInput v-model.trim="form.serialNo" maxlength="100" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="品牌"><ElInput v-model.trim="form.brand" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="型号"><ElInput v-model.trim="form.model" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="IP 地址"><ElInput v-model.trim="form.ip" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="安装位置"><ElInput v-model.trim="form.location" maxlength="200" /></ElFormItem>
          </ElCol>
          <ElCol :span="8">
            <ElFormItem label="绑定范围">
              <ElSelect v-model="form.bindLevel" class="w-full">
                <ElOption label="小区级" :value="1" />
                <ElOption label="指定层级" :value="2" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="16">
            <ElFormItem label="绑定层级">
              <ElSelect v-model="form.bindNodeId" clearable filterable class="w-full" :disabled="form.bindLevel === 1" placeholder="选择层级节点">
                <ElOption v-for="n in bindTargets" :key="n.id" :label="n.name" :value="n.id" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="参与门禁">
              <ElSwitch v-model="form.accessEnabled" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="状态">
              <ElSelect v-model="form.status" class="w-full">
                <ElOption label="正常" :value="1" />
                <ElOption label="停用" :value="2" />
                <ElOption label="故障" :value="3" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
        </ElRow>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">提交</ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="importResultVisible" title="导入结果" width="540px" align-center>
      <ElAlert
        v-if="importResult"
        :type="importResult.failed ? 'warning' : 'success'"
        :closable="false"
        show-icon
        :title="`共 ${importResult.total} 行，成功 ${importResult.success} 行，失败 ${importResult.failed} 行`"
      />
      <ul v-if="importResult?.errors?.length" class="mt-2 max-h-60 overflow-auto text-xs text-red-500">
        <li v-for="(msg, i) in importResult.errors" :key="i">{{ msg }}</li>
      </ul>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import { ElMessage, ElMessageBox, ElTag } from 'element-plus'
  import CommunitySelector from '../components/community-selector.vue'
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTable } from '@/hooks/core/useTable'
  import { usePropertyCommunity } from '@/utils/property-community'
  import {
    fetchDevicePage,
    fetchCreateDevice,
    fetchUpdateDevice,
    fetchDeleteDevice,
    fetchGetDeviceSummary,
    fetchNodeOptions
  } from '@/api/property'
  import { downloadFile, uploadBusinessFile } from '@/utils/web-basic'

  defineOptions({ name: 'PropertyDevice' })

  const { currentId } = usePropertyCommunity()
  const query = reactive<{ keyword?: string; deviceType?: number }>({})
  const dialogVisible = ref(false)
  const editing = ref<Property.Device | null>(null)
  const submitting = ref(false)
  const importing = ref(false)
  const importResultVisible = ref(false)
  const importResult = ref<{ total: number; success: number; failed: number; errors: string[] }>()
  const fileInputRef = ref<HTMLInputElement>()
  const stats = ref<Property.DeviceStats | null>(null)
  const bindTargets = ref<Property.Option[]>([])

  const form = reactive<Record<string, any>>({
    code: '',
    name: '',
    deviceType: 1,
    serialNo: '',
    brand: '',
    model: '',
    ip: '',
    location: '',
    bindLevel: 1,
    bindNodeId: '',
    accessEnabled: true,
    status: 1
  })

  const statCards = computed(() => [
    { label: '设备总数', value: stats.value?.total ?? 0 },
    { label: '门禁设备', value: stats.value?.access ?? 0 },
    { label: '在线', value: stats.value?.online ?? 0 },
    { label: '故障', value: stats.value?.maintain ?? 0 }
  ])

  const { columns, columnChecks, data, loading, pagination, getData, replaceSearchParams, handleSizeChange, handleCurrentChange, refreshData } = useTable({
    core: {
      apiFn: fetchDevicePage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', label: '序号', width: 70 },
        { prop: 'code', label: '设备编码', width: 130 },
        { prop: 'name', label: '设备名称', minWidth: 150 },
        {
          prop: 'deviceType',
          label: '类型',
          width: 110,
          formatter: (row: Property.Device) =>
            h(ElTag, { size: 'small' }, () => ({ 1: '门禁机', 2: '道闸', 3: '监控', 4: '人脸终端', 5: '对讲', 9: '其他' })[row.deviceType] || '-')
        },
        {
          prop: 'bindLevel',
          label: '绑定位置',
          width: 130,
          formatter: (row: any) => (row.bindNodeId && row.bindNodeId !== '0' ? (row.bindName || '层级') : '小区级')
        },
        { prop: 'ip', label: 'IP', width: 130 },
        {
          prop: 'online',
          label: '在线',
          width: 90,
          formatter: (row: Property.Device) =>
            h(ElTag, { size: 'small', type: row.online === 1 ? 'success' : 'info' }, () => (row.online === 1 ? '在线' : '离线'))
        },
        {
          prop: 'status',
          label: '状态',
          width: 90,
          formatter: (row: Property.Device) =>
            h(ElTag, { size: 'small', type: row.status === 1 ? 'success' : row.status === 3 ? 'danger' : 'info' }, () =>
              ({ 1: '正常', 2: '停用', 3: '故障' } as Record<number, string>)[row.status]
            )
        },
        {
          prop: 'operation',
          label: '操作',
          width: 90,
          fixed: 'right',
          formatter: (row: Property.Device) =>
            h(ArtButtonMore, {
              list: [
                { key: 'edit', label: '编辑', icon: 'ri:edit-2-line', auth: 'property:device:update' },
                { key: 'delete', label: '删除', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: 'property:device:delete' }
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
    replaceSearchParams({ keyword: query.keyword || undefined, deviceType: query.deviceType })
    getData()
  }

  const loadRefs = async () => {
    if (!currentId.value) return
    try {
      stats.value = await fetchGetDeviceSummary(currentId.value)
    } catch {
      stats.value = null
    }
    bindTargets.value = (await fetchNodeOptions(currentId.value, false)) || []
  }

  const openDialog = (row?: Property.Device) => {
    editing.value = row || null
    Object.assign(form, {
      code: row?.code || '',
      name: row?.name || '',
      deviceType: row?.deviceType || 1,
      serialNo: row?.serialNo || '',
      brand: row?.brand || '',
      model: row?.model || '',
      ip: row?.ip || '',
      location: row?.location || '',
      bindLevel: row?.bindNodeId ? 2 : 1,
      bindNodeId: row?.bindNodeId || '',
      accessEnabled: row?.accessEnabled ?? true,
      status: row?.status || 1
    })
    dialogVisible.value = true
  }

  const handleSubmit = async () => {
    if (!form.code || !form.name) return ElMessage.warning('请填写设备编码与名称')
    submitting.value = true
    try {
      const payload = { ...form, communityId: currentId.value, bindNodeId: form.bindLevel === 1 ? undefined : form.bindNodeId }
      if (editing.value) await fetchUpdateDevice(editing.value.id, payload)
      else await fetchCreateDevice(payload)
      dialogVisible.value = false
      refreshData()
      loadRefs()
    } finally {
      submitting.value = false
    }
  }

  const handleMore = async (item: ButtonMoreItem, row: Property.Device) => {
    if (item.key === 'edit') return openDialog(row)
    try {
      await ElMessageBox.confirm(`确定删除设备「${row.name}」吗？其门禁授权会一并解除。`, '删除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchDeleteDevice(row.id)
    refreshData()
    loadRefs()
  }

  const triggerImport = () => fileInputRef.value?.click()
  const downloadTemplate = () => downloadFile('/api/v1/property/devices/import-template', {}, '设备导入模板.xlsx')
  const handleImport = async (e: Event) => {
    const input = e.target as HTMLInputElement
    const file = input.files?.[0]
    input.value = ''
    if (!file || !currentId.value) return
    importing.value = true
    try {
      const res = await uploadBusinessFile<{ total: number; success: number; failed: number; errors: string[] }>(
        `/api/v1/property/devices/import?communityId=${currentId.value}`,
        file
      )
      importResult.value = res
      importResultVisible.value = true
      refreshData()
      loadRefs()
    } finally {
      importing.value = false
    }
  }
</script>
