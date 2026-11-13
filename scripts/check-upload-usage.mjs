/**
 * 上传规范检查：文件存储必须使用 @/utils/upload 的 uploadFile(file, { bizType })。
 *
 * 禁止：
 *  1) uploadFile('/api/...', file) 这种旧式"路径 + 文件"调用（应改用新 helper）；
 *  2) 直接在业务页面里写 /files/upload（应通过 helper，自动直传/回退）。
 * 允许：
 *  - utils/upload/index.ts 内部实现；
 *  - api/business.ts 里的 fetchUploadFile / oss-ticket / oss-record 封装；
 *  - Excel 等业务导入使用 uploadBusinessFile（服务端必须读取文件内容）。
 *
 * 用法：node scripts/check-upload-usage.mjs
 */
import fs from 'node:fs'
import path from 'node:path'

const root = 'web/src'
const allowFiles = new Set([
  path.normalize('web/src/utils/upload/index.ts'),
  path.normalize('web/src/api/business.ts'),
  path.normalize('web/src/utils/web-basic.ts')
])

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, entry.name)
    if (entry.isDirectory()) walk(p, out)
    else if (/\.(ts|vue)$/.test(entry.name)) out.push(p)
  }
  return out
}

const problems = []
for (const file of walk(root)) {
  const key = path.normalize(file)
  if (allowFiles.has(key)) continue
  const lines = fs.readFileSync(file, 'utf8').split(/\r?\n/)
  lines.forEach((line, i) => {
    if (/uploadFile\s*\(\s*['"`]\/api\//.test(line)) {
      problems.push(`${file}:${i + 1} 旧式 uploadFile(path, file)，请改用 @/utils/upload 的 uploadFile(file, { bizType })`)
    }
    if (line.includes('/files/upload') && !line.includes('//')) {
      problems.push(`${file}:${i + 1} 直接调用 /files/upload，请通过 @/utils/upload 的 uploadFile`)
    }
  })
}

if (problems.length) {
  console.error('上传规范检查未通过：')
  problems.forEach((p) => console.error(' - ' + p))
  process.exit(1)
}
console.log('上传规范检查通过：业务代码统一使用 @/utils/upload 的 uploadFile')
