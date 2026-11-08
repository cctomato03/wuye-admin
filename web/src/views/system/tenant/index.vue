<!-- 物业公司与小区管理（平台侧）：物业公司=租户，小区归属唯一物业公司；仅平台超级管理员可维护 -->
<template>
  <div class="art-full-height">
    <ElAlert
      class="mb-3"
      type="info"
      :closable="false"
      title="物业公司即租户：新增公司会自动创建公司管理员角色与管理员账号；小区必须归属一个物业公司，且只能归属一个。"
    />

    <ElTabs v-model="tab" class="tenant-tabs">
      <ElTabPane label="物业公司" name="company">
        <ElCard shadow="never">
          <div class="mb-3 flex flex-wrap items-center gap-2">
            <ElInput v-model="companyQuery.keyword" placeholder="公司名称 / 信用代码" clearable class="!w-56" @keyup.enter="loadCompanies" />
            <ElButton type="primary" @click="loadCompanies">查询</ElButton>
            <ElButton type="success" @click="openCompanyDialog()">新增物业公司</ElButton>
          </div>

          <ElTable :data="companies" v-loading="companyLoading" size="small" max-height="560">
            <ElTableColumn type="index" label="序号" width="70" />
            <ElTableColumn prop="name" label="物业公司" min-width="180" />
            <ElTableColumn prop="tenantCode" label="编码" width="120" />
            <ElTableColumn prop="level" label="资质" width="80" />
            <ElTableColumn prop="contactName" label="联系人" width="100" />
            <ElTableColumn prop="contactPhone" label="联系电话" width="130" />
            <ElTableColumn prop="communityCount" label="管理小区" width="95" />
            <ElTableColumn prop="userCount" label="账号数" width="85" />
            <ElTableColumn label="状态" width="90">
              <template #default="{ row }">
                <ElTag :type="row.status === 1 ? 'success' : row.status === 2 ? 'warning' : 'info'" size="small">
                  {{ ({ 1: '合作中', 2: '暂停', 3: '终止' } as Record<number, string>)[row.status] }}
                </ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="操作" width="200" fixed="right">
              <template #default="{ row }">
                <ElButton link type="primary" @click="openCompanyDialog(row)">编辑</ElButton>
                <ElButton link type="primary" @click="viewCommunities(row)">查看小区</ElButton>
                <ElButton link type="danger" @click="removeCompany(row)">删除</ElButton>
              </template>
            </ElTableColumn>
          </ElTable>
          <div class="mt-2 flex justify-end">
            <ElPagination
              v-model:current-page="companyPage"
              v-model:page-size="companySize"
              :total="companyTotal"
              layout="total, sizes, prev, pager, next"
              :page-sizes="[10, 20, 50]"
              @current-change="loadCompanies"
              @size-change="loadCompanies"
            />
          </div>
        </ElCard>
      </ElTabPane>

      <ElTabPane label="小区" name="community">
        <ElCard shadow="never">
          <div class="mb-3 flex flex-wrap items-center gap-2">
            <ElSelect v-model="communityQuery.tenantId" clearable filterable placeholder="所属物业公司" class="!w-56" @change="loadCommunities">
              <ElOption v-for="c in companies" :key="c.tenantId" :label="c.name" :value="c.tenantId" />
            </ElSelect>
            <ElInput v-model="communityQuery.keyword" placeholder="小区名称 / 编码" clearable class="!w-48" @keyup.enter="loadCommunities" />
            <ElSelect v-model="communityQuery.status" clearable placeholder="状态" class="!w-28">
              <ElOption label="正常" :value="1" />
              <ElOption label="停用" :value="0" />
            </ElSelect>
            <ElButton type="primary" @click="loadCommunities">查询</ElButton>
            <ElButton type="success" @click="openCommunityDialog()">新增小区</ElButton>
          </div>

          <ElTable :data="communities" v-loading="communityLoading" size="small" max-height="560">
            <ElTableColumn type="index" label="序号" width="70" />
            <ElTableColumn prop="name" label="小区名称" min-width="160" />
            <ElTableColumn prop="code" label="编码" width="120" />
            <ElTableColumn prop="tenantName" label="物业公司" min-width="150" />
            <ElTableColumn label="行政区" min-width="220">
              <template #default="{ row }">
                {{ [row.province, row.city, row.district, row.town, row.village].filter(Boolean).join('/') || '-' }}
              </template>
            </ElTableColumn>
            <ElTableColumn prop="buildingCount" label="楼栋" width="80" />
            <ElTableColumn prop="houseCount" label="房屋" width="80" />
            <ElTableColumn label="状态" width="90">
              <template #default="{ row }">
                <ElTag :type="row.status === 1 ? 'success' : 'info'" size="small">{{ row.status === 1 ? '正常' : '停用' }}</ElTag>
              </template>
            </ElTableColumn>
            <ElTableColumn label="操作" width="200" fixed="right">
              <template #default="{ row }">
                <ElButton link type="primary" @click="openCommunityDialog(row)">编辑</ElButton>
                <ElButton link type="primary" @click="toggleCommunity(row)">{{ row.status === 1 ? '停用' : '启用' }}</ElButton>
                <ElButton link type="danger" @click="removeCommunity(row)">删除</ElButton>
              </template>
            </ElTableColumn>
          </ElTable>
          <div class="mt-2 flex justify-end">
            <ElPagination
              v-model:current-page="communityPage"
              v-model:page-size="communitySize"
              :total="communityTotal"
              layout="total, sizes, prev, pager, next"
              :page-sizes="[10, 20, 50]"
              @current-change="loadCommunities"
              @size-change="loadCommunities"
            />
          </div>
        </ElCard>
      </ElTabPane>
    </ElTabs>

    <!-- 物业公司弹窗 -->
    <ElDialog v-model="companyDialog" :title="editingCompany ? '编辑物业公司' : '新增物业公司'" width="760px" align-center destroy-on-close>
      <ElForm ref="companyFormRef" :model="companyForm" :rules="companyRules" label-width="120px">
        <ElRow :gutter="12">
          <ElCol :span="12">
            <ElFormItem label="公司编码" prop="tenantCode">
              <ElInput v-model.trim="companyForm.tenantCode" placeholder="小写字母开头，如 wanke" :disabled="!!editingCompany" maxlength="64" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="公司名称" prop="name"><ElInput v-model.trim="companyForm.name" maxlength="100" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="统一社会信用代码"><ElInput v-model.trim="companyForm.creditCode" maxlength="32" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="资质等级"><ElInput v-model.trim="companyForm.level" placeholder="如 一级" maxlength="32" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="法人"><ElInput v-model.trim="companyForm.legalPerson" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="联系人"><ElInput v-model.trim="companyForm.contactName" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="联系电话"><ElInput v-model.trim="companyForm.contactPhone" maxlength="20" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="用户数上限">
              <ElInputNumber v-model="companyForm.maxUsers" :min="0" class="!w-full" controls-position="right" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="合作状态">
              <ElSelect v-model="companyForm.status" class="w-full">
                <ElOption label="合作中" :value="1" />
                <ElOption label="暂停" :value="2" />
                <ElOption label="终止" :value="3" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <template v-if="!editingCompany">
            <ElCol :span="12">
              <ElFormItem label="管理员账号">
                <ElInput v-model.trim="companyForm.adminUsername" placeholder="留空自动生成 admin_公司编码" maxlength="32" />
              </ElFormItem>
            </ElCol>
            <ElCol :span="12">
              <ElFormItem label="管理员密码">
                <ElInput v-model.trim="companyForm.adminPassword" placeholder="留空随机生成（只显示一次）" maxlength="32" />
              </ElFormItem>
            </ElCol>
            <ElCol :span="24">
              <ElAlert
                type="info"
                :closable="false"
                title="创建后自动生成「物业公司管理员」角色：可管理本公司小区，并可在本公司内继续创建角色、用户、部门与岗位。"
              />
            </ElCol>
          </template>
        </ElRow>
      </ElForm>
      <template #footer>
        <ElButton @click="companyDialog = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitCompany">提交</ElButton>
      </template>
    </ElDialog>

    <!-- 公司创建结果 -->
    <ElDialog v-model="resultDialog" title="物业公司创建成功" width="560px" align-center>
      <ElAlert type="success" :closable="false" show-icon :title="`管理员账号：${created?.adminUsername}`" />
      <div class="mt-3 rounded bg-gray-50 p-3 text-sm">
        <p>初始密码：<span class="font-mono font-bold">{{ created?.adminPassword }}</span></p>
        <p class="mt-1 text-g-500">请立即记录并转交客户，密码仅此一次可见。</p>
      </div>
      <template #footer>
        <ElButton type="primary" @click="resultDialog = false">我已记录</ElButton>
      </template>
    </ElDialog>

    <!-- 小区弹窗 -->
    <ElDialog v-model="communityDialog" :title="editingCommunity ? '编辑小区' : '新增小区'" width="720px" align-center destroy-on-close>
      <ElForm ref="communityFormRef" :model="communityForm" :rules="communityRules" label-width="110px">
        <ElRow :gutter="12">
          <ElCol :span="12">
            <ElFormItem label="所属物业公司" prop="tenantId">
              <ElSelect v-model="communityForm.tenantId" filterable placeholder="请选择物业公司" class="w-full" :disabled="!!editingCommunity">
                <ElOption v-for="c in companies" :key="c.tenantId" :label="c.name" :value="c.tenantId" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="小区编码" prop="code"><ElInput v-model.trim="communityForm.code" placeholder="如 XQ001" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="小区名称" prop="name"><ElInput v-model.trim="communityForm.name" maxlength="100" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="小区类型">
              <ElSelect v-model="communityForm.communityType" class="w-full">
                <ElOption label="住宅" :value="1" />
                <ElOption label="商业综合体" :value="2" />
                <ElOption label="公寓" :value="3" />
                <ElOption label="园区" :value="4" />
                <ElOption label="其他" :value="5" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="所属行政区">
              <ElCascader v-model="districtCodes" :props="districtProps" placeholder="省 / 市 / 区县 / 街道 / 社区" class="w-full" clearable filterable @change="handleDistrictChange" />
              <div v-if="districtLabel" class="mt-1 text-xs text-g-500">{{ districtLabel }}</div>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="详细地址"><ElInput v-model.trim="communityForm.address" maxlength="255" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="占地面积(㎡)">
              <ElInputNumber v-model="communityForm.totalArea" :min="0" :precision="2" class="!w-full" controls-position="right" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="交付日期">
              <ElDatePicker v-model="communityForm.handoverDate" type="date" value-format="YYYY-MM-DD" class="!w-full" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="物业负责人"><ElInput v-model.trim="communityForm.managerName" maxlength="64" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="负责人电话"><ElInput v-model.trim="communityForm.managerPhone" maxlength="20" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="客服电话"><ElInput v-model.trim="communityForm.servicePhone" maxlength="20" /></ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="状态"><ElSwitch v-model="communityForm.status" :active-value="1" :inactive-value="0" /></ElFormItem>
          </ElCol>
        </ElRow>
      </ElForm>
      <template #footer>
        <ElButton @click="communityDialog = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="submitCommunity">提交</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import { ElMessage, ElMessageBox, ElTag, type FormInstance, type FormRules } from 'element-plus'
  import {
    fetchCompanyPage,
    fetchCreateCompany,
    fetchUpdateCompany,
    fetchDeleteCompany,
    fetchCommunityPage,
    fetchCreateCommunity,
    fetchUpdateCommunity,
    fetchDeleteCommunity,
    fetchChangeCommunityStatus,
    fetchDistrictChildren,
    fetchDistrictPath
  } from '@/api/property'

  defineOptions({ name: 'Tenant' })

  const tab = ref<'company' | 'community'>('company')
  const submitting = ref(false)

  /* ------------------------------ 物业公司 ------------------------------ */
  const companies = ref<Property.Company[]>([])
  const companyLoading = ref(false)
  const companyQuery = reactive<{ keyword?: string }>({})
  const companyPage = ref(1)
  const companySize = ref(20)
  const companyTotal = ref(0)
  const companyDialog = ref(false)
  const editingCompany = ref<Property.Company | null>(null)
  const companyFormRef = ref<FormInstance>()
  const resultDialog = ref(false)
  const created = ref<{ adminUsername: string; adminPassword: string } | null>(null)
  const companyForm = reactive<Record<string, any>>({
    tenantCode: '',
    name: '',
    creditCode: '',
    level: '',
    legalPerson: '',
    contactName: '',
    contactPhone: '',
    maxUsers: 0,
    status: 1,
    adminUsername: '',
    adminPassword: ''
  })
  const companyRules: FormRules = {
    tenantCode: [
      { required: true, message: '请输入公司编码', trigger: 'blur' },
      { pattern: /^[a-z][\w-]*$/, message: '小写字母开头，可含数字、下划线、横线', trigger: 'blur' }
    ],
    name: [{ required: true, message: '请输入公司名称', trigger: 'blur' }]
  }

  const loadCompanies = async () => {
    companyLoading.value = true
    try {
      const res = await fetchCompanyPage({ current: companyPage.value, size: companySize.value, keyword: companyQuery.keyword || undefined })
      companies.value = res.records as any
      companyTotal.value = res.total
    } finally {
      companyLoading.value = false
    }
  }

  const openCompanyDialog = (row?: Property.Company) => {
    editingCompany.value = row || null
    companyFormRef.value?.clearValidate()
    Object.assign(companyForm, {
      tenantCode: row?.tenantCode || '',
      name: row?.name || '',
      creditCode: row?.creditCode || '',
      level: row?.level || '',
      legalPerson: row?.legalPerson || '',
      contactName: row?.contactName || '',
      contactPhone: row?.contactPhone || '',
      maxUsers: row?.userCount || 0,
      status: row?.status || 1,
      adminUsername: '',
      adminPassword: ''
    })
    companyDialog.value = true
  }

  const submitCompany = async () => {
    const valid = await companyFormRef.value?.validate().catch(() => false)
    if (!valid) return
    submitting.value = true
    try {
      if (editingCompany.value) {
        await fetchUpdateCompany(editingCompany.value.id, { ...companyForm })
        companyDialog.value = false
        loadCompanies()
      } else {
        const res = await fetchCreateCompany({ ...companyForm })
        companyDialog.value = false
        created.value = { adminUsername: res.adminUsername, adminPassword: res.adminPassword }
        resultDialog.value = true
        loadCompanies()
      }
    } finally {
      submitting.value = false
    }
  }

  const removeCompany = async (row: Property.Company) => {
    try {
      await ElMessageBox.confirm(`确定删除物业公司「${row.name}」吗？该公司下仍有小区时不允许删除。`, '删除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchDeleteCompany(row.id)
    loadCompanies()
  }

  const viewCommunities = (row: Property.Company) => {
    tab.value = 'community'
    communityQuery.tenantId = row.tenantId
    communityPage.value = 1
    loadCommunities()
  }

  /* ------------------------------ 小区 ------------------------------ */
  const communities = ref<Property.Community[]>([])
  const communityLoading = ref(false)
  const communityQuery = reactive<{ keyword?: string; tenantId?: string; status?: number }>({})
  const communityPage = ref(1)
  const communitySize = ref(20)
  const communityTotal = ref(0)
  const communityDialog = ref(false)
  const editingCommunity = ref<Property.Community | null>(null)
  const communityFormRef = ref<FormInstance>()
  const districtCodes = ref<string[]>([])
  const districtLabel = ref('')
  const districtCache = new Map<string, Property.DistrictOption[]>()
  // id → 节点元数据（ElCascader 的 change 只给 id 数组，需要自己映射回编码/名称）
  const districtMeta = new Map<string, { code: string; level: number; name: string }>()
  const districtProps = {
    lazy: true,
    value: 'id',
    label: 'name',
    leaf: (node: any) => node.level >= 5,
    lazyLoad: async (node: any, resolve: (nodes: any[]) => void) => {
      // 省级：node.level 为 0 且没有 value
      const key = node.value || 'root'
      if (districtCache.has(key)) return resolve(districtCache.get(key)!)
      try {
        const list = await fetchDistrictChildren({ parentId: node.value ? String(node.value) : undefined })
        ;(list || []).forEach((d) => districtMeta.set(d.id, { code: d.code, level: d.level, name: d.name }))
        const nodes = (list || []).map((d) => ({
          id: d.id,
          value: d.id,
          name: d.name,
          label: d.name,
          code: d.code,
          level: d.level,
          hasChildren: d.hasChildren,
          leaf: d.level >= 5 || !d.hasChildren
        }))
        districtCache.set(key, nodes as any)
        resolve(nodes)
      } catch {
        resolve([])
      }
    }
  }
  const communityForm = reactive<Record<string, any>>({
    tenantId: '',
    code: '',
    name: '',
    communityType: 1,
    province: '',
    city: '',
    district: '',
    town: '',
    village: '',
    districtCode: '',
    streetCode: '',
    villageCode: '',
    address: '',
    totalArea: 0,
    handoverDate: '',
    managerName: '',
    managerPhone: '',
    servicePhone: '',
    status: 1
  })
  const communityRules: FormRules = {
    tenantId: [{ required: true, message: '请选择所属物业公司', trigger: 'change' }],
    code: [{ required: true, message: '请输入小区编码', trigger: 'blur' }],
    name: [{ required: true, message: '请输入小区名称', trigger: 'blur' }]
  }

  const loadCommunities = async () => {
    communityLoading.value = true
    try {
      const res = await fetchCommunityPage({
        current: communityPage.value,
        size: communitySize.value,
        keyword: communityQuery.keyword || undefined,
        tenantId: communityQuery.tenantId || undefined,
        status: communityQuery.status
      })
      communities.value = res.records as any
      communityTotal.value = res.total
    } finally {
      communityLoading.value = false
    }
  }

  const openCommunityDialog = (row?: Property.Community) => {
    editingCommunity.value = row || null
    communityFormRef.value?.clearValidate()
    Object.assign(communityForm, {
      tenantId: row?.tenantId || communityQuery.tenantId || companies.value[0]?.tenantId || '',
      code: row?.code || '',
      name: row?.name || '',
      communityType: row?.communityType || 1,
      province: row?.province || '',
      city: row?.city || '',
      district: row?.district || '',
      town: row?.town || '',
      village: row?.village || '',
      districtCode: row?.districtCode || '',
      streetCode: row?.streetCode || '',
      villageCode: row?.villageCode || '',
      address: row?.address || '',
      totalArea: row?.totalArea || 0,
      handoverDate: row?.handoverDate?.slice(0, 10) || '',
      managerName: row?.managerName || '',
      managerPhone: row?.managerPhone || '',
      servicePhone: row?.servicePhone || '',
      status: row?.status ?? 1
    })
    districtCodes.value = []
    communityForm.districtCode = row?.districtCode || ''
    communityForm.streetCode = row?.streetCode || ''
    communityForm.villageCode = row?.villageCode || ''
    districtLabel.value = [row?.province, row?.city, row?.district, row?.town, row?.village].filter(Boolean).join(' / ')
    const finest = row?.villageCode || row?.streetCode || row?.districtCode || ''
    if (finest) {
      fetchDistrictPath(finest)
        .then((path) => {
          if (!path?.length) return
          districtCodes.value = path.map((p) => p.id)
          districtLabel.value = path.map((p) => p.name).join(' / ')
        })
        .catch(() => undefined)
    }
    communityDialog.value = true
  }

  const handleDistrictChange = (val: any) => {
    if (!Array.isArray(val) || !val.length) {
      districtLabel.value = ''
      communityForm.province = ''
      communityForm.city = ''
      communityForm.district = ''
      communityForm.town = ''
      communityForm.village = ''
      communityForm.districtCode = ''
      communityForm.streetCode = ''
      communityForm.villageCode = ''
      return
    }
    const metaOf = (idv: string) => districtMeta.get(String(idv))
    const ids = (val as string[]).map(String)
    const byLevel: Record<number, { code: string; name: string }> = {}
    ids.forEach((idv) => {
      const meta = metaOf(idv)
      if (meta) byLevel[meta.level] = { code: meta.code, name: meta.name }
    })
    communityForm.districtCode = byLevel[3]?.code || ''
    communityForm.streetCode = byLevel[4]?.code || ''
    communityForm.villageCode = byLevel[5]?.code || ''
    communityForm.province = byLevel[1]?.name || ''
    communityForm.city = byLevel[2]?.name || ''
    communityForm.district = byLevel[3]?.name || ''
    communityForm.town = byLevel[4]?.name || ''
    communityForm.village = byLevel[5]?.name || ''
    districtLabel.value = [communityForm.province, communityForm.city, communityForm.district, communityForm.town, communityForm.village]
      .filter(Boolean)
      .join(' / ')
    // 兜底：meta 缺失（例如直接回填）时按最末级编码还原
    const finest = communityForm.villageCode || communityForm.streetCode || communityForm.districtCode
    if (finest && (!communityForm.province || !communityForm.district)) {
      fetchDistrictPath(finest)
        .then((path) => {
          if (!path?.length) return
          const map: Record<number, string> = {}
          path.forEach((p) => (map[p.level] = p.name))
          communityForm.province = map[1] || communityForm.province
          communityForm.city = map[2] || communityForm.city
          communityForm.district = map[3] || communityForm.district
          communityForm.town = map[4] || communityForm.town
          communityForm.village = map[5] || communityForm.village
          districtLabel.value = [communityForm.province, communityForm.city, communityForm.district, communityForm.town, communityForm.village]
            .filter(Boolean)
            .join(' / ')
        })
        .catch(() => undefined)
    }
  }

  const submitCommunity = async () => {
    const valid = await communityFormRef.value?.validate().catch(() => false)
    if (!valid) return
    submitting.value = true
    try {
      if (editingCommunity.value) await fetchUpdateCommunity(editingCommunity.value.id, { ...communityForm })
      else await fetchCreateCommunity({ ...communityForm })
      communityDialog.value = false
      loadCommunities()
      loadCompanies()
    } finally {
      submitting.value = false
    }
  }

  const toggleCommunity = async (row: Property.Community) => {
    await fetchChangeCommunityStatus(row.id, row.status === 1 ? 0 : 1)
    loadCommunities()
  }

  const removeCommunity = async (row: Property.Community) => {
    try {
      await ElMessageBox.confirm(`确定删除小区「${row.name}」吗？有房屋/账单数据时不允许删除。`, '删除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchDeleteCommunity(row.id)
    loadCommunities()
  }

  onMounted(async () => {
    await loadCompanies()
    await loadCommunities()
  })
</script>

<style scoped>
  .tenant-tabs :deep(.el-tabs__content) {
    overflow: visible;
  }
</style>
