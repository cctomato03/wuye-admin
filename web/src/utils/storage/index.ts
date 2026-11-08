/**
 * 存储工具统一出口
 *
 * @module utils/storage
 */
import { STORAGE_VERSION, StorageConfig, storageKeyManager } from './storage-key-manager'

export { STORAGE_VERSION, StorageConfig, storageKeyManager }
export { StorageKeyManager, getVersionedItem, setVersionedItem } from './storage-key-manager'

/**
 * 本地存储兼容性检查（App 启动时调用）
 *
 * 做三件事：
 *  1. 探测 localStorage 是否可用（隐私模式/被禁用时不能直接崩）；
 *  2. 首次运行写入当前版本号；
 *  3. 版本变化时迁移上一版本的 pinia 持久化数据。
 */
export function checkStorageCompatibility(): boolean {
  if (typeof window === 'undefined' || typeof localStorage === 'undefined') return false
  try {
    const probe = '__storage_probe__'
    localStorage.setItem(probe, '1')
    localStorage.removeItem(probe)
    const stored = localStorage.getItem(StorageConfig.VERSION_KEY)
    if (stored !== String(STORAGE_VERSION)) {
      storageKeyManager.migrate()
      localStorage.setItem(StorageConfig.VERSION_KEY, String(STORAGE_VERSION))
    }
    return true
  } catch (err) {
    console.warn('[storage] localStorage 不可用，部分设置将无法保存', err)
    return false
  }
}

/** 读取 JSON（失败返回 null，不抛异常） */
export function getJSON<T>(key: string): T | null {
  try {
    const raw = localStorage.getItem(key)
    return raw ? (JSON.parse(raw) as T) : null
  } catch {
    return null
  }
}

/** 写入 JSON */
export function setJSON(key: string, value: unknown): void {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    /* 忽略配额错误 */
  }
}

/** 删除键 */
export function removeItem(key: string): void {
  try {
    localStorage.removeItem(key)
  } catch {
    /* ignore */
  }
}
