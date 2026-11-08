/**
 * 生成行政区划数据集（省/市/区县/街道/社区 五级）
 *
 *   node scripts/build-district-data.mjs
 *
 * 数据来源：国家统计局行政区划（开源镜像 modood/Administrative-divisions-of-China）。
 * 产物：internal/service/data/districts.json.gz（紧凑格式：[{c:编码,n:名称,l:层级}]），
 * 后端启动时若 pm_district 为空会自动导入（也提供 POST /property/districts/sync 手动重同步）。
 *
 * 说明：
 *  - 源文件 villages.json 约 85MB，直连容易被重置，这里用 HTTP Range 分片下载并落盘缓存，
 *    中断后重跑会自动复用已下载的分片，不会从头再来；
 *  - 生成物约 5~7MB（gzip），随二进制一起内嵌，部署环境无需外网。
 */
import { createWriteStream, existsSync, mkdirSync, readFileSync, statSync, writeFileSync } from 'node:fs'
import { createGzip } from 'node:zlib'
import { join } from 'node:path'

const MIRRORS = [
  'https://raw.githubusercontent.com/modood/Administrative-divisions-of-China/master/dist/',
  'https://ghproxy.net/https://raw.githubusercontent.com/modood/Administrative-divisions-of-China/master/dist/'
]
const FILES = [
  ['provinces.json', 2],
  ['cities.json', 4],
  ['areas.json', 6],
  ['streets.json', 9],
  ['villages.json', 12]
]
const CHUNK = 1024 * 1024
const CACHE_DIR = join(process.cwd(), '.cache', 'districts')
const OUT = join(process.cwd(), 'internal', 'service', 'data', 'districts.json.gz')

mkdirSync(CACHE_DIR, { recursive: true })

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

/** 小文件直接下载；大文件走 Range 分片（带续传） */
async function download(path) {
  const target = join(CACHE_DIR, path)
  if (existsSync(target) && statSync(target).size > 0) {
    console.log(`  复用缓存 ${path}（${statSync(target).size} 字节）`)
    return readFileSync(target, 'utf8')
  }
  const part = target + '.part'
  let offset = existsSync(part) ? statSync(part).size : 0
  let total = 0
  let mirror = 0

  while (true) {
    const url = MIRRORS[mirror % MIRRORS.length] + path
    try {
      const headers = offset > 0 ? { Range: `bytes=${offset}-${offset + CHUNK - 1}` } : {}
      const res = await fetch(url, { headers })
      if (res.status === 416) break // 已经下完
      if (res.status !== 200 && res.status !== 206) throw new Error('status ' + res.status)
      const buf = Buffer.from(await res.arrayBuffer())
      if (res.status === 200) {
        // 服务端不支持 Range：一次性重下
        writeFileSync(part, buf)
        offset = buf.length
      } else {
        const cr = res.headers.get('content-range') || ''
        total = Number(cr.split('/')[1] || 0)
        const stream = createWriteStream(part, { flags: offset === 0 ? 'w' : 'a' })
        stream.write(buf)
        await new Promise((r) => stream.end(r))
        offset += buf.length
      }
      process.stdout.write(`\r  ${path} ${(offset / 1024 / 1024).toFixed(1)}MB / ${(total / 1024 / 1024).toFixed(1)}MB`)
      if (total && offset >= total) break
      if (!total && res.status === 200) break
    } catch (e) {
      mirror++
      process.stdout.write(`\r  ${path} 重试（${e.message}）`)
      await sleep(500)
    }
  }
  process.stdout.write('\n')

  const text = readFileSync(part, 'utf8')
  writeFileSync(target, text)
  return text
}

const rows = []
for (const [file, level] of FILES) {
  const text = await download(file)
  const arr = JSON.parse(text)
  console.log(`  ${file}: ${arr.length} 条`)
  for (const it of arr) rows.push({ c: it.code, n: it.name, l: level })
}
console.log(`合计 ${rows.length} 条`)

const gz = createGzip({ level: 9 }).end(JSON.stringify(rows))
const chunks = []
for await (const c of gz) chunks.push(c)
const buf = Buffer.concat(chunks)
mkdirSync(join(process.cwd(), 'internal', 'service', 'data'), { recursive: true })
writeFileSync(OUT, buf)
console.log(`已生成 ${OUT}（${(buf.length / 1024 / 1024).toFixed(2)} MB）`)
