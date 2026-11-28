// Quét giao diện QUẢN TRỊ để lập "danh mục chuỗi tiếng Việt" phục vụ đa ngôn ngữ.
//
//   node scripts/extract-ui-strings.mjs             -> ghi backend-go/internal/handlers/uicatalog/{catalog.vi.json,en.json}
//   node scripts/extract-ui-strings.mjs --check     -> báo chuỗi nào chưa có bản dịch tiếng Anh (thoát mã 1 nếu thiếu)
//   node scripts/extract-ui-strings.mjs --missing   -> in "số<TAB>chuỗi" của các chuỗi chưa có bản dịch tiếng Anh
//   node scripts/extract-ui-strings.mjs --prune     -> bỏ khỏi src/i18n/en.json các câu không còn dùng trong giao diện
//   node scripts/extract-ui-strings.mjs --merge f.tsv -> nạp bản dịch "số<TAB>English" (số theo danh sách --missing) vào src/i18n/en.json
//
// Cách hoạt động: trình duyệt chỉ dịch ĐÚNG những chuỗi nằm trong danh mục này (so khớp nguyên văn), nên nội dung của khách
// (tin nhắn, tên, sản phẩm...) không bao giờ bị dịch hay gửi đi đâu. Chuỗi có biến được ghi dạng mẫu: "Đã chọn {0} trang".
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { parse } from '@vue/compiler-sfc'
import ts from 'typescript'

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const SRC = path.join(ROOT, 'src')
const EN_FILE = path.join(SRC, 'i18n', 'en.json')
const OUT_DIR = path.resolve(ROOT, '..', 'backend-go', 'internal', 'handlers', 'uicatalog')

// Chữ cái tiếng Việt có dấu
const VI = /[àáạảãâầấậẩẫăằắặẳẵèéẹẻẽêềếệểễìíịỉĩòóọỏõôồốộổỗơờớợởỡùúụủũưừứựửữỳýỵỷỹđ]/i
const ATTRS = new Set(['placeholder', 'title', 'alt', 'aria-label', 'label'])
// Chữ giao diện hiếm khi dài quá ~200 ký tự; dài hơn thường là mẫu nội dung / câu lệnh AI (không dịch)
const MAX_LEN = 260

// Web công khai (đã có t() riêng theo thị trường) và chính module dịch — không đưa vào danh mục quản trị
const EXCLUDE = [
  /^src\/lib\/i18n\.ts$/,
  /^src\/views\/Public.*\.vue$/,
  /^src\/components\/store\/(CartDrawer|AccountMenu|CustomerAuthDialog|MarketSwitcher|NewsletterForm|PublicShell|BannerSlider)\.vue$/,
  /^src\/lib\/(publicSite|storeHost|cart|customer)\.ts$/,
  /\.d\.ts$/,
  /^src\/i18n\//
]

const norm = (s) => s.replace(/\s+/g, ' ').trim()
const strings = new Map() // chuỗi -> file đầu tiên chứa nó

function add(raw, file) {
  const s = norm(String(raw ?? ''))
  if (!s || s.length > MAX_LEN || !VI.test(s)) return
  // "{{full_name}}": mẫu tin nhắn gửi cho khách (nội dung của shop, không phải chữ giao diện).
  // Riêng câu gợi ý kết thúc bằng "..." (placeholder hướng dẫn nhập) thì vẫn là chữ giao diện.
  if (s.includes('{{') && !/(\.\.\.|…)$/.test(s)) return
  // Mẫu chỉ gồm biến + dấu câu thì bỏ
  const rest = s.replace(/\{\d+\}/g, '').replace(/[\s\d.,:;!?()[\]/%|·•–—-]+/g, '')
  if (rest.length < 2) return
  if (!strings.has(s)) strings.set(s, file)
}

// ---------- TypeScript / biểu thức ----------
function scanTs(code, file, kind = ts.ScriptKind.TS) {
  const sf = ts.createSourceFile('x.ts', code, ts.ScriptTarget.Latest, true, kind)
  const visit = (node) => {
    const parent = node.parent
    if (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) return
    if (ts.isCallExpression(node) && /^console\./.test(node.expression.getText(sf))) return
    if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) {
      const isKey = parent && (ts.isPropertyAssignment(parent) || ts.isPropertySignature(parent)) && parent.name === node
      if (!isKey) add(node.text, file)
      return
    }
    if (ts.isTemplateExpression(node)) {
      let pat = node.head.text
      node.templateSpans.forEach((sp, i) => {
        pat += `{${i}}` + sp.literal.text
      })
      add(pat, file)
      node.templateSpans.forEach((sp) => visit(sp.expression))
      return
    }
    ts.forEachChild(node, visit)
  }
  visit(sf)
}

function scanExpression(code, file) {
  if (!code || !VI.test(code)) return
  scanTs(`const __x = (\n${code}\n)`, file)
}

// ---------- Template Vue ----------
function walkTemplate(node, file) {
  const children = node.children || []
  let run = []
  const flush = () => {
    if (!run.length) return
    let text = ''
    let n = 0
    let hasVar = false
    for (const c of run) {
      if (c.type === 2) text += c.content
      else {
        text += `{${n++}}`
        hasVar = true
        scanExpression(c.content?.content, file) // chuỗi nằm trong biểu thức: {{ ok ? 'Bật' : 'Tắt' }}
      }
    }
    add(text, file)
    void hasVar
    run = []
  }
  for (const c of children) {
    if (c.type === 2 || c.type === 5) run.push(c)
    else if (c.type === 1) {
      flush()
      walkTemplate(c, file)
    } // type 3 (comment): bỏ qua, không cắt đoạn chữ
  }
  flush()

  if (node.type === 1) {
    for (const p of node.props || []) {
      if (p.type === 6 && ATTRS.has(p.name) && p.value) add(p.value.content, file)
      if (p.type === 7 && p.exp?.content) scanExpression(p.exp.content, file)
    }
  }
}

function scanVue(source, rel) {
  const { descriptor } = parse(source, { filename: rel })
  if (descriptor.template?.ast) walkTemplate(descriptor.template.ast, rel)
  for (const block of [descriptor.script, descriptor.scriptSetup]) {
    if (block) scanTs(block.content, rel, block.lang === 'tsx' ? ts.ScriptKind.TSX : ts.ScriptKind.TS)
  }
}

function listFiles(dir) {
  const out = []
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name)
    if (e.isDirectory()) out.push(...listFiles(p))
    else if (/\.(vue|ts)$/.test(e.name)) out.push(p)
  }
  return out
}

// ---------- Thông báo từ máy chủ (Go) ----------
// Giao diện hiển thị nguyên văn câu báo lỗi / thông báo máy chủ trả về ({"error": "..."}), nên các câu này cũng cần dịch.
// Câu dùng fmt.Sprintf: %s %d %v... được đổi thành {0} {1}... để so khớp theo mẫu.
const GO_DIR = path.resolve(ROOT, '..', 'backend-go', 'internal', 'handlers')
function goString(lit) {
  try {
    return JSON.parse('"' + lit + '"')
  } catch {
    return lit.replace(/\\"/g, '"').replace(/\\n/g, ' ')
  }
}
function scanGo() {
  if (!fs.existsSync(GO_DIR)) return
  const re = /"(?:error|message|warning|detail|hint)"\s*:\s*(fmt\.Sprintf\(\s*)?"((?:[^"\\\n]|\\.)*)"(\s*\+)?/g
  for (const f of fs.readdirSync(GO_DIR).sort()) {
    if (!f.endsWith('.go') || f.endsWith('_test.go')) continue
    const code = fs.readFileSync(path.join(GO_DIR, f), 'utf8')
    for (const m of code.matchAll(re)) {
      if (m[3]) continue // ghép nhiều đoạn bằng "+": không lấy nửa câu
      let s = goString(m[2])
      if (m[1] || /%[sdvqf]/.test(s)) {
        let i = 0
        s = s
          .replace(/%%/g, '')
          .replace(/%(?:\.\d+)?[sdvqf]/g, () => `{${i++}}`)
          .replace(//g, '%')
      }
      add(s, 'backend/' + f)
    }
  }
}

for (const abs of listFiles(SRC)) {
  const rel = path.relative(ROOT, abs).split(path.sep).join('/')
  if (EXCLUDE.some((re) => re.test(rel))) continue
  const code = fs.readFileSync(abs, 'utf8')
  if (!VI.test(code)) continue
  if (abs.endsWith('.vue')) scanVue(code, rel)
  else scanTs(code, rel)
}

scanGo()

const catalog = [...strings.keys()].sort((a, b) => (a < b ? -1 : a > b ? 1 : 0))
const en = fs.existsSync(EN_FILE) ? JSON.parse(fs.readFileSync(EN_FILE, 'utf8')) : {}
const missing = catalog.filter((s) => !(s in en) || !String(en[s]).trim())
const extra = Object.keys(en).filter((s) => !strings.has(s))

const args = process.argv.slice(2)
if (args.includes('--missing')) {
  missing.forEach((s, i) => console.log(`${i}\t${s}`))
  process.exit(0)
}
if (args.includes('--merge')) {
  const f = args[args.indexOf('--merge') + 1]
  const lines = fs.readFileSync(f, 'utf8').split(/\r?\n/).filter((l) => l.trim())
  let ok = 0
  for (const line of lines) {
    // Định dạng "số|English" (hoặc "số<TAB>English"); dấu | đầu tiên ngăn cách, phần còn lại là bản dịch
    const m = /^(\d+)\s*[|\t]\s?(.*)$/.exec(line)
    const idx = m ? Number(m[1]) : NaN
    const val = m ? m[2].trim() : ''
    if (!Number.isInteger(idx) || idx < 0 || idx >= missing.length || !val) {
      console.error('Dòng không hợp lệ:', line.slice(0, 80))
      continue
    }
    // Mẫu có biến phải giữ nguyên các {n}
    const ph = (s) => (s.match(/\{\d+\}/g) || []).sort().join(',')
    if (ph(missing[idx]) !== ph(val)) {
      console.error(`Sai biến {n} ở #${idx}: "${missing[idx]}" -> "${val}"`)
      continue
    }
    en[missing[idx]] = val
    ok++
  }
  fs.mkdirSync(path.dirname(EN_FILE), { recursive: true })
  fs.writeFileSync(EN_FILE, JSON.stringify(Object.fromEntries(Object.entries(en).sort(([a], [b]) => (a < b ? -1 : 1))), null, 1) + '\n')
  console.log(`Đã nạp ${ok}/${lines.length} dòng. Còn thiếu: ${catalog.filter((s) => !(s in en)).length}`)
  process.exit(0)
}

if (args.includes('--prune')) {
  const keep = Object.entries(en).filter(([k]) => strings.has(k)).sort(([a], [b]) => (a < b ? -1 : 1))
  fs.writeFileSync(EN_FILE, JSON.stringify(Object.fromEntries(keep), null, 1) + '\n')
  console.log(`Đã bỏ ${extra.length} câu tiếng Anh không còn được dùng trong giao diện.`)
  process.exit(0)
}

if (args.includes('--check')) {
  console.log(`Danh mục: ${catalog.length} chuỗi · đã có tiếng Anh: ${catalog.length - missing.length} · thiếu: ${missing.length} · thừa: ${extra.length}`)
  if (missing.length) console.log('Ví dụ thiếu:\n  ' + missing.slice(0, 15).join('\n  '))
  process.exit(missing.length ? 1 : 0)
}

fs.mkdirSync(OUT_DIR, { recursive: true })
fs.writeFileSync(path.join(OUT_DIR, 'catalog.vi.json'), JSON.stringify(catalog, null, 1) + '\n')
// Bản tiếng Anh chỉ giữ chuỗi còn trong danh mục (làm ngữ cảnh gợi ý cho AI khi dịch ngôn ngữ khác)
fs.writeFileSync(path.join(OUT_DIR, 'en.json'), JSON.stringify(Object.fromEntries(catalog.filter((s) => s in en).map((s) => [s, en[s]])), null, 1) + '\n')

const perFile = {}
for (const f of strings.values()) perFile[f] = (perFile[f] || 0) + 1
console.log(`Danh mục: ${catalog.length} chuỗi (mẫu có biến: ${catalog.filter((s) => /\{\d+\}/.test(s)).length}) · thiếu tiếng Anh: ${missing.length} · thừa: ${extra.length}`)
console.log('Theo file (chuỗi xuất hiện đầu tiên):')
Object.entries(perFile).sort((a, b) => b[1] - a[1]).slice(0, 25).forEach(([f, n]) => console.log(`  ${String(n).padStart(4)}  ${f}`))
