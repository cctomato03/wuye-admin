import { ref } from 'vue'
import { fetchGetUserInfo, fetchCompanyPage } from '@/api/property'

/** 组织与权限的"当前物业公司"（平台默认租户可切换，其它公司锁定自身） */
export const ORG_TENANT_KEY = 'org.currentTenantId'

export function currentOrgTenantId(): string {
  return localStorage.getItem(ORG_TENANT_KEY) || ''
}

export function setOrgTenantId(id: string) {
  if (id) localStorage.setItem(ORG_TENANT_KEY, id)
  else localStorage.removeItem(ORG_TENANT_KEY)
}

const tenantId = ref(currentOrgTenantId())
const options = ref<Array<{ id: string; name: string }>>([])
const isPlatform = ref(false)
let loaded = false

export function useOrgTenant() {
  const load = async () => {
    if (loaded) return
    loaded = true
    try {
      const info: any = await fetchGetUserInfo()
      const user = info?.user || info?.data?.user || info
      const tid = String(user?.tenantId || user?.tenantID || '1')
      // 平台默认租户（tenant_id=1）可切换任意物业公司
      isPlatform.value = tid === '1'
      if (!isPlatform.value) {
        tenantId.value = tid
        setOrgTenantId(tid)
        return
      }
      const res = await fetchCompanyPage({ current: 1, size: 200 })
      options.value = ((res.records || []) as any[]).map((c) => ({ id: String(c.tenantId), name: c.name }))
      const exists = options.value.some((o) => o.id === tenantId.value)
      if (!exists) tenantId.value = options.value[0]?.id || '1'
      setOrgTenantId(tenantId.value)
    } catch {
      /* 忽略：非平台用户不需要下拉 */
    }
  }
  const setTenant = (id: string) => {
    tenantId.value = id
    setOrgTenantId(id)
  }
  return { tenantId, options, isPlatform, load, setTenant }
}
