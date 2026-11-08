<!-- 层级管理：自定义层级树（小区为根，节点可任意添加，只需挂到上一级） -->
<template>
  <div class="art-full-height flex gap-3">
    <ElCard class="!w-80 shrink-0">
      <div class="mb-2"><CommunitySelector @change="handleCommunityChange" /></div>
      <ElSpace wrap class="mb-2">
        <ElButton type="primary" size="small" :disabled="!currentId" @click="openDialog(null, 0)">新增一级层级</ElButton>
        <ElButton size="small" :disabled="!selected" @click="openDialog(null, selected?.id)">新增子层级</ElButton>
      </ElSpace>
      <ElInput v-model="keyword" size="small" placeholder="搜索层级名称" clearable class="mb-2" />
      <ElScrollbar height="calc(100vh - 330px)">
        <ElTree
          ref="treeRef"
          :data="tree"
          node-key="id"
          :props="{ label: 'name', children: 'children' }"
          :filter-node-method="filterNode"
          highlight-current
          default-expand-all
          @node-click="handleSelect"
        >
          <template #default="{ data }">
            <span class="flex items-center gap-1">
              <span>{{ data.name }}</span>
              <ElTag size="small" type="info">{{ data.houseCount }}</ElTag>
            </span>
          </template>
        </ElTree>
        <ElEmpty v-if="!tree.length" description="还没有层级，先新增一级层级" :image-size="70" />
      </ElScrollbar>
    </ElCard>

    <ElCard class="art-table-card flex-1">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="loadTree">
        <template #left>
          <ElSpace>
            <ElButton type="primary" :disabled="!currentId" @click="openDialog(null, 0)">新增层级</ElButton>
            <ElButton :disabled="!selected" @click="openDialog(selected, selected?.parentId)">编辑选中</ElButton>
            <ElButton type="danger" plain :disabled="!selected" @click="removeNode(selected)">删除选中</ElButton>
          </ElSpace>
        </template>
      </ArtTableHeader>
      <div class="mb-2 px-4 text-sm text-g-500">
        当前层级：{{ selected?.fullName || '未选择（右侧展示全部层级）' }}
      </div>
      <ElTable :data="tableRows" v-loading="loading" size="small" max-height="calc(100vh - 320px)">
        <ElTableColumn prop="fullName" label="层级全称" min-width="220" />
        <ElTableColumn prop="levelName" label="类型" width="90" />
        <ElTableColumn prop="levelNo" label="层级序号" width="90" />
        <ElTableColumn prop="code" label="编码" width="150" />
        <ElTableColumn prop="houseCount" label="房屋数" width="90" />
        <ElTableColumn prop="childCount" label="子层级" width="90" />
        <ElTableColumn prop="parentName" label="上级" min-width="140" />
        <ElTableColumn label="操作" width="170" fixed="right">
          <template #default="{ row }">
            <ElButton link type="primary" @click="openDialog(null, row.id)">加子级</ElButton>
            <ElButton link type="primary" @click="openDialog(row, row.parentId)">编辑</ElButton>
            <ElButton link type="danger" @click="removeNode(row)">删除</ElButton>
          </template>
        </ElTableColumn>
      </ElTable>

      <ElDialog v-model="dialogVisible" :title="editing ? '编辑层级' : '新增层级'" width="560px" align-center destroy-on-close>
        <ElForm :model="form" label-width="100px">
          <ElFormItem label="层级名称" required>
            <ElInput v-model.trim="form.name" placeholder="如 1栋 / 1单元 / 1层 / 商业区" maxlength="64" />
          </ElFormItem>
          <ElFormItem label="上级层级">
            <ElSelect v-model="form.parentId" clearable filterable placeholder="不选=直接挂在小区下" class="w-full">
              <ElOption v-for="n in nodeOptions" :key="n.id" :label="n.name" :value="n.id" />
            </ElSelect>
          </ElFormItem>
          <ElFormItem label="层级类型">
            <ElSelect v-model="form.nodeType" class="w-full">
              <ElOption label="区域" :value="1" />
              <ElOption label="楼栋" :value="2" />
              <ElOption label="单元" :value="3" />
              <ElOption label="楼层" :value="4" />
              <ElOption label="自定义" :value="5" />
            </ElSelect>
          </ElFormItem>
          <ElFormItem label="编码">
            <ElInput v-model.trim="form.code" placeholder="留空自动生成，小区内唯一" maxlength="64" />
          </ElFormItem>
          <ElFormItem label="楼层号">
            <ElInputNumber v-model="form.floorNo" :min="-10" :max="200" class="!w-full" controls-position="right" />
          </ElFormItem>
          <ElFormItem label="排序">
            <ElInputNumber v-model="form.sort" :min="0" :max="9999" class="!w-full" controls-position="right" />
          </ElFormItem>
        </ElForm>
        <template #footer>
          <ElButton @click="dialogVisible = false">取消</ElButton>
          <ElButton type="primary" :loading="submitting" @click="submit">提交</ElButton>
        </template>
      </ElDialog>
    </ElCard>
  </div>
</template>

<script setup lang="ts">
  import { ElMessage, ElMessageBox, ElTag } from 'element-plus'
  import CommunitySelector from '../components/community-selector.vue'
  import { usePropertyCommunity } from '@/utils/property-community'
  import { fetchNodeTree, fetchNodeOptions, fetchCreateNode, fetchUpdateNode, fetchDeleteNode } from '@/api/property'

  defineOptions({ name: 'PropertyHierarchy' })

  const { currentId } = usePropertyCommunity()
  const treeRef = ref()
  const tree = ref<any[]>([])
  const flatNodes = ref<Property.StructureNodeItem[]>([])
  const keyword = ref('')
  const loading = ref(false)
  const submitting = ref(false)
  const dialogVisible = ref(false)
  const editing = ref<Property.StructureNodeItem | null>(null)
  const selected = ref<Property.StructureNodeItem | null>(null)
  const nodeOptions = ref<Property.Option[]>([])
  const form = reactive<Record<string, any>>({
    name: '',
    parentId: '',
    nodeType: 5,
    code: '',
    floorNo: 0,
    sort: 0
  })

  const columnChecks = ref<any[]>([])

  const tableRows = computed(() => {
    if (!selected.value) return flatNodes.value
    const root = selected.value
    return flatNodes.value.filter((n) => n.id === root.id || n.path.includes(`,${root.id},`) || n.path.endsWith(`,${root.id}`))
  })

  watch(keyword, (v) => treeRef.value?.filter(v))
  const filterNode = (value: string, data: any) => !value || data.name?.includes(value)

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

  const loadTree = async () => {
    if (!currentId.value) {
      tree.value = []
      flatNodes.value = []
      return
    }
    loading.value = true
    try {
      const list = (await fetchNodeTree(currentId.value)) || []
      flatNodes.value = list
      tree.value = buildTree(list)
      nodeOptions.value = (await fetchNodeOptions(currentId.value, false)) || []
      if (selected.value) {
        selected.value = flatNodes.value.find((n) => n.id === selected.value!.id) || null
      }
    } finally {
      loading.value = false
    }
  }

  const handleCommunityChange = async () => {
    selected.value = null
    await loadTree()
  }

  const handleSelect = (node: any) => {
    selected.value = flatNodes.value.find((n) => n.id === node.id) || null
  }

  const openDialog = (row: Property.StructureNodeItem | null, parentId?: string | number) => {
    editing.value = row
    Object.assign(form, {
      name: row?.name || '',
      parentId: parentId !== undefined && parentId !== null && parentId !== 0 ? String(parentId) : row?.parentId || '',
      nodeType: row?.nodeType || 5,
      code: row?.code || '',
      floorNo: row?.floorNo || 0,
      sort: row?.sort || 0
    })
    dialogVisible.value = true
  }

  const submit = async () => {
    if (!form.name) return ElMessage.warning('请填写层级名称')
    submitting.value = true
    try {
      const payload = { ...form, communityId: currentId.value, parentId: form.parentId || 0 }
      if (editing.value) await fetchUpdateNode(editing.value.id, payload)
      else await fetchCreateNode(payload)
      dialogVisible.value = false
      await loadTree()
    } finally {
      submitting.value = false
    }
  }

  const removeNode = async (row: Property.StructureNodeItem | null) => {
    if (!row) return
    try {
      await ElMessageBox.confirm(`确定删除层级「${row.fullName || row.name}」吗？有子层级或房屋时不允许删除。`, '删除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchDeleteNode(row.id)
    if (selected.value?.id === row.id) selected.value = null
    await loadTree()
  }
</script>
