<!-- 房屋管理：房屋挂在层级节点上，支持按层级筛选、批量生成与 Excel 导入 -->
<template>
  <div class="art-full-height flex gap-3">
    <ElCard class="!w-72 shrink-0">
      <div class="mb-2"><CommunitySelector @change="handleCommunityChange" /></div>
      <ElTree
        :data="nodeTree"
        node-key="id"
        :props="{ label: 'name', children: 'children' }"
        highlight-current
        default-expand-all
        @node-click="handleNodeClick"
        class="mb-2"
      >
        <template #default="{ data }">
          <span class="flex items-center gap-1">
            <span>{{ data.name }}</span>
            <ElTag size="small" type="info">{{ data.houseCount }}</ElTag>
          </span>
        </template>
      </ElTree>
      <ElButton size="small" text @click="clearNode">显示全部房屋</ElButton>
    </ElCard>

    <ElCard class="art-table-card flex-1">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElSpace wrap>
            <ElButton v-auth="'property:structure:create'" type="primary" @click="openDialog()">新增房屋</ElButton>
            <ElButton v-auth="'property:structure:create'" @click="batchVisible = true">批量生成</ElButton>
            <ElButton v-auth="'property:structure:import'" :loading="importing" @click="triggerImport">导入 Excel</ElButton>
            <ElButton v-auth="'property:structure:import'" text @click="downloadTemplate">下载模板</ElButton>
            <ElButton @click="typeVisible = true">房屋类型</ElButton>
            <input ref="fileInputRef" type="file" accept=".xlsx,.xls" class="hidden" @change="handleImport" />
          </ElSpace>
        </template>
      </ArtTableHeader>
      <div class="mb-2 px-4 text-sm text-g-500">当前层级：{{ currentNodeName || '全部' }}</div>
      <ArtTable
        :loading="loading"
        :data="data"
        :columns="columns"
        :pagination="pagination"
        @pagination:size-change="handleSizeChange"
        @pagination:current-change="handleCurrentChange"
      />
    </ElCard>

    <ElDialog v-model="dialogVisible" :title="editing ? '编辑房屋' : '新增房屋'" width="640px" align-center destroy-on-close>
      <ElForm :model="form" label-width="100px">
        <ElRow :gutter="12">
          <ElCol :span="12">
            <ElFormItem label="所属层级" required>
              <ElSelect v-model="form.nodeId" filterable class="w-full" placeholder="选择房屋所在层级">
                <ElOption v-for="n in nodeOptions" :key="n.id" :label="n.name" :value="n.id" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="房屋编码" required><ElInput v-model.trim="form.code" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="房号" required><ElInput v-model.trim="form.houseNo" placeholder="如 101" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="房屋用途">
              <ElSelect v-model="form.usageType" class="w-full">
                <ElOption label="住宅" :value="1" />
                <ElOption label="商业" :value="2" />
                <ElOption label="办公" :value="3" />
                <ElOption label="车位" :value="4" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="建筑面积"><ElInputNumber v-model="form.areaBuild" :min="0" :precision="2" class="!w-full" controls-position="right" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="套内面积"><ElInputNumber v-model="form.areaUse" :min="0" :precision="2" class="!w-full" controls-position="right" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="房屋类型">
              <ElSelect v-model="form.houseTypeId" clearable class="w-full">
                <ElOption v-for="t in houseTypes" :key="t.id" :label="t.name" :value="t.id" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="房屋状态">
              <ElSelect v-model="form.houseStatus" class="w-full">
                <ElOption label="空置" :value="1" />
                <ElOption label="自住" :value="2" />
                <ElOption label="出租" :value="3" />
                <ElOption label="装修中" :value="4" />
                <ElOption label="停用" :value="5" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="朝向"><ElInput v-model.trim="form.orientation" maxlength="20" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="户型"><ElInput v-model.trim="form.layout" maxlength="64" /></ElFormItem>
          </ElCol>
        </ElRow>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submit">提交</ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="batchVisible" title="批量生成房屋" width="560px" align-center destroy-on-close>
      <ElAlert type="info" :closable="false" class="mb-3" title="在选中的层级下顺序生成：编码=前缀-序号，全称=层级全称+序号。" />
      <ElForm :model="batchForm" label-width="100px">
        <ElFormItem label="所属层级" required>
          <ElSelect v-model="batchForm.nodeId" filterable class="w-full">
            <ElOption v-for="n in nodeOptions" :key="n.id" :label="n.name" :value="n.id" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem label="生成数量"><ElInputNumber v-model="batchForm.count" :min="1" :max="5000" class="!w-full" /></ElFormItem>
        <ElFormItem label="起始序号"><ElInputNumber v-model="batchForm.startNo" :min="1" :max="9999" class="!w-full" /></ElFormItem>
        <ElFormItem label="编码前缀"><ElInput v-model.trim="batchForm.codePrefix" placeholder="留空用层级编码" maxlength="64" /></ElFormItem>
        <ElFormItem label="建筑面积"><ElInputNumber v-model="batchForm.areaBuild" :min="0" :precision="2" class="!w-full" /></ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="batchVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitBatch">生成</ElButton>
      </template>
    </ElDialog>

    <ElDialog v-model="typeVisible" title="房屋类型" width="620px" align-center destroy-on-close>
      <ElSpace class="mb-2">
        <ElInput v-model="newTypeName" placeholder="类型名称，如 三室两厅" maxlength="64" class="!w-56" />
        <ElButton type="primary" @click="addHouseType">新增</ElButton>
      </ElSpace>
      <ElTable :data="houseTypes" size="small" max-height="320">
        <ElTableColumn prop="name" label="名称" />
        <ElTableColumn prop="layout" label="户型" />
        <ElTableColumn label="操作" width="90">
          <template #default="{ row }">
            <ElButton type="danger" link @click="removeHouseType(row)">删除</ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElDialog>

    <ElDialog v-model="importResultVisible" title="导入结果" width="560px" align-center>
      <ElAlert v-if="importResult" :type="importResult.failed ? 'warning' : 'success'" :closable="false" show-icon
        :title="`共 ${importResult.total} 行，成功 ${importResult.success} 行，失败 ${importResult.failed} 行`" />
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
    fetchNodeTree,
    fetchNodeOptions,
    fetchHousePage,
    fetchCreateHouse,
    fetchUpdateHouse,
    fetchDeleteHouse,
    fetchBatchCreateHouses,
    fetchGetHouseTypes,
    fetchCreateHouseType,
    fetchDeleteHouseType
  } from '@/api/property'
  import { downloadFile, uploadFile } from '@/utils/web-basic'

  defineOptions({ name: 'PropertyHouse' })

  const { currentId } = usePropertyCommunity()
  const nodeTree = ref<any[]>([])
  const nodeOptions = ref<Property.Option[]>([])
  const currentNode = ref<Property.StructureNodeItem | null>(null)
  const currentNodeName = computed(() => currentNode.value?.fullName || '')
  const houseTypes = ref<Property.HouseType[]>([])
  const dialogVisible = ref(false)
  const batchVisible = ref(false)
  const typeVisible = ref(false)
  const importResultVisible = ref(false)
  const importResult = ref<{ total: number; success: number; failed: number; errors: string[] }>()
  const importing = ref(false)
  const submitting = ref(false)
  const editing = ref<Property.House | null>(null)
  const fileInputRef = ref<HTMLInputElement>()
  const newTypeName = ref('')

  const form = reactive<Record<string, any>>({
    nodeId: '',
    code: '',
    houseNo: '',
    usageType: 1,
    areaBuild: 0,
    areaUse: 0,
    houseTypeId: '',
    houseStatus: 1,
    orientation: '',
    layout: ''
  })
  const batchForm = reactive<Record<string, any>>({
    nodeId: '',
    count: 10,
    startNo: 1,
    codePrefix: '',
    areaBuild: 90
  })

  const { columns, columnChecks, data, loading, pagination, getData, replaceSearchParams, handleSizeChange, handleCurrentChange, refreshData } = useTable({
    core: {
      apiFn: fetchHousePage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', label: '序号', width: 70 },
        { prop: 'fullName', label: '房屋', minWidth: 170 },
        { prop: 'code', label: '编码', width: 150 },
        { prop: 'nodePath', label: '所属层级', minWidth: 160 },
        { prop: 'floorNo', label: '楼层', width: 80 },
        { prop: 'areaBuild', label: '面积(㎡)', width: 100 },
        { prop: 'houseTypeName', label: '类型', width: 110 },
        {
          prop: 'usageType',
          label: '用途',
          width: 90,
          formatter: (row: Property.House) =>
            ({ 1: '住宅', 2: '商业', 3: '办公', 4: '车位', 5: '其他' } as Record<number, string>)[row.usageType] || '-'
        },
        {
          prop: 'houseStatus',
          label: '状态',
          width: 90,
          formatter: (row: Property.House) => {
            const map: Record<number, { t: string; type: string }> = {
              1: { t: '空置', type: 'info' },
              2: { t: '自住', type: 'success' },
              3: { t: '出租', type: 'warning' },
              4: { t: '装修中', type: 'primary' },
              5: { t: '停用', type: 'danger' }
            }
            const s = map[row.houseStatus] || map[1]
            return h(ElTag, { type: s.type as any, size: 'small' }, () => s.t)
          }
        },
        { prop: 'residentCount', label: '在册住户', width: 100 },
        {
          prop: 'operation',
          label: '操作',
          width: 90,
          fixed: 'right',
          formatter: (row: Property.House) =>
            h(ArtButtonMore, {
              list: [
                { key: 'edit', label: '编辑房屋', icon: 'ri:edit-2-line', auth: 'property:structure:update' },
                { key: 'delete', label: '删除房屋', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: 'property:structure:delete' }
              ],
              onClick: (item: ButtonMoreItem) => handleMore(item, row)
            })
        }
      ]
    }
  })

  const buildTree = (nodes: Property.StructureNodeItem[]) => {
    const map = new Map<string, any>()
    nodes.forEach((n) => map.set(n.id, { ...n, children: [] }))
    const roots: any[] = []
    nodes.forEach((n) => {
      const cur = map.get(n.id)
      if (n.parentId && map.has(n.parentId)) map.get(n.parentId)!.children.push(cur)
      else roots.push(cur)
    })
    return roots
  }

  const loadNodes = async () => {
    if (!currentId.value) return
    const list = (await fetchNodeTree(currentId.value)) || []
    nodeTree.value = buildTree(list)
    nodeOptions.value = (await fetchNodeOptions(currentId.value, false)) || []
    houseTypes.value = (await fetchGetHouseTypes(currentId.value)) || []
  }

  const handleCommunityChange = async () => {
    currentNode.value = null
    await loadNodes()
    replaceSearchParams({ communityId: currentId.value })
    refreshData()
  }

  const handleNodeClick = (node: any) => {
    currentNode.value = node
    replaceSearchParams({ communityId: currentId.value, nodeId: node.id })
    refreshData()
  }
  const clearNode = () => {
    currentNode.value = null
    replaceSearchParams({ communityId: currentId.value, nodeId: undefined })
    refreshData()
  }

  const openDialog = (row?: Property.House) => {
    editing.value = row || null
    Object.assign(form, {
      nodeId: row?.nodeId || currentNode.value?.id || nodeOptions.value[0]?.id || '',
      code: row?.code || '',
      houseNo: row?.houseNo || '',
      usageType: row?.usageType || 1,
      areaBuild: row?.areaBuild || 0,
      areaUse: row?.areaUse || 0,
      houseTypeId: row?.houseTypeId || '',
      houseStatus: row?.houseStatus || 1,
      orientation: row?.orientation || '',
      layout: row?.layout || ''
    })
    dialogVisible.value = true
  }

  const submit = async () => {
    if (!form.nodeId || !form.code || !form.houseNo) return ElMessage.warning('请选择所属层级并填写编码与房号')
    submitting.value = true
    try {
      const payload = { ...form, communityId: currentId.value }
      if (editing.value) await fetchUpdateHouse(editing.value.id, payload)
      else await fetchCreateHouse(payload)
      dialogVisible.value = false
      refreshData()
      loadNodes()
    } finally {
      submitting.value = false
    }
  }

  const submitBatch = async () => {
    if (!batchForm.nodeId) return ElMessage.warning('请选择所属层级')
    submitting.value = true
    try {
      await fetchBatchCreateHouses({ ...batchForm, communityId: currentId.value })
      batchVisible.value = false
      refreshData()
      loadNodes()
    } finally {
      submitting.value = false
    }
  }

  const handleMore = async (item: ButtonMoreItem, row: Property.House) => {
    if (item.key === 'edit') return openDialog(row)
    try {
      await ElMessageBox.confirm(`确定删除房屋「${row.fullName}」吗？有在册住户或未结账单时不允许删除。`, '删除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchDeleteHouse(row.id)
    refreshData()
    loadNodes()
  }

  const addHouseType = async () => {
    if (!newTypeName.value) return
    await fetchCreateHouseType({ communityId: currentId.value, name: newTypeName.value, usageType: 1 })
    newTypeName.value = ''
    houseTypes.value = (await fetchGetHouseTypes(currentId.value)) || []
  }
  const removeHouseType = async (row: Property.HouseType) => {
    await fetchDeleteHouseType(row.id)
    houseTypes.value = (await fetchGetHouseTypes(currentId.value)) || []
  }

  const triggerImport = () => fileInputRef.value?.click()
  const downloadTemplate = () => downloadFile('/api/v1/property/structures/import-template', {}, '房屋导入模板.xlsx')
  const handleImport = async (e: Event) => {
    const input = e.target as HTMLInputElement
    const file = input.files?.[0]
    input.value = ''
    if (!file || !currentId.value) return
    importing.value = true
    try {
      const res = await uploadFile<{ total: number; success: number; failed: number; errors: string[] }>(
        `/api/v1/property/structures/import?communityId=${currentId.value}`,
        file
      )
      importResult.value = res
      importResultVisible.value = true
      refreshData()
      loadNodes()
    } finally {
      importing.value = false
    }
  }
</script>
