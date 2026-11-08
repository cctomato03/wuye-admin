/**
 * 版本化存储键管理
 *
 * Pinia 持久化插件的 key 统一由这里生成：`sys-v{version}-{storeId}`。
 * 好处：升级版本后可以直接换一批键，避免旧结构（字段增删）反序列化报错；
 * 同时把上一版本里同名的键迁移过来，用户不会「升级后所有设置被重置」。
 *
 * @module utils/storage/storage-key-manager
 */
import { StorageConfig } from './storage-config'

/** 当前存储结构版本：store 结构发生不兼容变化时 +1 */
export const STORAGE_VERSION = 1

export class StorageKeyManager {
  private readonly prefix: string
  private readonly previousPrefix: string

  constructor(version: number = STORAGE_VERSION) {
    this.prefix = `sys-v${version}-`
    this.previousPrefix = version > 1 ? `sys-v${version - 1}-` : 'sys-'
  }

  /** 生成某个 store 的持久化键 */
  getStorageKey(storeId: string): string {
    return `${this.prefix}${storeId}`
  }

  /** 反解 storeId（调试用） */
  getStoreId(key: string): string | null {
    return key.startsWith(this.prefix) ? key.slice(this.prefix.length) : null
  }

  /**
   * 迁移旧版本的 store 数据：当前键不存在时，用上一版本的键补齐。
   * 返回迁移的键数量。
   */
  migrate(): number {
    if (typeof localStorage === 'undefined') return 0
    let migrated = 0
    const keys: string[] = []
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i)
      if (k) keys.push(k)
    }
    keys.forEach((k) => {
      // 旧版本形如 sys-v0-user / sys-user
      const match = k.match(/^sys-v(\d+)-(.+)$/)
      const rawStoreId = match ? match[2] : k.startsWith('sys-') && !k.startsWith('sys-v') ? k.slice(4) : null
      if (!rawStoreId) return
      const target = this.getStorageKey(rawStoreId)
      if (target === k) return
      if (localStorage.getItem(target) !== null) return
      const value = localStorage.getItem(k)
      if (value === null) return
      try {
        localStorage.setItem(target, value)
        migrated++
      } catch {
        /* 存储空间不足时忽略 */
      }
    })
    return migrated
  }
}

/** 默认实例（store 初始化时复用同一个） */
export const storageKeyManager = new StorageKeyManager()

/** 直接读取一个版本化键的字符串值 */
export function getVersionedItem(storeId: string): string | null {
  if (typeof localStorage === 'undefined') return null
  return localStorage.getItem(storageKeyManager.getStorageKey(storeId))
}

/** 写入一个版本化键 */
export function setVersionedItem(storeId: string, value: string): void {
  if (typeof localStorage === 'undefined') return
  localStorage.setItem(storageKeyManager.getStorageKey(storeId), value)
}

export { StorageConfig }
