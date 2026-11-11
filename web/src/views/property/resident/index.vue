<!-- 住户管理：登记住户（可跨小区按证件回显）、房屋关系（业主/家人/租户含有效期）、续期/还原、人脸、历史 -->
<template>
  <div class="art-full-height">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      <CommunitySelector @change="handleCommunityChange" />
      <ElInput v-model="query.keyword" placeholder="姓名/手机号/证件号" clearable class="!w-52" @keyup.enter="handleSearch" />
      <ElSelect v-model="query.relRole" clearable placeholder="角色" class="!w-28">
        <ElOption label="业主" :value="1" />
        <ElOption label="家人" :value="2" />
        <ElOption label="租户" :value="3" />
      </ElSelect>
      <ElSelect v-model="query.relStatus" clearable placeholder="状态" class="!w-28">
        <ElOption label="有效" :value="1" />
        <ElOption label="已到期" :value="2" />
        <ElOption label="已解除" :value="3" />
      </ElSelect>
      <ElButton type="primary" @click="handleSearch">查询</ElButton>
      <ElButton @click="handleReset">重置</ElButton>
      <ElCheckbox v-model="onlyExpiring" @change="handleSearch">仅看 30 天内到期</ElCheckbox>
    </div>

    <ElCard class="art-table-card">
      <ArtTableHeader v-model:columns="columnChecks" :loading="loading" @refresh="refreshData">
        <template #left>
          <ElButton v-auth="'property:resident:create'" type="primary" @click="openDialog()" v-ripple>登记住户</ElButton>
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

    <!-- 登记/编辑住户 -->
    <ElDialog v-model="dialogVisible" :title="editing ? '编辑住户' : '登记住户'" width="900px" align-center destroy-on-close>
      <ElForm ref="formRef" :model="form" :rules="rules" label-width="110px">
        <ElAlert
          v-if="lookupTip"
          class="mb-3"
          :type="lookupTip.type"
          :closable="false"
          show-icon
          :title="lookupTip.text"
        />
        <ElRow :gutter="12">
          <ElCol :span="8">
            <ElFormItem label="证件类型">
              <ElSelect v-model="form.idType" class="w-full">
                <ElOption label="身份证" :value="1" />
                <ElOption label="护照" :value="2" />
                <ElOption label="港澳通行证" :value="3" />
                <ElOption label="台胞证" :value="4" />
                <ElOption label="其他" :value="9" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="16">
            <ElFormItem label="证件号码" prop="idNo">
              <ElInput v-model.trim="form.idNo" placeholder="输入后自动查询是否已登记" maxlength="64" @blur="handleLookup" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="8">
            <ElFormItem label="姓名" prop="name">
              <ElInput v-model.trim="form.name" maxlength="64" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="8">
            <ElFormItem label="性别">
              <ElSelect v-model="form.gender" class="w-full">
                <ElOption label="未知" :value="0" />
                <ElOption label="男" :value="1" />
                <ElOption label="女" :value="2" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="8">
            <ElFormItem label="手机号">
              <ElInput v-model.trim="form.phone" maxlength="20" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="8">
            <ElFormItem label="出生日期">
              <ElDatePicker v-model="form.birthday" type="date" value-format="YYYY-MM-DD" class="!w-full" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="8">
            <ElFormItem label="民族">
              <ElInput v-model.trim="form.nation" maxlength="32" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="8">
            <ElFormItem label="紧急联系人">
              <ElInput v-model.trim="form.emergencyContact" maxlength="64" />
            </ElFormItem>
          </ElCol>
        </ElRow>

        <ElDivider content-position="left">房屋关系</ElDivider>
        <ElRow :gutter="12">
          <ElCol :span="24">
            <ElFormItem label="房屋">
              <HouseCascader v-model="form.houseId" :disabled="!!editing" placeholder="按层级选择房屋" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="身份">
              <ElSelect v-model="form.relRole" class="w-full" style="min-width: 180px" :disabled="!!editing">
                <ElOption label="业主" :value="1" />
                <ElOption label="家人" :value="2" />
                <ElOption label="租户" :value="3" />
              </ElSelect>
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="与业主关系">
              <ElInput v-model.trim="form.relation" placeholder="本人/配偶…" maxlength="32" style="min-width: 180px" :disabled="!!editing" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem label="开始日期">
              <ElDatePicker v-model="form.startDate" type="date" value-format="YYYY-MM-DD" class="!w-full" :disabled="!!editing" />
            </ElFormItem>
          </ElCol>
          <ElCol :span="12">
            <ElFormItem :label="form.relRole === 3 ? '到期日期（必填）' : '到期日期'">
              <ElDatePicker v-model="form.endDate" type="date" value-format="YYYY-MM-DD" class="!w-full" :disabled="!!editing" />
            </ElFormItem>
          </ElCol>
        </ElRow>
      </ElForm>
      <template #footer>
        <ElButton @click="dialogVisible = false">取消</ElButton>
        <ElButton type="primary" :loading="submitting" @click="handleSubmit">提交</ElButton>
      </template>
    </ElDialog>

    <!-- 详情抽屉 -->
    <ElDrawer v-model="detailVisible" title="住户详情" size="680px">
      <div v-if="detail">
        <ElDescriptions :column="3" border size="small">
          <ElDescriptionsItem label="姓名">{{ detail.name }}</ElDescriptionsItem>
          <ElDescriptionsItem label="证件号">{{ detail.idNo }}</ElDescriptionsItem>
          <ElDescriptionsItem label="手机号">{{ detail.phone || '-' }}</ElDescriptionsItem>
          <ElDescriptionsItem label="性别">{{ detail.gender === 1 ? '男' : detail.gender === 2 ? '女' : '未知' }}</ElDescriptionsItem>
          <ElDescriptionsItem label="其他小区登记">{{ detail.otherCommunityCount || 0 }} 处</ElDescriptionsItem>
          <ElDescriptionsItem label="人脸照片">{{ detail.faces?.length || 0 }} 张</ElDescriptionsItem>
        </ElDescriptions>

        <ElDivider content-position="left">房屋关系</ElDivider>
        <ElTable :data="detail.houses || []" size="small">
          <ElTableColumn prop="communityName" label="小区" width="120" />
          <ElTableColumn prop="houseName" label="房屋" min-width="140" />
          <ElTableColumn label="身份" width="80">
            <template #default="{ row }">{{ ['', '业主', '家人', '租户'][row.relRole] }}</template>
          </ElTableColumn>
          <ElTableColumn prop="endDate" label="到期日" width="120">
            <template #default="{ row }">{{ row.endDate?.slice(0, 10) || '长期' }}</template>
          </ElTableColumn>
          <ElTableColumn label="状态" width="90">
            <template #default="{ row }">
              <ElTag :type="row.status === 1 ? 'success' : 'info'" size="small">{{ ['', '有效', '已到期', '已解除'][row.status] }}</ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn label="操作" width="180">
            <template #default="{ row }">
              <ElButton v-if="row.status !== 1" size="small" link type="primary" @click="handleRenew(row)">续期/还原</ElButton>
              <ElButton v-if="row.status === 1" size="small" link type="danger" @click="handleUnbind(row)">解除</ElButton>
            </template>
          </ElTableColumn>
        </ElTable>

        <ElDivider content-position="left">人脸照片</ElDivider>
        <div class="flex flex-wrap gap-2">
          <div v-for="f in detail.faces || []" :key="f.id" class="relative">
            <img :src="f.fileUrl" class="h-24 w-20 rounded object-cover" />
            <ElButton class="!absolute right-0 top-0" size="small" type="danger" circle @click="removeFace(f.id)">×</ElButton>
          </div>
          <ElButton v-auth="'property:resident:face'" @click="addFace">+ 人脸</ElButton>
        </div>

        <ElDivider content-position="left">关系变更历史</ElDivider>
        <ElTimeline>
          <ElTimelineItem v-for="log in history" :key="log.id" :timestamp="log.operateTime?.slice(0, 19).replace('T', ' ')">
            {{ actionText(log.action) }} · {{ log.operatorName || '系统' }}
            <span v-if="log.afterEnd" class="text-g-500">（到期 {{ log.afterEnd.slice(0, 10) }}）</span>
          </ElTimelineItem>
        </ElTimeline>
      </div>
    </ElDrawer>
  </div>
</template>

<script setup lang="ts">
  import HouseCascader from '../components/house-cascader.vue'
  import { ElMessage, ElMessageBox, ElTag, type FormInstance, type FormRules } from 'element-plus'
  import CommunitySelector from '../components/community-selector.vue'
  import ArtButtonMore from '@/components/core/forms/art-button-more/index.vue'
  import type { ButtonMoreItem } from '@/components/core/forms/art-button-more/index.vue'
  import { useTable } from '@/hooks/core/useTable'
  import { uploadFile } from '@/utils/web-basic'
  import { usePropertyCommunity } from '@/utils/property-community'
  import {
    fetchResidentPage,
    fetchGetResident,
    fetchCreateResident,
    fetchUpdateResident,
    fetchDeleteResident,
    fetchLookupResident,
    fetchGetResidentHistory,
    fetchBindResidentHouse,
    fetchUnbindResidentHouse,
    fetchRenewResidentHouse,
    fetchRestoreResidentHouse,
    fetchAddResidentFace,
    fetchDeleteResidentFace
  } from '@/api/property'

  defineOptions({ name: 'PropertyResident' })

  const { currentId } = usePropertyCommunity()
  const query = reactive<{ keyword?: string; relRole?: number; relStatus?: number }>({})
  const onlyExpiring = ref(false)

  const dialogVisible = ref(false)
  const detailVisible = ref(false)
  const editing = ref<Property.Resident | null>(null)
  const detail = ref<Property.Resident | null>(null)
  const history = ref<Property.ResidentLog[]>([])
  const submitting = ref(false)
  const formRef = ref<FormInstance>()
  const lookupTip = ref<{ type: 'success' | 'info'; text: string } | null>(null)
  const houseLoading = ref(false)

  const form = reactive<Record<string, any>>({
    idType: 1,
    idNo: '',
    name: '',
    gender: 0,
    phone: '',
    birthday: '',
    nation: '',
    emergencyContact: '',
    houseId: '',
    relRole: 1,
    relation: '',
    startDate: '',
    endDate: ''
  })

  const rules: FormRules = {
    idNo: [{ required: true, message: '请输入证件号码', trigger: 'blur' }],
    name: [{ required: true, message: '请输入姓名', trigger: 'blur' }]
  }

  const { columns, columnChecks, data, loading, pagination, getData, replaceSearchParams, handleSizeChange, handleCurrentChange, refreshData } = useTable({
    core: {
      apiFn: fetchResidentPage,
      apiParams: { current: 1, size: 20 },
      columnsFactory: () => [
        { type: 'index', label: '序号', width: 70 },
        { prop: 'name', label: '姓名', width: 110 },
        { prop: 'idNo', label: '证件号', width: 170 },
        { prop: 'phone', label: '手机号', width: 130 },
        { prop: 'houseName', label: '房屋', minWidth: 150 },
        {
          prop: 'relRole',
          label: '身份',
          width: 90,
          formatter: (row: Property.ResidentRow) =>
            h(ElTag, { size: 'small', type: row.relRole === 1 ? 'success' : row.relRole === 3 ? 'warning' : 'primary' }, () =>
              ({ 1: '业主', 2: '家人', 3: '租户' } as Record<number, string>)[row.relRole]
            )
        },
        { prop: 'relation', label: '关系', width: 90 },
        {
          prop: 'endDate',
          label: '到期日',
          width: 130,
          formatter: (row: Property.ResidentRow) => (row.endDate ? h('span', { class: expiring(row.endDate) ? 'text-red-500' : '' }, row.endDate.slice(0, 10)) : '长期')
        },
        {
          prop: 'relStatus',
          label: '状态',
          width: 90,
          formatter: (row: Property.ResidentRow) => {
            const map: Record<number, { t: string; type: 'success' | 'warning' | 'danger' | 'info' | 'primary' }> = {
              1: { t: '有效', type: 'success' },
              2: { t: '已到期', type: 'warning' },
              3: { t: '已解除', type: 'info' }
            }
            const s = map[row.relStatus] || map[1]
            return h(ElTag, { size: 'small', type: s.type }, () => s.t)
          }
        },
        {
          prop: 'operation',
          label: '操作',
          width: 90,
          fixed: 'right',
          formatter: (row: Property.ResidentRow) =>
            h(ArtButtonMore, {
              list: [
                { key: 'detail', label: '查看详情', icon: 'ri:eye-line' },
                { key: 'edit', label: '编辑资料', icon: 'ri:edit-2-line', auth: 'property:resident:update' },
                { key: 'renew', label: '续期', icon: 'ri:time-line', auth: 'property:resident:bind' },
                { key: 'unbind', label: '解除关系', icon: 'ri:close-circle-line', color: '#e6a23c', auth: 'property:resident:bind' },
                { key: 'delete', label: '删除住户', icon: 'ri:delete-bin-4-line', color: '#f56c6c', auth: 'property:resident:delete' }
              ],
              onClick: (item: ButtonMoreItem) => handleMore(item, row)
            })
        }
      ]
    }
  })

  const expiring = (date: string) => new Date(date).getTime() - Date.now() < 30 * 86400000

  const handleCommunityChange = (id: string) => {
    replaceSearchParams({ communityId: id })
    refreshData()
  }
  const handleSearch = () => {
    replaceSearchParams({
      keyword: query.keyword || undefined,
      relRole: query.relRole,
      relStatus: query.relStatus,
      expiring: onlyExpiring.value || undefined
    })
    getData()
  }
  const handleReset = () => {
    query.keyword = ''
    query.relRole = undefined
    query.relStatus = undefined
    onlyExpiring.value = false
    handleSearch()
  }



  const openDialog = () => {
    editing.value = null
    lookupTip.value = null
    formRef.value?.clearValidate()
    Object.assign(form, {
      idType: 1,
      idNo: '',
      name: '',
      gender: 0,
      phone: '',
      birthday: '',
      nation: '',
      emergencyContact: '',
      houseId: '',
      relRole: 1,
      relation: '',
      startDate: '',
      endDate: ''
    })
    dialogVisible.value = true
  }

  const handleLookup = async () => {
    if (!form.idNo) return
    try {
      const res = await fetchLookupResident({ idType: form.idType, idNo: form.idNo })
      if (res?.found) {
        form.name = res.name || form.name
        form.phone = res.phone || form.phone
        form.gender = res.gender ?? form.gender
        form.birthday = res.birthday || form.birthday
        form.nation = res.nation || form.nation
        lookupTip.value = {
          type: 'success',
          text: `该证件已在本物业公司登记过：${res.name}${res.hasOtherCommunity ? '（还在其他小区登记，资料已回显，保存时按需修改）' : ''}`
        }
      } else {
        lookupTip.value = { type: 'info', text: '新住户，请继续填写资料' }
      }
    } catch {
      lookupTip.value = null
    }
  }

  const handleSubmit = async () => {
    const valid = await formRef.value?.validate().catch(() => false)
    if (!valid) return
    if (form.relRole === 3 && !form.endDate) return ElMessage.warning('租户必须填写到期日期')
    submitting.value = true
    try {
      if (editing.value) {
        await fetchUpdateResident(editing.value.id, { ...form })
      } else {
        await fetchCreateResident({ ...form, communityId: currentId.value })
      }
      dialogVisible.value = false
      refreshData()
    } finally {
      submitting.value = false
    }
  }

  const openDetail = async (residentId: string) => {
    detail.value = await fetchGetResident(residentId)
    history.value = (await fetchGetResidentHistory(residentId)) || []
    detailVisible.value = true
  }

  const handleMore = async (item: ButtonMoreItem, row: Property.ResidentRow) => {
    if (item.key === 'detail') return openDetail(row.residentId)
    if (item.key === 'edit') {
      const res = await fetchGetResident(row.residentId)
      editing.value = res
      Object.assign(form, {
        idType: res.idType,
        idNo: res.idNo,
        name: res.name,
        gender: res.gender,
        phone: res.phone,
        birthday: res.birthday?.slice(0, 10) || '',
        nation: res.nation || '',
        emergencyContact: res.emergencyContact || '',
        houseId: row.houseId,
        relRole: row.relRole,
        relation: row.relation || '',
        startDate: '',
        endDate: ''
      })
      dialogVisible.value = true
      return
    }
    if (item.key === 'renew') return handleRenew(row)
    if (item.key === 'unbind') return handleUnbind(row)
    if (item.key === 'delete') {
      try {
        await ElMessageBox.confirm(`确定删除住户「${row.name}」吗？会解除其在本小区的房屋关系并撤销门禁权限。`, '删除确认', {
          type: 'warning'
        })
      } catch {
        return
      }
      await fetchDeleteResident(row.residentId)
      refreshData()
    }
  }

  const handleRenew = async (row: { relId: string; relStatus?: number; status?: number; endDate?: string }) => {
    const relId = row.relId || (row as any).id
    const status = row.relStatus ?? row.status
    try {
      if (status !== 1 && (!row.endDate || new Date(row.endDate) < new Date())) {
        const { value } = await ElMessageBox.prompt('请输入新的到期日期（YYYY-MM-DD）', '续期', {
          inputValue: new Date(Date.now() + 365 * 86400000).toISOString().slice(0, 10)
        })
        await fetchRenewResidentHouse(relId, { endDate: value })
      } else {
        await fetchRestoreResidentHouse(relId)
      }
      ElMessage.success('操作成功')
      refreshData()
      if (detail.value) openDetail(detail.value.id)
    } catch {
      /* 取消 */
    }
  }

  const handleUnbind = async (row: { relId?: string; id?: string }) => {
    const relId = row.relId || row.id!
    try {
      await ElMessageBox.confirm('确定解除该房屋关系吗？解除后可还原。', '解除确认', { type: 'warning' })
    } catch {
      return
    }
    await fetchUnbindResidentHouse(relId)
    refreshData()
    if (detail.value) openDetail(detail.value.id)
  }

  const addFace = async () => {
    if (!detail.value) return
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = 'image/*'
    input.onchange = async () => {
      const file = input.files?.[0]
      if (!file) return
      // 统一走 uploadFile：本地磁盘中转 / OSS 直传自动选择
      const uploaded = await uploadFile<{ url: string }>('/api/v1/files/upload', file)
      await fetchAddResidentFace(detail.value!.id, {
        communityId: currentId.value,
        fileUrl: uploaded.url,
        isPrimary: !detail.value!.faces?.length
      })
      openDetail(detail.value!.id)
    }
    input.click()
  }

  const removeFace = async (id: string) => {
    await fetchDeleteResidentFace(id)
    if (detail.value) openDetail(detail.value.id)
  }

  const actionText = (action: number) =>
    ({ 1: '登记', 2: '续期', 3: '到期', 4: '解除', 5: '删除', 6: '还原' } as Record<number, string>)[action] || '变更'
</script>
