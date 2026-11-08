/**
 * 物业模块「当前小区」上下文
 *
 * 物业侧所有业务数据都挂在小区上，页面顶部统一用 CommunitySelector 选择小区，
 * 这里负责：拉取被授权小区、记住上次选择、在各页面间共享同一份状态。
 *
 * 注意：这里只是「界面上的当前小区」，真正的越权防护在后端（pm_user_community + scope）。
 *
 * @module utils/property-community
 */
import { ref, computed } from 'vue'
import { fetchGetCommunityOptions } from '@/api/property'

const STORAGE_KEY = 'property.currentCommunityId'

const options = ref<Property.Option[]>([])
const currentId = ref<string>(localStorage.getItem(STORAGE_KEY) || '')
const loading = ref(false)
let loaded = false

/** 加载可选小区（只加载一次） */
export async function loadCommunities(force = false) {
  if (loaded && !force) return options.value
  loading.value = true
  try {
    const list = (await fetchGetCommunityOptions()) || []
    options.value = list
    // 上次选择已不在授权范围（被撤权/删除）时回落到第一个
    if (!list.some((c) => c.id === currentId.value)) {
      const def = list.find((c) => c.extra === 'default')
      currentId.value = def?.id || list[0]?.id || ''
      if (currentId.value) localStorage.setItem(STORAGE_KEY, currentId.value)
    }
    loaded = true
    return list
  } finally {
    loading.value = false
  }
}

/** 切换当前小区 */
export function setCommunity(id: string) {
  currentId.value = id
  if (id) localStorage.setItem(STORAGE_KEY, id)
}

/** 当前小区名称 */
export const currentCommunity = computed(() => options.value.find((c) => c.id === currentId.value))

/** 组合式：页面里一行拿到状态与刷新能力 */
export function usePropertyCommunity() {
  return { options, currentId, currentCommunity, loading, loadCommunities, setCommunity }
}
