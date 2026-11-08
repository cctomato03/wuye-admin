/**
 * 本地存储配置
 *
 * 集中管理所有直接落在 localStorage / sessionStorage 里的键名，
 * 避免各模块各写一套字符串导致「同一份数据两个键」的问题。
 *
 * @module utils/storage/storage-config
 */
export const StorageConfig = {
  /** 存储键版本前缀：v 后面的数字变化时，旧数据会被自动迁移/丢弃 */
  VERSION_KEY: 'sys-version',

  /** 当前主题（light / dark / system） */
  THEME_KEY: 'sys-theme',

  /** 主题色等外观设置 */
  THEME_COLOR_KEY: 'sys-theme-color',

  /** 上次登录的用户 ID（多账号切换时用于判断是否要清理状态） */
  LAST_USER_ID_KEY: 'sys-last-user-id',

  /** 响应式布局下的当前菜单类型（vertical / horizontal / mixed / dual） */
  RESPONSIVE_MENU_TYPE_KEY: 'sys-responsive-menu-type',

  /** 侧边栏折叠状态 */
  SIDEBAR_COLLAPSED_KEY: 'sys-sidebar-collapsed',

  /** 全局搜索历史 */
  SEARCH_HISTORY_KEY: 'sys-search-history',

  /** 首页快捷入口 */
  FAST_ENTER_KEY: 'sys-fast-enter',

  /** 锁屏状态 */
  LOCK_SCREEN_KEY: 'sys-lock-screen',

  /** 数据表格列设置缓存前缀（后面拼业务表名） */
  TABLE_COLUMNS_PREFIX: 'sys-table-columns-',

  /** 物业模块：当前小区 */
  PROPERTY_COMMUNITY_KEY: 'property.currentCommunityId'
} as const

export type StorageKey = (typeof StorageConfig)[keyof typeof StorageConfig]
