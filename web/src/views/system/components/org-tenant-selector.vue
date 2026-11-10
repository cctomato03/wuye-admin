<!-- 组织与权限：物业公司切换（平台默认租户可见；物业公司用户锁定本公司） -->
<template>
  <div v-if="isPlatform && options.length" class="flex items-center gap-2">
    <span class="text-sm text-g-600">物业公司</span>
    <ElSelect
      :model-value="tenantId"
      filterable
      class="!w-56"
      placeholder="选择物业公司"
      @update:model-value="handleChange"
    >
      <ElOption v-for="o in options" :key="o.id" :label="o.name" :value="o.id" />
    </ElSelect>
  </div>
</template>

<script setup lang="ts">
  import { onMounted } from 'vue'
  import { useOrgTenant } from '@/utils/org-tenant'

  const emit = defineEmits<{ change: [tenantId: string] }>()
  const { tenantId, options, isPlatform, load, setTenant } = useOrgTenant()

  onMounted(async () => {
    await load()
    emit('change', tenantId.value)
  })

  const handleChange = (id: string) => {
    setTenant(id)
    emit('change', id)
  }
</script>
