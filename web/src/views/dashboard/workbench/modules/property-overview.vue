<!-- 工作台 · 物业概览：小区关键指标 + 近半年收费趋势 + 门禁授权状态（原独立菜单已并入工作台） -->
<template>
  <ElCard v-if="visible" class="mb-3" shadow="never">
    <template #header>
      <div class="flex flex-wrap items-center justify-between gap-2">
        <span class="font-medium">物业概览</span>
        <div class="flex items-center gap-2">
          <ElSelect v-model="communityId" filterable placeholder="选择小区" class="!w-52" size="small" @change="loadData">
            <ElOption v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </ElSelect>
          <ElButton size="small" :loading="loading" @click="loadData()">刷新</ElButton>
        </div>
      </div>
    </template>

    <div class="mb-3 grid grid-cols-2 gap-3 md:grid-cols-4">
      <div v-for="card in cards" :key="card.label" class="rounded-lg bg-[var(--el-fill-color-lighter)] p-3">
        <div class="text-xs text-g-500">{{ card.label }}</div>
        <div class="mt-1 text-2xl font-semibold">{{ card.value }}</div>
      </div>
    </div>

    <ElRow :gutter="12">
      <ElCol :span="14">
        <div class="mb-2 text-sm font-medium">近半年收费趋势（应收 / 实收）</div>
        <ElTable :data="data?.feeTrend || []" size="small" max-height="220">
          <ElTableColumn prop="billMonth" label="账期" width="100" />
          <ElTableColumn prop="count" label="账单数" width="90" />
          <ElTableColumn prop="receivable" label="应收合计" />
          <ElTableColumn prop="received" label="实收合计" />
          <ElTableColumn label="收缴率" width="100">
            <template #default="{ row }">
              {{ row.receivable ? ((row.received / row.receivable) * 100).toFixed(1) + '%' : '-' }}
            </template>
          </ElTableColumn>
        </ElTable>
        <ElEmpty v-if="!data?.feeTrend?.length" description="暂无账单数据" :image-size="60" />
      </ElCol>
      <ElCol :span="10">
        <div class="mb-2 text-sm font-medium">门禁授权状态</div>
        <div class="mb-1 flex items-center justify-between text-sm">
          <span>成功</span><span class="font-semibold text-green-600">{{ data?.community?.accessSuccess ?? 0 }}</span>
        </div>
        <ElProgress :percentage="accessPercent('success')" :stroke-width="10" status="success" :show-text="false" class="mb-2" />
        <div class="mb-1 flex items-center justify-between text-sm">
          <span>待下发</span><span class="font-semibold text-amber-500">{{ data?.community?.accessPending ?? 0 }}</span>
        </div>
        <ElProgress :percentage="accessPercent('pending')" :stroke-width="10" status="warning" :show-text="false" class="mb-2" />
        <div class="mb-1 flex items-center justify-between text-sm">
          <span>失败（可重试）</span><span class="font-semibold text-red-500">{{ data?.community?.accessFailed ?? 0 }}</span>
        </div>
        <ElProgress :percentage="accessPercent('failed')" :stroke-width="10" status="exception" :show-text="false" />
        <div class="mt-3 flex gap-2">
          <ElButton size="small" @click="$router.push('/property/devices/access')">门禁授权</ElButton>
          <ElButton size="small" @click="$router.push('/property/fee/bill')">账单管理</ElButton>
        </div>
      </ElCol>
    </ElRow>
  </ElCard>
</template>

<script setup lang="ts">
  import { fetchGetPropertyDashboard } from '@/api/property'
  import { usePropertyCommunity } from '@/utils/property-community'

  const { options: communities, loading: communityLoading, loadCommunities } = usePropertyCommunity()
  const data = ref<Property.Dashboard | null>(null)
  const loading = ref(false)
  const communityId = ref('')
  const visible = ref(true)

  const cards = computed(() => [
    { label: '房屋总数', value: data.value?.community?.houseTotal ?? 0 },
    { label: '在册住户', value: data.value?.community?.residentTotal ?? 0 },
    { label: '本月应收', value: `¥${data.value?.community?.receivable ?? 0}` },
    { label: '本月欠费', value: `¥${data.value?.community?.outstanding ?? 0}` }
  ])

  const accessPercent = (key: 'success' | 'pending' | 'failed') => {
    const c = data.value?.community
    if (!c) return 0
    const total = (c.accessSuccess || 0) + (c.accessPending || 0) + (c.accessFailed || 0)
    if (!total) return 0
    const v = key === 'success' ? c.accessSuccess : key === 'pending' ? c.accessPending : c.accessFailed
    return Math.round(((v || 0) / total) * 100)
  }

  const loadData = async (id?: unknown) => {
    loading.value = true
    try {
      const cid = typeof id === 'string' && id ? id : communityId.value || undefined
      data.value = await fetchGetPropertyDashboard(cid)
      // 没有任何授权小区（例如纯后台管理员）时隐藏整块，保持工作台干净
      visible.value = !!data.value?.community || (data.value?.communityCount ?? 0) > 0
    } catch {
      visible.value = false
    } finally {
      loading.value = false
    }
  }

  onMounted(async () => {
    await loadCommunities().catch(() => undefined)
    communityId.value = communities.value[0]?.id || ''
    await loadData()
  })
</script>
