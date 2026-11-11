<!-- 公共组件：按「小区层级 → 房屋」级联选择房屋，供单位/车辆/住户/访客等业务使用 -->
<template>
  <ElCascader
    v-model="cascaderValue"
    :options="options"
    :props="{ value: 'value', label: 'label', children: 'children' }"
    :placeholder="placeholder || '按层级选择房屋'"
    :disabled="disabled || !currentId"
    :loading="loading"
    filterable
    clearable
    class="w-full"
    @change="handleChange"
  />
</template>

<script setup lang="ts">
  import { watch } from 'vue'
  import { usePropertyCommunity } from '@/utils/property-community'
  import { fetchHouseCascader } from '@/api/property'

  const props = withDefaults(
    defineProps<{
      modelValue?: string
      usageType?: number
      placeholder?: string
      disabled?: boolean
    }>(),
    { modelValue: '', usageType: 0, placeholder: '', disabled: false }
  )
  const emit = defineEmits<{
    'update:modelValue': [value: string]
    change: [houseId: string]
  }>()

  const { currentId } = usePropertyCommunity()
  const options = ref<Property.CascaderNode[]>([])
  const cascaderValue = ref<string[]>([])
  const loading = ref(false)

  const findPath = (list: Property.CascaderNode[], target: string, prefix: string[] = []): string[] | null => {
    for (const item of list) {
      const path = [...prefix, item.value]
      if (item.value === target) return path
      if (item.children?.length) {
        const found = findPath(item.children, target, path)
        if (found) return found
      }
    }
    return null
  }

  const load = async () => {
    if (!currentId.value) {
      options.value = []
      return
    }
    loading.value = true
    try {
      options.value = (await fetchHouseCascader(currentId.value, props.usageType || undefined)) || []
      syncValue()
    } finally {
      loading.value = false
    }
  }

  const syncValue = () => {
    if (!props.modelValue) {
      cascaderValue.value = []
      return
    }
    const path = findPath(options.value, 'h-' + props.modelValue)
    if (path) cascaderValue.value = path
  }

  watch(() => props.modelValue, syncValue)
  watch(
    () => props.usageType,
    () => load()
  )
  watch(currentId, () => load(), { immediate: true })

  const handleChange = (val: any) => {
    const arr = Array.isArray(val) ? (val as string[]) : []
    const last = arr[arr.length - 1] || ''
    const houseId = last.startsWith('h-') ? last.slice(2) : ''
    emit('update:modelValue', houseId)
    emit('change', houseId)
  }
</script>
