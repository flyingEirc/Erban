<template>
  <div class="modal-mask" @click.self="emit('close')">
    <div class="modal">
      <header class="modal-header">{{ isEdit ? '编辑 SSH' : '新建 SSH' }}</header>

      <div class="modal-body layout">
        <!-- Left vertical nav -->
        <nav class="vnav">
          <button class="vnav-item" :class="{ active: activeNav === 'ssh' }" @click="activeNav = 'ssh'">SSH 连接</button>
          <button class="vnav-item" :class="{ active: activeNav === 'proxy' }" @click="activeNav = 'proxy'">代理</button>
        </nav>

        <!-- Right content -->
        <section class="panel">
          <!-- SSH 连接：常规 + 认证 同屏纵向展示（无折叠） -->
          <div v-if="activeNav === 'ssh'" class="panel-inner">
            <!-- 常规 Section -->
            <div class="section">
              <div class="section-title">
                <span>常规</span>
                <span class="spacer"></span>
              </div>
              <div class="section-body">
                <div class="form-grid">
                  <label class="lbl" for="label">名称</label>
                  <div class="ctrl">
                    <input id="label" v-model.trim="form.label"  class="w-360" :aria-invalid="submitted && !!errors.label" :aria-describedby="submitted && errors.label ? 'err-label' : undefined" />
                    <p v-if="submitted && errors.label" id="err-label" class="err">{{ errors.label }}</p>
                  </div>

                  <label class="lbl" for="host">主机</label>
                  <div class="ctrl">
                    <div class="row-inline">
                      <input id="host" v-model.trim="hostOnly"  class="w-360" :aria-invalid="submitted && !!errors.host" :aria-describedby="submitted && errors.host ? 'err-host' : undefined" />
                      <input id="port" v-model.trim="portText" type="text" inputmode="numeric"  class="w-160" :aria-invalid="submitted && !!errors.port" :aria-describedby="submitted && errors.port ? 'err-port' : undefined" />
                    </div>
                    <p v-if="submitted && errors.host" id="err-host" class="err">{{ errors.host }}</p>
                    <p v-if="submitted && errors.port" id="err-port" class="err">{{ errors.port }}</p>
                  </div>
                </div>
              </div>
            </div>

            <!-- 认证 Section -->
            <div class="section">
              <div class="section-title">
                <span>认证</span>
                <span class="spacer"></span>
              </div>
              <div class="section-body">
                <div class="form-grid">
                  <label class="lbl">方法</label>
                  <div class="ctrl">
                    <div class="seg">
                      <button type="button"
                              class="seg-item"
                              :class="{ active: form.auth==='password' }"
                              @click="form.auth='password'">密码</button>
                      <button type="button"
                              class="seg-item"
                              :class="{ active: form.auth==='key' }"
                              @click="form.auth='key'">私钥</button>
                    </div>
                  </div>

                  <label class="lbl" for="user">用户名</label>
                  <div class="ctrl">
                    <input id="user" v-model.trim="form.user" placeholder="root" class="w-240" :aria-invalid="submitted && !!errors.user" :aria-describedby="submitted && errors.user ? 'err-user' : undefined" />
                    <p v-if="submitted && errors.user" id="err-user" class="err">{{ errors.user }}</p>
                  </div>

                  <template v-if="form.auth==='password'">
                    <label class="lbl" for="password">密码</label>
                    <div class="ctrl">
                      <input id="password" v-model="form.password" type="password" placeholder="登录密码" class="w-240" :aria-invalid="submitted && !!errors.secret" :aria-describedby="submitted && errors.secret ? 'err-secret' : undefined" />
                      <p v-if="submitted && errors.secret" id="err-secret" class="err">{{ errors.secret }}</p>
                    </div>
                  </template>
                  <template v-else>
                    <label class="lbl">私钥文件</label>
                    <div class="ctrl">
                      <div class="file-selector w-480">
                        <button type="button" class="file-btn" @click="triggerFileInput">选择文件</button>
                        <span class="file-path" v-if="selectedFileName">{{ selectedFileName }}</span>
                        <span class="file-path empty" v-else>未选择文件</span>
                        <input ref="fileInput" type="file" @change="onPickFile" style="display: none" accept=".pem,.key" />
                      </div>
                      <p v-if="submitted && errors.secret" class="err">{{ errors.secret }}</p>
                    </div>
                  </template>
                </div>
              </div>
            </div>
          </div>

          <!-- 代理服务器：左侧文本，右侧文本框自适应宽度 -->
          <div v-else-if="activeNav === 'proxy'" class="panel-inner">
            <div class="section">
              <div class="section-title">
                <span>代理服务器</span>
              </div>
              <div class="section-body">
                <div class="form-grid">
                  <label class="lbl">协议</label>
                  <div class="ctrl">
                    <div class="seg">
                      <button type="button"
                              class="seg-item"
                              :class="{ active: proxy.protocol==='http' }"
                              @click="proxy.protocol='http'">HTTP</button>
                      <button type="button"
                              class="seg-item"
                              :class="{ active: proxy.protocol==='socks5' }"
                              @click="proxy.protocol='socks5'">SOCKS5</button>
                    </div>
                  </div>
                  <label class="lbl" for="proxy-addr">代理地址</label>
                  <div class="ctrl">
                    <input id="proxy-addr" v-model.trim="proxy.address" placeholder="127.0.0.1:10808" class="w-480" />
                  </div>

                  <label class="lbl" for="proxy-user">用户名</label>
                  <div class="ctrl">
                    <input id="proxy-user" v-model.trim="proxy.user" placeholder="可选" class="w-280" />
                  </div>

                  <label class="lbl" for="proxy-pass">密码</label>
                  <div class="ctrl">
                    <input id="proxy-pass" v-model="proxy.password" type="password" placeholder="可选" class="w-280" />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>
      </div>

      <footer class="modal-actions">
        <button class="btn-cancel" @click="emit('close')">取消</button>
        <button class="btn-primary" :disabled="!isValid" @click="onSubmit">{{ isEdit ? '保存' : '创建' }}</button>
      </footer>
    </div>
  </div>
  
</template>

<script setup lang="ts">
import { reactive, computed, ref, onMounted, watch } from 'vue'

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'create', payload: { label: string; host: string; user: string; auth: 'password'|'key'; password?: string; pemId?: string; pemDataB64?: string; proxy?: string }): void
}>()

const props = defineProps<{
  initial?: { label: string; host: string; user: string; auth: 'password'|'key'; password?: string; proxy?: string }
}>()

const fileInput = ref<HTMLInputElement>()

// Left nav state
const activeNav = ref<'ssh'|'proxy'|'tunnel'>('ssh')
// No collapsible sections now

// SSH form
const form = reactive({
  label: '',
  user: 'root',
  auth: 'password' as 'password'|'key', 
  password: '',
  pemId: '',
  pemDataB64: ''
})

// UI-only selected filename
const selectedFileName = ref('')

// Host/port separated for better UX; combined on submit
const hostOnly = ref('')
const portText = ref('22')

// Proxy settings
const proxy = reactive({
  protocol: 'http' as 'http' | 'socks5',
  address: '',
  user: '',
  password: ''
})

const proxyPreview = computed(() => buildProxyUrl())

const isEdit = computed(() => !!props.initial)

const isValid = computed(() => {
  // Name, host, port, user are required
  if (!form.label.trim()) return false
  if (!hostOnly.value.trim()) return false
  if (!form.user.trim()) return false
  const port = Number(portText.value)
  if (!port || !Number.isInteger(port) || port < 1 || port > 65535) return false
  if (form.auth === 'password') return form.password.length > 0
  return !!(form.pemId || form.pemDataB64)
})

const errors = computed(() => {
  const e: Record<string, string | null> = { label: null, host: null, port: null, user: null, secret: null }
  if (!form.label.trim()) e.label = '请输入名称'
  if (!hostOnly.value.trim()) e.host = '请输入主机名或 IP'
  const p = Number(portText.value)
  if (!p || !Number.isInteger(p) || p < 1 || p > 65535) e.port = '端口需为 1-65535 的数字'
  if (!form.user.trim()) e.user = '请输入用户名'
  if (form.auth === 'password') {
    if (!form.password) e.secret = '请输入密码'
  } else {
    if (!form.pemId && !form.pemDataB64) e.secret = '请选择私钥文件'
  }
  return e
})

const submitted = ref(false)

function onSubmit() {
  submitted.value = true
  if (!isValid.value) return
  create()
}

function create() {
  const host = hostOnly.value.trim()
  const port = Number(portText.value || 22)
  const hostWithPort = port ? `${host}:${port}` : host

  emit('create', {
    label: form.label.trim(),
    host: hostWithPort,
    user: form.user.trim(),
    auth: form.auth,
    password: form.auth === 'password' ? form.password : undefined,
    pemId: form.auth === 'key' ? form.pemId : undefined,
    pemDataB64: form.auth === 'key' ? form.pemDataB64 : undefined,
    proxy: buildProxyUrl() || undefined,
  })
}

function triggerFileInput() {
  fileInput.value?.click()
}

async function onPickFile(e: Event) {
  const input = e.target as HTMLInputElement
  const files = input.files
  if (!files || files.length === 0) return

  const f: any = files[0]
  selectedFileName.value = f.name || ''
  // Read file as base64 (Data URL) and store the payload part
  try {
    const reader = new FileReader()
    reader.onload = async () => {
      const res = reader.result as string
      if (typeof res === 'string') {
        const i = res.indexOf(',')
        form.pemDataB64 = i >= 0 ? res.slice(i + 1) : res
        try {
          // Use filename as the key and file content (base64) as the value
          form.pemId = selectedFileName.value || (f && (f as File).name) || ''
          const bridge: any = (window as any).go?.main?.SSHBridge
          if (bridge && bridge.KeyPut && form.pemId) {
            const err = await bridge.KeyPut(form.pemId, form.pemDataB64)
            if (err) console.error('KeyPut error:', err)
          }
        } catch (err) {
          console.warn('store key error:', err)
        }
      }
    }
    reader.readAsDataURL(f as File)
  } catch {}
}

/* No collapsible sections, so toggleCollapse is not needed */

function applyInitial() {
  const init = props.initial
  if (!init) return
  form.label = init.label || ''
  form.user = init.user || 'root'
  form.auth = init.auth || 'password'
  form.password = init.password || ''
  form.pemId = ''
  selectedFileName.value = ''
  // parse host:port
  let h = init.host || ''
  let p: number | null = 22
  const idx = h.lastIndexOf(':')
  if (idx > -1) {
    const maybePort = h.slice(idx + 1)
    if (/^\d+$/.test(maybePort)) {
      p = Number(maybePort)
      h = h.slice(0, idx)
    }
  }
  hostOnly.value = h
  portText.value = String(p ?? 22)

  // parse proxy url if provided: scheme://[user[:pass]@]host:port
  if (init.proxy && typeof init.proxy === 'string' && init.proxy.trim().length > 0) {
    try {
      const u = new URL(init.proxy)
      proxy.protocol = (u.protocol.replace(':','') as any) || 'http'
      const hp = [u.hostname, u.port].filter(Boolean).join(':')
      proxy.address = hp
      proxy.user = decodeURIComponent(u.username || '')
      proxy.password = decodeURIComponent(u.password || '')
    } catch {
      const m = init.proxy.match(/^(\w+):\/\/(?:([^:@\/]+)(?::([^@\/]*))?@)?([^:\/]+)(?::(\d+))?/)
      if (m) {
        proxy.protocol = (m[1] as any) || 'http'
        proxy.user = m[2] ? decodeURIComponent(m[2]) : ''
        proxy.password = m[3] ? decodeURIComponent(m[3]) : ''
        proxy.address = [m[4] || '', m[5] || ''].filter(Boolean).join(':')
      }
    }
  }
}

watch(() => props.initial, () => applyInitial(), { immediate: true })
onMounted(applyInitial)

function normalizeHostPort(addr: string): string {
  if (!addr) return ''
  let a = addr.trim()
  // Strip any scheme if user pasted a full URL
  a = a.replace(/^\w+:\/\//, '')
  // Strip credentials if included
  const at = a.lastIndexOf('@')
  if (at !== -1) a = a.slice(at + 1)
  return a
}

function buildProxyUrl(): string {
  const hp = normalizeHostPort(proxy.address)
  if (!hp) return ''
  const scheme = proxy.protocol === 'socks5' ? 'socks5' : 'http'
  const user = (proxy.user || '').trim()
  const pass = proxy.password ?? ''
  const cred = user ? `${encodeURIComponent(user)}${pass ? ':' + encodeURIComponent(pass) : ''}@` : ''
  return `${scheme}://${cred}${hp}`
}
</script>

<style scoped>
.modal-mask {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  display: grid;
  place-items: center;
  z-index: 50;
}

.modal {
  width: 860px; /* 更紧凑 */
  max-width: calc(100vw - 48px);
  background: #111827;
  color: #e5e7eb;
  /* Separator style tokens for consistent lines */
  --sep-color: rgba(255, 255, 255, 0.08);
  --sep-width: 1px;
  border: var(--sep-width) solid var(--sep-color);
  border-radius: 10px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5);
}

.modal-header {
  padding: 10px 24px; /* 上下更紧凑 */
  font-weight: 600;
  font-size: 15px;
  border-bottom: var(--sep-width) solid var(--sep-color);
}

.modal-body.layout {
  padding: 0;
  display: grid;
  grid-template-columns: 200px 1fr;
  min-height: 320px; /* 更紧凑 */
}

/* Left vertical nav */
.vnav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 12px;
  border-right: var(--sep-width) solid var(--sep-color);
}
.vnav-item {
  text-align: left;
  padding: 8px 10px; /* 更紧凑 */
  border-radius: 8px;
  border: 1px solid transparent;
  background: transparent;
  color: rgba(229,231,235,0.85);
  cursor: pointer;
  transition: all .2s;
}
.vnav-item:hover { background: rgba(255,255,255,0.04); }
.vnav-item.active { background: rgba(37,99,235,0.15); border-color: rgba(37,99,235,0.35); color: #fff; }

/* Right panel */
.panel { height: 100%; overflow: auto; padding: 16px 24px; } /* 固定高度内滚动 */
.panel-inner { display: flex; flex-direction: column; gap: 10px; }

.section { display: flex; flex-direction: column; gap: 12px; margin-bottom: 16px; }
.section:last-child { margin-bottom: 0; }
.section-title { display: flex; align-items: center; font-weight: 600; font-size: 14px; padding-bottom: 4px; }
.section-title .spacer { flex: 1; }
.section-title .caret { display: none; }
.section.collapsible .section-title { cursor: default; }
.section-body { display: block; }

.form-grid { display: grid; grid-template-columns: 50px 1fr; column-gap: 12px; row-gap: 12px; align-items: center; }
.lbl { text-align: right; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; color: rgba(115, 156, 237, 0.9); font-size: 13px; }
.ctrl { display: flex; flex-direction: column; align-items: flex-start; min-width: 0; }
.row-inline { display: flex; gap: 12px; align-items: center; width: 100%; }
.flex-1 { flex: 1 1 auto; min-width: 0; }
.ctrl-inline { display: flex; align-items: center; gap: 8px; }

label { font-size: 13px; color: rgba(229, 231, 235, 0.85); text-align: left; }

input,
select {
  width: 100%;
  /* Improve text-to-underline spacing for clarity */
  box-sizing: border-box;
  line-height: 1.4;
  padding: 8px 10px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.253);
  background: transparent;
  color: #ffffff;
  font-size: 13px;
  transition: all 0.2s;
}

input:focus,
select:focus { outline: none; border-color: rgba(37, 99, 235, 0.6); background: rgba(255, 255, 255, 0.03); }

/* 提升 placeholder 对比度 */
input::placeholder, select::placeholder { color: rgba(229,231,235,0.6); }

/* File picker */
.file-selector { display: flex; align-items: center; gap: 10px; }
.file-btn {
  padding: 6px 14px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  background: transparent;
  color: #e5e7eb;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
  flex-shrink: 0;
}
.file-btn:hover { background: rgba(255,255,255,0.05); border-color: rgba(255,255,255,0.25); }
.file-path { font-size: 12px; color: rgba(229,231,235,0.7); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.file-path.empty { color: rgba(229,231,235,0.35); }

/* 控件建议宽度 */
.w-160 { width: 120px; max-width: 80%; }
.w-200 { width: 200px; max-width: 80%; }
.w-240 { width: 240px; max-width: 80%; }
.w-280 { width: 280px; max-width: 80%; }
.w-360 { width: 300px; max-width: 80%; }
.w-480 { width: 480px; max-width: 80%; }
.ctrl > input, .ctrl > select, .ctrl > .file-selector { max-width: 100%; }

/* 错误提示 */
.err { color: #fca5a5; font-size: 12px; line-height: 14px; margin-top: 4px; }

/* 覆盖高度以保持弹窗尺寸在切换面板时不变 */
.modal-body.layout { height: 520px; }
.vnav { height: 100%; box-sizing: border-box; }

/* 缩短左侧导航列与按钮长度 */
.modal-body.layout { grid-template-columns: 140px 1fr; }
.vnav-item { width: 110px; padding: 6px 8px; font-size: 13px; }

/* 认证方式分段按钮 */
.seg { display: inline-flex; background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 2px; }
.seg-item { padding: 4px 10px; font-size: 12px; color: #cbd5e1; background: transparent; border: 0; border-radius: 6px; cursor: pointer; }
.seg-item.active { background: #2563eb; color: #fff; }
.seg-item:not(.active):hover { background: rgba(255,255,255,0.06); }

/* Footer */
.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 10px 24px; /* 上下更紧凑 */
  border-top: var(--sep-width) solid var(--sep-color);
}

.btn-cancel,
.btn-primary {
  padding: 7px 18px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  border: 1px solid;
}
.btn-cancel { background: transparent; color: rgba(229,231,235,0.85); border-color: rgba(255,255,255,0.2); }
.btn-cancel:hover { background: rgba(255,255,255,0.05); border-color: rgba(255,255,255,0.3); }
.btn-primary { background: #2563eb; border-color: #2563eb; color: #fff; }
.btn-primary:hover:not(:disabled) { background: #1d4ed8; border-color: #1d4ed8; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

/* Hide placeholder tip inside tunnel panel now that UI is implemented */
.panel-inner .empty-tip { display: none; }

/* Responsive */
@media (max-width: 860px) {
  .modal { width: 96vw; }
  .modal-body.layout { grid-template-columns: 160px 1fr; }
}
/* <1280 时退化为单列表单（label 上方） */
@media (max-width: 1279px) {
  .form-grid { display: block; }
  .lbl { display: block; text-align: left; margin-bottom: 8px; }
  .ctrl { margin-bottom: 16px; }
}
@media (max-width: 560px) {
  .modal-body.layout { display: block; }
  .vnav { flex-direction: row; border-right: none; border-bottom: var(--sep-width) solid var(--sep-color); }
  .ctrl { margin-bottom: 16px; }
}
</style>
