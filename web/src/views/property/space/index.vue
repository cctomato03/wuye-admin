<!-- 小区基础信息管理：顶部小区卡片 + 左侧层级树 + 右侧层级/房屋信息卡片 -->
<template>
  <div class="art-full-height flex flex-col gap-3">
    <!-- 顶部：小区信息卡片（与页面同宽） -->
    <ElCard shadow="never" class="w-full">
      <div class="flex flex-wrap items-center gap-x-8 gap-y-2">
        <div class="flex items-baseline gap-2">
          <span class="text-lg font-semibold">{{ communityCard?.name || '请选择小区' }}</span>
          <span v-if="communityCard?.code" class="text-xs text-g-500">{{ communityCard.code }}</span>
        </div>
        <span class="text-sm text-g-600">地址：{{ communityCard?.address || '-' }}</span>
        <span class="text-sm text-g-600">负责人：{{ communityCard?.managerName || '-' }}</span>
        <span class="text-sm text-g-600">客服：{{ communityCard?.servicePhone || '-' }}</span>
        <div class="ml-auto flex flex-wrap gap-2">
          <ElTag type="primary">楼栋 {{ communityCard?.buildings ?? 0 }}</ElTag>
          <ElTag type="success">房屋 {{ communityCard?.houses ?? 0 }}</ElTag>
          <ElTag type="warning">住户 {{ communityCard?.residents ?? 0 }}</ElTag>
          <ElTag type="info">设备 {{ communityCard?.devices ?? 0 }}</ElTag>
        </div>
      </div>
      <div class="mt-2 flex flex-wrap items-center gap-2">
        <CommunitySelector @change="handleCommunityChange" />
        <ElButton type="primary" :disabled="!currentId" @click="openNodeDialog(null, defaultParentId())">新建</ElButton>
        <ElButton v-auth="'property:structure:import'" :loading="importing" :disabled="!currentId" @click="triggerImport">导入 Excel</ElButton>
        <ElButton text @click="downloadTemplate">下载模板</ElButton>
        <ElButton type="danger" plain :loading="clearing" :disabled="!currentId" @click="clearSpace">清空层级与房屋</ElButton>
        <input ref="fileInputRef" type="file" accept=".xlsx,.xls" class="hidden" @change="handleImport" />
      </div>
    </ElCard>

    <div class="flex min-h-0 flex-1 gap-3">
      <!-- 左：层级树 -->
      <ElCard class="!w-80 shrink-0">
        <ElButton type="primary" class="mb-2 w-full" :disabled="!currentId" @click="openNodeDialog(null, defaultParentId())">新建</ElButton>
        <ElInput v-model="treeKeyword" size="small" placeholder="搜索层级名称 / 地址" clearable class="mb-2" />
        <ElScrollbar height="calc(100vh - 380px)">
          <ElTree
            ref="treeRef"
            :data="treeData"
            node-key="id"
            :props="{ label: 'label', children: 'children', isLeaf: 'isLeaf' }"
            :filter-node-method="filterNode"
            :load="loadTreeChildren"
            lazy
            highlight-current
            @node-click="handleTreeClick"
          >
            <template #default="{ data }">
              <span class="flex items-center gap-1">
                <ElTag v-if="data.rowType !== 'house'" size="small" type="primary">{{ data.typeName }}</ElTag>
                <ElTag v-else size="small" type="success">房屋</ElTag>
                <span>{{ data.label }}</span>
              </span>
            </template>
          </ElTree>
          <ElEmpty v-if="!treeData.length" description="请选择小区" :image-size="70" />
        </ElScrollbar>
      </ElCard>

      <!-- 右：信息卡片 -->
      <ElCard class="flex-1 overflow-auto">
        <!-- 小区 -->
        <template v-if="selected?.rowType === 'root' && communityCard">
          <div class="mb-3 flex items-center justify-between">
            <span class="text-base font-semibold">小区名称：{{ communityCard.name }}</span>
          </div>
          <ElDescriptions :column="2" border size="small">
            <ElDescriptionsItem label="小区范围">
              <ElSpace wrap>
                <ElTag v-for="n in rootChildNames" :key="n" size="small">{{ n }}</ElTag>
                <span v-if="!rootChildNames.length">-</span>
              </ElSpace>
            </ElDescriptionsItem>
            <ElDescriptionsItem label="房屋总数">{{ communityCard.houses }}</ElDescriptionsItem>
            <ElDescriptionsItem label="住户人数">{{ communityCard.residents }}</ElDescriptionsItem>
            <ElDescriptionsItem label="关联设备">{{ communityCard.devices }}</ElDescriptionsItem>
            <ElDescriptionsItem label="小区地址" :span="2">{{ communityCard.address || '-' }}</ElDescriptionsItem>
          </ElDescriptions>
        </template>

        <!-- 层级 -->
        <template v-else-if="selected?.rowType === 'node' && nodeCard">
          <div class="mb-3 flex flex-wrap items-center gap-2">
            <span class="text-base font-semibold">{{ nodeCard.typeName }}名称：{{ nodeCard.name }}</span>
            <ElSpace class="ml-auto">
              <ElButton size="small" @click="openNodeDialog(null, nodeCard.id)">新增下级</ElButton>
              <ElButton size="small" @click="openNodeDialog(selected, nodeCard.parentId || '0')">编辑</ElButton>
              <ElButton size="small" @click="openHouseDialog(null, nodeCard.id)">新增房屋</ElButton>
              <ElButton size="small" type="danger" plain @click="removeNode">删除</ElButton>
            </ElSpace>
          </div>
          <ElDescriptions :column="2" border size="small">
            <ElDescriptionsItem :label="`${nodeCard.typeName}范围`">
              <ElSpace wrap>
                <ElTag v-for="n in nodeCard.childNames" :key="n" size="small">{{ n }}</ElTag>
                <span v-if="!nodeCard.childNames.length">-</span>
              </ElSpace>
            </ElDescriptionsItem>
            <ElDescriptionsItem label="房屋总数">{{ nodeCard.houseTotal }}</ElDescriptionsItem>
            <ElDescriptionsItem label="住户人数">{{ nodeCard.residentCount }}</ElDescriptionsItem>
            <ElDescriptionsItem label="关联设备">
              <ElSpace wrap>
                <ElTag v-for="d in nodeCard.devices" :key="d" size="small" type="info">{{ d }}</ElTag>
                <span v-if="!nodeCard.devices.length">-</span>
              </ElSpace>
            </ElDescriptionsItem>
            <ElDescriptionsItem :label="`${nodeCard.typeName}地址`" :span="2">{{ nodeCard.address || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="上级层级">{{ nodeCard.parentName || '小区' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="层级路径">{{ nodeCard.pathNames?.join(' / ') || '-' }}</ElDescriptionsItem>
          </ElDescriptions>

          <div class="mt-4">
            <div class="mb-2 text-sm font-medium">该{{ nodeCard.typeName }}下的房屋（{{ nodeCard.houseTotal }}）</div>
            <ElTable :data="nodeHouses" size="small" max-height="320">
              <ElTableColumn prop="fullName" label="房屋" min-width="180" />
              <ElTableColumn prop="code" label="编码" width="150" />
              <ElTableColumn prop="houseNo" label="房号" width="100" />
              <ElTableColumn prop="areaBuild" label="建筑(㎡)" width="100" />
              <ElTableColumn label="状态" width="90">
                <template #default="{ row }">{{ statusText(row.houseStatus) }}</template>
              </ElTableColumn>
              <ElTableColumn label="操作" width="80">
                <template #default="{ row }">
                  <ElButton link type="primary" @click="selectHouseRow(row)">查看</ElButton>
                </template>
              </ElTableColumn>
            </ElTable>
          </div>
        </template>

        <!-- 房屋 -->
        <template v-else-if="selected?.rowType === 'house' && houseCard">
          <div class="mb-3 flex flex-wrap items-center gap-2">
            <span class="text-base font-semibold">房屋名称：{{ houseCard.house.fullName }}</span>
            <ElSpace class="ml-auto">
              <ElButton size="small" @click="openHouseDialog(selected)">编辑</ElButton>
              <ElButton size="small" type="danger" plain @click="removeHouse">删除</ElButton>
            </ElSpace>
          </div>
          <ElDescriptions :column="3" border size="small">
            <ElDescriptionsItem label="房屋编码">{{ houseCard.house.code }}</ElDescriptionsItem>
            <ElDescriptionsItem label="房号">{{ houseCard.house.houseNo }}</ElDescriptionsItem>
            <ElDescriptionsItem label="用途">{{ usageText(houseCard.house.usageType) }}</ElDescriptionsItem>
            <ElDescriptionsItem label="建筑面积">{{ houseCard.house.areaBuild }} ㎡</ElDescriptionsItem>
            <ElDescriptionsItem label="套内面积">{{ houseCard.house.areaUse }} ㎡</ElDescriptionsItem>
            <ElDescriptionsItem label="公摊面积">{{ houseCard.house.areaShare }} ㎡</ElDescriptionsItem>
            <ElDescriptionsItem label="类型">{{ houseCard.house.houseTypeName || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="户型/朝向">{{ [houseCard.house.layout, houseCard.house.orientation].filter(Boolean).join(' / ') || '-' }}</ElDescriptionsItem>
            <ElDescriptionsItem label="状态">{{ statusText(houseCard.house.houseStatus) }}</ElDescriptionsItem>
            <ElDescriptionsItem label="房屋地址" :span="3">{{ houseCard.address || houseCard.house.fullName }}</ElDescriptionsItem>
          </ElDescriptions>

          <div class="mt-4">
            <div class="mb-2 text-sm font-medium">住户信息（{{ houseCard.residents.length }}）</div>
            <ElTable :data="houseCard.residents" size="small" max-height="260">
              <ElTableColumn prop="name" label="姓名" width="120" />
              <ElTableColumn prop="roleName" label="身份" width="90" />
              <ElTableColumn prop="relation" label="与业主关系" width="120" />
              <ElTableColumn prop="phone" label="联系电话" width="140" />
              <ElTableColumn prop="startDate" label="入住" width="110" />
              <ElTableColumn label="有效期至" width="110">
                <template #default="{ row }">{{ row.endDate || '长期' }}</template>
              </ElTableColumn>
              <ElTableColumn label="主联系人" width="90">
                <template #default="{ row }">{{ row.isMain ? '是' : '否' }}</template>
              </ElTableColumn>
            </ElTable>
            <ElEmpty v-if="!houseCard.residents.length" description="暂无在册住户" :image-size="60" />
          </div>

          <div class="mt-4">
            <div class="mb-2 text-sm font-medium">车辆信息（{{ houseCard.vehicles.length }}）</div>
            <ElTable :data="houseCard.vehicles" size="small" max-height="220">
              <ElTableColumn prop="plateNo" label="车牌号" width="140" />
              <ElTableColumn prop="vehicleTypeName" label="车型" width="110" />
              <ElTableColumn prop="parkingSpaceNo" label="车位" width="120" />
            </ElTable>
            <ElEmpty v-if="!houseCard.vehicles.length" description="暂无登记车辆" :image-size="60" />
          </div>
        </template>

        <ElEmpty v-else description="点击左侧层级查看信息卡片" :image-size="90" />
      </ElCard>
    </div>

    <!-- 新建/编辑层级：只绑定上级 + 名称 + 类型（类型来自字典） -->
    <ElDialog v-model="nodeDialog" :title="editingNode ? '编辑层级' : '新建层级'" width="520px" align-center destroy-on-close>
      <ElForm :model="nodeForm" label-width="90px">
        <ElFormItem label="上级层级" required>
          <ElTreeSelect
            v-model="nodeForm.parentId"
            :data="parentTree"
            :props="{ label: 'name', value: 'id', children: 'children' }"
            check-strictly
            filterable
            :default-expand-all="false"
            class="w-full"
            placeholder="选择上级层级（默认小区为顶级）"
          />
        </ElFormItem>
        <ElFormItem label="层级名称" required>
          <ElInput v-model.trim="nodeForm.name" placeholder="如 1栋 / 1单元 / 1层 / 一期" maxlength="64" />
        </ElFormItem>
        <ElFormItem label="层级类型" required>
          <ElSelect v-model="nodeForm.nodeType" class="w-full" placeholder="请选择层级类型">
            <ElOption v-for="d in nodeTypes" :key="d.value" :label="d.label" :value="Number(d.value)" />
          </ElSelect>
        </ElFormItem>
        <ElAlert type="info" :closable="false" title="层级地址自动生成：上级地址 + 层级名称（小区下一级 = 小区地址 + 名称）；同一上级下名称不能重复。" />
      </ElForm>
      <template #footer>
        <ElButton @click="nodeDialog = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitNode">提交</ElButton>
      </template>
    </ElDialog>

    <!-- 房屋弹窗：可挂小区或任意层级 -->
    <ElDialog v-model="houseDialog" :title="editingHouse ? '编辑房屋' : '新增房屋'" width="820px" align-center destroy-on-close>
      <ElForm ref="houseFormRef" :model="houseForm" :rules="houseRules" label-width="100px">
        <ElRow :gutter="12">
          <ElCol :span="8">
            <ElFormItem label="所属层级" prop="nodeId">
              <ElTreeSelect
                v-model="houseForm.nodeId"
                :data="nodeTreeOptions"
                :props="{ label: 'name', value: 'id', children: 'children' }"
                check-strictly
                filterable
                clearable
                :default-expand-all="false"
                class="w-full"
                placeholder="选择所属层级（可直接选小区）"
              />
            </ElFormItem>
          </ElCol>
          <ElCol :span="8"><ElFormItem label="房屋编码" prop="code"><ElInput v-model.trim="houseForm.code" maxlength="64" /></ElFormItem></ElCol>
          <ElCol :span="8"><ElFormItem label="房号" prop="houseNo"><ElInput v-model.trim="houseForm.houseNo" placeholder="如 101 / 1号 / 101铺" maxlength="64" /></ElFormItem></ElCol>
          <ElCol :span="8">
            <ElFormItem label="房屋用途">
              <ElSelect v-model="houseForm.usageType" class="w-full">
                <ElOption label="住宅" :value="1" />
                <ElOption label="商业" :value="2" />
                <ElOption label="办公" :value="3" />
                <ElOption label="其他" :value="5" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="8">
            <ElFormItem label="房屋类型">
              <ElSelect v-model="houseForm.houseTypeId" clearable class="w-full">
                <ElOption v-for="t in houseTypes" :key="t.id" :label="t.name" :value="t.id" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="8">
            <ElFormItem label="房屋状态">
              <ElSelect v-model="houseForm.houseStatus" class="w-full">
                <ElOption label="空置" :value="1" />
                <ElOption label="自住" :value="2" />
                <ElOption label="出租" :value="3" />
                <ElOption label="装修中" :value="4" />
                <ElOption label="停用" :value="5" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="8"><ElFormItem label="建筑面积" prop="areaBuild"><ElInputNumber v-model="houseForm.areaBuild" :min="0" :precision="2" class="!w-full" /></ElFormItem></ElCol>
          <ElCol :span="8"><ElFormItem label="套内面积"><ElInputNumber v-model="houseForm.areaUse" :min="0" :precision="2" class="!w-full" /></ElFormItem></ElCol>
          <ElCol :span="8"><ElFormItem label="公摊面积"><ElInputNumber v-model="houseForm.areaShare" :min="0" :precision="2" class="!w-full" /></ElFormItem></ElCol>
          <ElCol :span="8"><ElFormItem label="户型"><ElInput v-model.trim="houseForm.layout" placeholder="3室2厅" maxlength="64" /></ElFormItem></ElCol>
          <ElCol :span="8"><ElFormItem label="朝向"><ElInput v-model.trim="houseForm.orientation" placeholder="南 / 东南" maxlength="20" /></ElFormItem></ElCol>
          <ElCol :span="8">
            <ElFormItem label="装修">
              <ElSelect v-model="houseForm.decoration" class="w-full">
                <ElOption label="未知" :value="0" />
                <ElOption label="毛坯" :value="1" />
                <ElOption label="简装" :value="2" />
                <ElOption label="精装" :value="3" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="8">
            <ElFormItem label="产权性质">
              <ElSelect v-model="houseForm.propertyRight" class="w-full">
                <ElOption label="商品房" :value="1" />
                <ElOption label="安置房" :value="2" />
                <ElOption label="公房" :value="3" />
                <ElOption label="自建" :value="4" />
                <ElOption label="其他" :value="9" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="8"><ElFormItem label="层高(m)"><ElInputNumber v-model="houseForm.ceilingHeight" :min="0" :precision="2" class="!w-full" /></ElFormItem></ElCol>
          <ElCol :span="8"><ElFormItem label="临街商铺"><ElSwitch v-model="houseForm.isStreet" /></ElFormItem></ElCol>
          <ElCol :span="8"><ElFormItem label="门面宽(m)"><ElInputNumber v-model="houseForm.shopWidth" :min="0" :precision="2" class="!w-full" /></ElFormItem></ElCol>
          <ElCol :span="8"><ElFormItem label="是否可售"><ElSwitch v-model="houseForm.isSale" /></ElFormItem></ElCol>
          <ElCol :span="8"><ElFormItem label="交付日期"><ElDatePicker v-model="houseForm.deliveryDate" type="date" value-format="YYYY-MM-DD" class="!w-full" /></ElFormItem></ElCol>
          <ElCol :span="16"><ElFormItem label="附属设施"><ElInput v-model.trim="houseForm.facilities" placeholder="水/电/燃气/空调/消防…" maxlength="500" /></ElFormItem></ElCol>
          <ElCol :span="24"><ElFormItem label="备注"><ElInput v-model="houseForm.remark" type="textarea" :rows="2" maxlength="500" /></ElFormItem></ElCol>
        </ElRow>
      </ElForm>
      <template #footer>
        <ElButton @click="houseDialog = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitHouse">提交</ElButton>
      </template>
    </ElDialog>

    <!-- 导入结果 -->
    <ElDialog v-model="importResultVisible" title="导入结果" width="640px" align-center>
      <ElAlert v-if="importResult" :type="importResult.failed ? 'warning' : 'success'" :closable="false" show-icon
        :title="`共 ${importResult.total} 行，成功 ${importResult.success} 行，失败 ${importResult.failed} 行`" />
      <ElAlert v-if="importResult?.aborted" class="mt-2" type="error" :closable="false" show-icon
        title="存在错误，本次未导入任何数据。请修正下列问题后重新上传。" />
      <ul v-if="importResult?.errors?.length" class="mt-2 max-h-60 overflow-auto text-xs text-red-500">
        <li v-for="(msg, i) in importResult.errors" :key="i">{{ msg }}</li>
      </ul>
      <div class="mt-3 text-xs text-g-500">
        导入规则：模板三列（房屋编码 / 房屋地址 / 房屋面积）。房屋地址需以「小区地址 + 小区名称」开头，
        例如「湖北省武汉市江岸区黄浦大街25号黄浦雅苑1栋4单元1楼1号」；系统会按 区/栋/单元/层(楼)/号 自动补齐层级后挂载房屋。面积留空按 0 处理。
      </div>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import { ElMessage, ElMessageBox, ElTag, type FormInstance, type FormRules } from 'element-plus'
  import CommunitySelector from '../components/community-selector.vue'
  import { usePropertyCommunity } from '@/utils/property-community'
  import { fetchGetDictData } from '@/api/business'
  import {
    fetchNodeTree,
    fetchNodeOptions,
    fetchNodeChildren,
    fetchNodeCard,
    fetchHouseCard,
    fetchCommunityCard,
    fetchCreateNode,
    fetchUpdateNode,
    fetchDeleteNode,
    fetchHousePage,
    fetchCreateHouse,
    fetchClearSpace,
    fetchUpdateHouse,
    fetchDeleteHouse,
    fetchGetHouseTypes,
    fetchCreateHouseType,
    fetchDeleteHouseType
  } from '@/api/property'
  import { downloadFile, uploadFile } from '@/utils/web-basic'

  defineOptions({ name: 'PropertySpace' })

  interface TreeRow {
    id: string
    label: string
    rowType: 'root' | 'node' | 'house'
    typeName: string
    isLeaf?: boolean
    raw?: any
  }

  const { currentId } = usePropertyCommunity()
  const treeRef = ref()
  const treeData = ref<TreeRow[]>([])
  const treeKeyword = ref('')
  const nodeOptions = ref<Property.Option[]>([])
  const allNodes = ref<Property.StructureNodeItem[]>([])

  // 层级树（根为"小区（顶级）"），用于上级层级/房屋所属层级的下拉（默认全部折叠）
  const buildSelectTree = (excludeId?: string) => {
    const excluded = new Set<string>()
    if (excludeId) {
      excluded.add(excludeId)
      let changed = true
      while (changed) {
        changed = false
        allNodes.value.forEach((n) => {
          if (n.parentId && excluded.has(n.parentId) && !excluded.has(n.id)) {
            excluded.add(n.id)
            changed = true
          }
        })
      }
    }
    const list = allNodes.value.filter((n) => !excluded.has(n.id))
    const byParent = new Map<string, any[]>()
    list.forEach((n) => {
      const key = n.parentId && n.parentId !== '0' ? n.parentId : '0'
      const arr = byParent.get(key) || []
      arr.push({ id: n.id, name: n.name, children: [] as any[] })
      byParent.set(key, arr)
    })
    const build = (parent: string): any[] => {
      const arr = byParent.get(parent) || []
      arr.forEach((item) => (item.children = build(item.id)))
      return arr
    }
    return build('0')
  }
  const nodeTreeOptions = computed(() => [{ id: '0', name: '小区（顶级）', children: buildSelectTree() }])
  const parentTree = computed(() => [{ id: '0', name: '小区（顶级）', children: buildSelectTree(editingNode.value?.id) }])
  const communityCard = ref<Property.CommunityCard | null>(null)
  const nodeCard = ref<Property.NodeCard | null>(null)
  const houseCard = ref<Property.HouseCard | null>(null)
  const nodeHouses = ref<Property.House[]>([])
  const selected = ref<TreeRow | null>(null)
  const rootChildNames = ref<string[]>([])
  const nodeTypes = ref<WebBasic.DictData[]>([])
  const houseTypes = ref<Property.HouseType[]>([])
  const loading = ref(false)
  const submitting = ref(false)
  const clearing = ref(false)
  const importing = ref(false)
  const exporting = ref(false)
  const nodeDialog = ref(false)
  const houseDialog = ref(false)
  const importResultVisible = ref(false)
  const importResult = ref<{ total: number; success: number; failed: number; errors: string[]; aborted?: boolean }>()
  const editingNode = ref<TreeRow | null>(null)
  const editingHouse = ref<TreeRow | null>(null)
  const fileInputRef = ref<HTMLInputElement>()
  const houseFormRef = ref<FormInstance>()
  const newTypeName = ref('')

  const nodeForm = reactive<Record<string, any>>({ parentId: '0', name: '', nodeType: undefined })


  const houseForm = reactive<Record<string, any>>({
    nodeId: '0',
    code: '',
    houseNo: '',
    usageType: 1,
    houseTypeId: '',
    houseStatus: 1,
    areaBuild: 0,
    areaUse: 0,
    areaShare: 0,
    layout: '',
    orientation: '',
    decoration: 0,
    propertyRight: 1,
    ceilingHeight: 0,
    isStreet: false,
    shopWidth: 0,
    isSale: true,
    deliveryDate: '',
    facilities: '',
    remark: ''
  })
  const houseRules: FormRules = {
    code: [{ required: true, message: '请填写房屋编码', trigger: 'blur' }],
    houseNo: [{ required: true, message: '请填写房号', trigger: 'blur' }],
    areaBuild: [{ required: true, message: '请填写建筑面积', trigger: 'blur' }]
  }

  const usageText = (u?: number) => ({ 1: '住宅', 2: '商业', 3: '办公', 4: '车位', 5: '其他' } as Record<number, string>)[u || 5] || '其他'
  const statusText = (s?: number) => ({ 1: '空置', 2: '自住', 3: '出租', 4: '装修中', 5: '停用' } as Record<number, string>)[s || 1] || '空置'

  watch(treeKeyword, (v) => treeRef.value?.filter(v))
  const filterNode = (value: string, data: any) => !value || String(data?.label || '').includes(value)

  // 树懒加载：子层级 + 直接挂载的房屋
  const loadTreeChildren = async (node: any, resolve: (rows: TreeRow[]) => void) => {
    const row = (node?.data || node) as TreeRow
    try {
      const list = (await fetchNodeChildren(currentId.value, row.rowType === 'root' ? '0' : row.id)) || []
      const rows: TreeRow[] = []
      const childNodes: string[] = []
      list.forEach((it) => {
        if (it.rowType === 'node' && it.node) {
          childNodes.push(it.node.name)
          rows.push({
            id: it.node.id,
            label: it.node.name,
            rowType: 'node',
            typeName: it.node.typeName || '层级',
            isLeaf: !((it.node.childCount ?? 0) > 0 || (it.node.houseCount ?? 0) > 0),
            raw: it.node
          })
        } else if (it.rowType === 'house' && it.house) {
          // 树里房屋只显示房号，避免整条地址把树撑得很长
          rows.push({ id: it.house.id, label: it.house.houseNo || it.house.code, rowType: 'house', typeName: '房屋', isLeaf: true, raw: it.house })
        }
      })
      if (row.rowType === 'root') rootChildNames.value = childNodes
      resolve(rows)
    } catch {
      resolve([])
    }
  }

  const loadCards = async () => {
    if (!currentId.value) {
      communityCard.value = null
      return
    }
    communityCard.value = await fetchCommunityCard(currentId.value)
    nodeOptions.value = (await fetchNodeOptions(currentId.value, false)) || []
    allNodes.value = (await fetchNodeTree(currentId.value)) || []
    houseTypes.value = (await fetchGetHouseTypes(currentId.value)) || []
  }

  const resetTree = async () => {
    nodeCard.value = null
    houseCard.value = null
    nodeHouses.value = []
    selected.value = null
    treeData.value = communityCard.value
      ? [{ id: '0', label: communityCard.value.name, rowType: 'root', typeName: '小区' }]
      : []
    await nextTick()
    // 展开根节点，直接看到第一层
    const root = treeRef.value?.store?.nodesMap?.['0']
    if (root) root.expand()
  }

  const reload = async () => {
    loading.value = true
    try {
      await loadCards()
      await resetTree()
      if (selected.value?.rowType === 'node') await selectNode(selected.value)
      if (selected.value?.rowType === 'house') await selectHouse(selected.value as any)
    } finally {
      loading.value = false
    }
  }

  const handleCommunityChange = async () => {
    selected.value = null
    await reload()
  }

  const handleTreeClick = async (row: TreeRow) => {
    selected.value = row
    if (row.rowType === 'root') {
      nodeCard.value = null
      houseCard.value = null
      nodeHouses.value = []
      return
    }
    if (row.rowType === 'node') return selectNode(row)
    return selectHouse(row)
  }

  const selectNode = async (row: TreeRow) => {
    nodeCard.value = await fetchNodeCard(row.id)
    houseCard.value = null
    const res = await fetchHousePage({ communityId: currentId.value, nodeId: row.id, current: 1, size: 200 })
    nodeHouses.value = (res.records as any) || []
  }

  const selectHouseRow = (h: Property.House) => {
    const row: TreeRow = { id: h.id, label: h.fullName, rowType: 'house', typeName: '房屋', isLeaf: true, raw: h }
    selected.value = row
    selectHouse(row)
  }

  const selectHouse = async (row: TreeRow) => {
    selected.value = row
    houseCard.value = await fetchHouseCard(row.id)
    nodeCard.value = null
    nodeHouses.value = []
  }

  // 新建时默认上级：若在树中选中了层级/房屋，则用它的节点；否则小区（顶级）
  const defaultParentId = () => {
    if (!selected.value) return '0'
    if (selected.value.rowType === 'node') return selected.value.id
    if (selected.value.rowType === 'house') return selected.value.raw?.nodeId || '0'
    return '0'
  }

  const openNodeDialog = (row: TreeRow | null, parentId: string) => {
    editingNode.value = row
    Object.assign(nodeForm, {
      parentId: parentId || row?.raw?.parentId || '0',
      name: row?.label || row?.raw?.name || '',
      nodeType: row?.raw?.nodeType
    })
    nodeDialog.value = true
  }

  const submitNode = async () => {
    if (!nodeForm.name) return ElMessage.warning('请填写层级名称')
    if (!nodeForm.nodeType) return ElMessage.warning('请选择层级类型')
    submitting.value = true
    try {
      const payload = { communityId: currentId.value, parentId: nodeForm.parentId || 0, name: nodeForm.name, nodeType: nodeForm.nodeType }
      if (editingNode.value) await fetchUpdateNode(editingNode.value.id, payload)
      else await fetchCreateNode(payload)
      nodeDialog.value = false
      await reload()
    } finally {
      submitting.value = false
    }
  }

  const removeNode = async () => {
    if (!nodeCard.value) return
    try {
      await ElMessageBox.confirm(`确定删除「${nodeCard.value.name}」吗？有子层级或房屋时不允许删除。`, '删除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchDeleteNode(nodeCard.value.id)
    selected.value = null
    nodeCard.value = null
    await reload()
  }

  const openHouseDialog = (row: TreeRow | null, nodeId?: string) => {
    editingHouse.value = row
    const h = row?.raw
    Object.assign(houseForm, {
      nodeId: h?.nodeId || nodeId || (selected.value?.rowType === 'node' ? selected.value.id : '0'),
      code: h?.code || '',
      houseNo: h?.houseNo || '',
      usageType: h?.usageType || 1,
      houseTypeId: h?.houseTypeId || '',
      houseStatus: h?.houseStatus || 1,
      areaBuild: h?.areaBuild || 0,
      areaUse: h?.areaUse || 0,
      areaShare: h?.areaShare || 0,
      layout: h?.layout || '',
      orientation: h?.orientation || '',
      decoration: h?.decoration || 0,
      propertyRight: h?.propertyRight || 1,
      ceilingHeight: h?.ceilingHeight || 0,
      isStreet: !!h?.isStreet,
      shopWidth: h?.shopWidth || 0,
      isSale: h?.isSale ?? true,
      deliveryDate: h?.deliveryDate?.slice(0, 10) || '',
      facilities: h?.facilities || '',
      remark: h?.remark || ''
    })
    houseDialog.value = true
  }

  const submitHouse = async () => {
    const valid = await houseFormRef.value?.validate().catch(() => false)
    if (!valid) return
    submitting.value = true
    try {
      const payload = { ...houseForm, communityId: currentId.value, nodeId: houseForm.nodeId || '0' }
      if (editingHouse.value) await fetchUpdateHouse(editingHouse.value.id, payload)
      else await fetchCreateHouse(payload)
      houseDialog.value = false
      await reload()
    } finally {
      submitting.value = false
    }
  }

  const removeHouse = async () => {
    if (!houseCard.value) return
    try {
      await ElMessageBox.confirm(`确定删除房屋「${houseCard.value.house.fullName}」吗？有在册住户或未结账单时不允许删除。`, '删除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchDeleteHouse(houseCard.value.house.id)
    selected.value = null
    houseCard.value = null
    await reload()
  }



  const clearSpace = async () => {
    if (!currentId.value) return
    try {
      await ElMessageBox.confirm(
        '将删除本小区下所有层级与房屋（含导入的数据），此操作不可恢复；如已有住户/账单/车辆/单位/访客数据会被拒绝。确定继续？',
        '清空层级与房屋',
        { type: 'warning', confirmButtonText: '确认清空', cancelButtonText: '取消' }
      )
    } catch {
      return
    }
    clearing.value = true
    try {
      await fetchClearSpace(currentId.value)
      selected.value = null
      nodeCard.value = null
      houseCard.value = null
      nodeHouses.value = []
      await reload()
    } finally {
      clearing.value = false
    }
  }

  const triggerImport = () => fileInputRef.value?.click()
  const downloadTemplate = () =>
    downloadFile('/api/v1/property/structures/import-template', { communityId: currentId.value }, '小区基础信息导入模板.xlsx')
  const handleImport = async (e: Event) => {
    const input = e.target as HTMLInputElement
    const file = input.files?.[0]
    input.value = ''
    if (!file || !currentId.value) return
    importing.value = true
    try {
      const res = await uploadFile<{ total: number; success: number; failed: number; errors: string[]; aborted?: boolean }>(
        `/api/v1/property/structures/import?communityId=${currentId.value}`,
        file
      )
      importResult.value = res
      importResultVisible.value = true
      await reload()
    } finally {
      importing.value = false
    }
  }



  onMounted(async () => {
    nodeTypes.value = (await fetchGetDictData('space_node_type')) || []
  })
</script>
