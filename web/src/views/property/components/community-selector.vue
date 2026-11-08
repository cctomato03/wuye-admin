<!-- 小区选择器：物业模块所有页面顶部共用
     - 选择结果持久化在 localStorage，切换后其它页面自动沿用；
     - 进入页面时若未选择且当前物业公司有小区，自动选中第一个；
     - 当前公司没有配置任何小区时，明确提示"暂无管理小区"，页面不再发起业务请求。
-->
<template>
  <div class="flex flex-wrap items-center gap-2">
    <span class="text-sm text-g-600">当前小区</span>
    <ElSelect
      :model-value="currentId"
      :loading="loading"
      filterable
      clearable
      placeholder="请选择小区"
      class="!w-64"
      @update:model-value="handleChange"
    >
      <ElOption v-for="item in options" :key="item.id" :label="item.name" :value="item.id">
        <span>{{ item.name }}</span>
        <span class="ml-2 text-xs text-g-400">{{ item.code }}</span>
      </ElOption>
    </ElSelect>
    <ElTag v-if="!loading && !options.length" type="warning" size="small">当前物业公司暂无管理小区</ElTag>
    <span v-else-if="options.length > 1" class="text-xs text-g-400">共 {{ options.length }} 个小区，切换后会记住</span>
  </div>
  <ElAlert
    v-if="!loading && !options.length"
    class="mt-2"
    type="warning"
    :closable="false"
    show-icon
    title="没有可选小区：请联系平台管理员在「系统管理 → 物业公司与小区」为本公司创建小区后再操作。"
  />
</template>

<script setup lang="ts">
  import { onMounted } from 'vue'
  import { usePropertyCommunity } from '@/utils/property-community'

  const emit = defineEmits<{ change: [id: string] }>()

  const { options, currentId, loading, loadCommunities, setCommunity } = usePropertyCommunity()

  // 每次进入页面都刷新一次（管理员可能刚授权了新小区），随后按"上次选择 → 第一个"确定当前小区
  onMounted(async () => {
    await loadCommunities(true).catch(() => undefined)
    emit('change', currentId.value)
  })

  const handleChange = (id: string) => {
    setCommunity(id || '')
    emit('change', id || '')
  }
</script>
