/**
 * 文件上传（统一入口）
 *
 * 策略：优先「前端直传 OSS」，不可用时自动回退「服务端中转上传」。
 *
 *   1. 调后端 /files/oss-ticket 拿签名（本地磁盘存储会返回「不支持」）；
 *   2. 拿到签名后浏览器直接 PUT 到 OSS（不占用应用服务器上行带宽）；
 *   3. 上传成功后调 /files/oss-record 把对象键登记进文件表；
 *   4. 1~3 任一步失败（CORS 未配置、签名过期、网络异常、存储非 OSS）→ 回退 /files/upload。
 *
 * OSS 侧需配置 CORS：允许站点域名发起 PUT，允许 Content-Type 请求头。
 *
 * @module utils/upload
 */
import {
  fetchGetOssTicket,
  fetchRecordOssFile,
  fetchUploadFile,
  fetchGetPublicConfig,
  mapFile
} from '@/api/business'

export interface UploadOptions {
  /** 业务类型，决定对象键里的目录（如 avatar / document / site） */
  bizType?: string
  /** 是否静默成功提示 */
  silent?: boolean
}

/**
 * 后端是否支持直传（公开参数 sys.upload.direct，由服务端存储配置决定）。
 * 缓存一次，避免每次上传都多一个请求；本地存储时直接走中转上传，不产生无意义的 400。
 */
let directSupported: boolean | null = null

async function canDirectUpload(): Promise<boolean> {
  if (directSupported !== null) return directSupported
  try {
    const cfg = await fetchGetPublicConfig()
    directSupported = `${cfg?.['sys.upload.direct']}` === 'true'
  } catch {
    directSupported = false
  }
  return directSupported
}

/**
 * 上传一个文件，返回文件记录（url / objectKey 等）。
 *
 * @param file   浏览器 File 对象
 * @param options 业务类型等选项
 */
export async function uploadFile(
  file: File,
  options: UploadOptions = {}
): Promise<WebBasic.FileItem> {
  const bizType = options.bizType || 'default'

  // 存储为 OSS 且开启直传时：只走直传，失败直接报错，绝不回退服务端上传（避免占用应用服务器带宽）
  if (await canDirectUpload()) {
    const ticket = await fetchGetOssTicket({
      originName: file.name,
      size: file.size,
      bizType
    })
    if (!ticket?.uploadUrl) {
      throw new Error('OSS 直传签名失败：后端未返回上传地址，请检查 OSS 配置与权限')
    }

    let uploaded = false
    if (ticket.method === 'POST' && ticket.formFields) {
      // 表单直传：multipart/form-data 属于"简单请求"，不触发 CORS 预检
      const fd = new FormData()
      Object.entries(ticket.formFields).forEach(([k, v]) => fd.append(k, String(v)))
      fd.append('file', file)
      try {
        const res = await fetch(ticket.uploadUrl, { method: 'POST', body: fd })
        uploaded = res.ok || res.type === 'opaque'
      } catch (err) {
        // 响应可能被浏览器 CORS 拦截，但上传请求本身可能已成功：交给回执接口用 HEAD 校验
        console.warn('[upload] 表单直传响应不可读（可能未配置 CORS），改由回执接口校验：', err)
      }
    } else {
      // 预签名 PUT（兼容 upload_mode=put）
      let uploadUrl = ticket.uploadUrl
      if (typeof window !== 'undefined' && window.location.protocol === 'https:' && uploadUrl.startsWith('http://')) {
        uploadUrl = 'https://' + uploadUrl.slice('http://'.length)
        console.warn('[upload] 签名地址为 http，已自动升级为 https 以规避混合内容拦截')
      }
      const res = await fetch(uploadUrl, {
        method: ticket.method || 'PUT',
        headers: ticket.headers || {},
        body: file
      })
      if (!res.ok) {
        const detail = await res.text().catch(() => '')
        throw new Error(
          `OSS 直传失败（HTTP ${res.status}）：请检查 Bucket 的 CORS 是否允许本站点 PUT、以及 AccessKey 是否有 PutObject 权限。${detail.slice(0, 200)}`
        )
      }
      uploaded = true
    }

    try {
      // 无论上传响应是否可读，都以服务端 HEAD 校验为准登记文件
      return await fetchRecordOssFile({
        objectKey: ticket.objectKey,
        originName: file.name,
        size: file.size,
        mime: file.type,
        bizType
      })
    } catch (err) {
      console.error('[upload] 直传回执失败：', err, 'uploaded=' + uploaded)
      throw new Error(
        '文件上传 OSS 失败或未生效（回执校验未通过）：请检查 Bucket 的 CORS、AccessKey 的 PutObject 权限，或改用 storage.oss.upload_mode=put 并配好 CORS。' +
          ((err as Error)?.message ? ' 原因：' + (err as Error).message : '')
      )
    }
  }

  // 仅本地磁盘存储（未开启直传）才使用服务端中转
  const uploaded = await fetchUploadFile(file, bizType)
  return mapFile(uploaded)
}

export default uploadFile
