<template>
  <div class="modal-mask" @click.self="close">
    <div class="modal">
      <header class="modal-header">
        <span>端口转发</span>
        <button class="close" type="button" @click="close">×</button>
      </header>

      <section class="modal-body">
        <p class="session-hint" :class="{ inactive: !props.active }"> 当前 SSH 会话： {{ props.active ? '已连接 - 您可以创建转发' : '未连接 - 您只能查看/停止现有转发' }} </p>

        <div class="section">
          <div class="section-title">
            <span>新建转发</span>
          </div>
          <div class="section-body">
            <div class="form-field">
              <label>转发类型</label>
              <select v-model="tunnel.mode">
                <option value="local">本地</option>
                <option value="remote">远程</option>
                <option value="dynamic">动态 SOCKS5</option>
              </select>
            </div>
            <div class="mode-switch" v-if="false">
              <button type="button" class="seg-item" :class="{ active: tunnel.mode === 'local' }" @click="tunnel.mode = 'local'">本地</button>
              <button type="button" class="seg-item" :class="{ active: tunnel.mode === 'remote' }" @click="tunnel.mode = 'remote'">Remote</button>
              <button type="button" class="seg-item" :class="{ active: tunnel.mode === 'dynamic' }" @click="tunnel.mode = 'dynamic'">动态 SOCKS5</button>
            </div>

            <div v-if="tunnel.mode === 'local'" class="form-grid">
              <div class="form-field">
                <label>本地监听</label>
                <div class="inline">
                  <input v-model.trim="tunnel.local.fromIP" placeholder="127.0.0.1" />
                  <input v-model="tunnel.local.fromPort" @input="sanitizePortInput" placeholder="9000" inputmode="numeric" />
                </div>
              </div>
              <div class="form-field">
                <label>远程目标</label>
                <div class="inline">
                  <input v-model.trim="tunnel.local.toIP" placeholder="host" readonly />
                  <input v-model="tunnel.local.toPort" @input="sanitizePortInput" placeholder="80" inputmode="numeric" />
                </div>
              </div>
            </div>

            <div v-else-if="tunnel.mode === 'remote'" class="form-grid">
              <div class="form-field">
                <label>远程绑定</label>
                <div class="inline">
                  <input v-model.trim="tunnel.remote.fromIP" placeholder="0.0.0.0" readonly />
                  <input v-model="tunnel.remote.fromPort" @input="sanitizePortInput" placeholder="10022" inputmode="numeric" />
                </div>
              </div>
              <div class="form-field">
                <label>本地目标</label>
                <div class="inline">
                  <input v-model.trim="tunnel.remote.toIP" placeholder="127.0.0.1" />
                  <input v-model="tunnel.remote.toPort" @input="sanitizePortInput" placeholder="22" inputmode="numeric" />
                </div>
              </div>
            </div>

            <div v-else class="form-grid">
              <div class="form-field">
                <label>SOCKS 监听地址</label>
                <input v-model.trim="tunnel.dynamic.bind" placeholder="127.0.0.1:1080" />
              </div>
            </div>

            <div class="actions-row">
              <button
                class="action-btn action-start"
                :disabled="!props.active || tunnelBusy || !props.sessionId"
                @click="onStartForward"
              >创建转发</button>
              <button
                class="action-btn action-refresh"
                :disabled="tunnelBusy"
                @click="refreshForwards"
              >刷新</button>
              <button
                class="action-btn action-stop"
                :disabled="tunnelBusy || !props.sessionId || forwards.length === 0"
                @click="onStopAll"
              >全部停止</button>
            </div>
            <p v-if="tunnelErr" class="err">{{ tunnelErr }}</p>
          </div>
        </div>

        <div class="section">
          <div class="section-title">
            <span>转发列表</span>
          </div>
          <div class="section-body">
            <div v-if="forwards.length === 0" class="empty-card">
              <div class="empty-title">暂无转发</div>
              <p class="empty-sub">No forwards，请点击上方“Create Forward”开始</p>
            </div>
            <div v-else class="forward-list">
              <div v-for="f in forwards" :key="f.id" class="forward-item">
                <div class="forward-info">
                  <div class="line">{{ labelForMode(f.mode) }} · {{ f.from }}<template v-if="f.to"> → {{ f.to }}</template></div>
                  <div class="sub">ID：{{ f.id }}</div>
                </div>
                <button class="btn-link" :disabled="tunnelBusy" @click="onStop(f.id)">停止</button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <footer class="modal-actions">
        <button class="btn-secondary" @click="close">关闭</button>
      </footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch, computed } from 'vue'
import { listForwards, startDynamicForward, startLocalForward, startRemoteForward, stopAllForwards, stopForward, type ForwardItem } from '../services/portforward'

const props = defineProps<{
  instances: Array<{ id: string; label?: string; host?: string; user?: string }>
  active: boolean
  sessionId: string
  activeInstanceId?: string
}>()

const emit = defineEmits<{ (e: 'close'): void }>()

const tunnel = reactive({
  mode: 'local' as 'local' | 'remote' | 'dynamic',
  local: { fromIP: '127.0.0.1', fromPort: '', toIP: '', toPort: '' },
  remote: { fromIP: '0.0.0.0', fromPort: '', toIP: '127.0.0.1', toPort: '' },
  dynamic: { bind: '' }
})

const forwards = ref<ForwardItem[]>([])
const tunnelBusy = ref(false)
const tunnelErr = ref('')

function close() {
  emit('close')
}

function labelForMode(mode: ForwardItem['mode']) {
  if (mode === 'local') return '本地'
  if (mode === 'remote') return '远程'
  return '动态'
}

function validAddr(addr: string) {
  if (!addr) return false
  const value = addr.trim()
  const match = /^((25[0-5]|2[0-4]\d|1\d{2}|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d{2}|[1-9]?\d)){3}):(\d+)$/.exec(value)
  if (!match) return false
  const port = Number(match[5])
  return port >= 0 && port <= 65535
}

// ---- Validation helpers ----
function validIP(ip: string) {
  if (!ip) return false
  const r = /^(25[0-5]|2[0-4]\d|1\d{2}|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d{2}|[1-9]?\d)){3}$/
  return r.test(ip.trim())
}

function validPort(port: string) {
  if (!port) return false
  const n = Number(port)
  return Number.isInteger(n) && n >= 0 && n <= 65535
}

// 端口输入限定为数字（0-9）
function sanitizePortInput(e: Event) {
  const t = e.target as HTMLInputElement
  const cleaned = (t.value || '').replace(/[^0-9]/g, '')
  t.value = cleaned
}

// 当前激活实例（用于自动填充本地/远程默认 IP）
const activeInstance = computed(() => props.instances.find(i => i.id === props.activeInstanceId))

function applyAutoIPDefaults() {
  // 本地转发：远程目标 IP 使用当前实例 host（若可用）
  if (activeInstance?.value?.host) {
    const parts = activeInstance.value.host.split(":");
    console.log(parts)
    tunnel.local.toIP = parts[0];
    tunnel.local.toPort = parts[1];
  }
  // 远程转发：远程绑定 IP 固定为 0.0.0.0
  tunnel.remote.fromIP = '0.0.0.0'
}

// ---- List & control forwards ----
async function refreshForwards() {
  try {
    tunnelErr.value = ''
    if (!props.sessionId) {
      forwards.value = []
      return
    }
    forwards.value = await listForwards(props.sessionId)
  } catch (err) {
    console.warn('list forwards failed', err)
  }
}

async function onStop(id: string) {
  try {
    tunnelErr.value = ''
    if (!props.sessionId) {
      tunnelErr.value = 'No active session'
      return
    }
    const err = await stopForward(props.sessionId, id)
    if (err) tunnelErr.value = err
    await refreshForwards()
  } catch (err: any) {
    tunnelErr.value = err?.message || String(err)
  }
}

async function onStopAll() {
  try {
    tunnelErr.value = ''
    if (!props.sessionId) {
      tunnelErr.value = 'No active session'
      return
    }
    const err = await stopAllForwards(props.sessionId)
    if (err) tunnelErr.value = err
    await refreshForwards()
  } catch (err: any) {
    tunnelErr.value = err?.message || String(err)
  }
}
async function onStartForward() {
  if (!props.active || tunnelBusy.value) return
  if (!props.sessionId) {
    tunnelErr.value = "No active session"
    return
  }
  tunnelBusy.value = true
  tunnelErr.value = ''
  try {
    let err = ''
    if (tunnel.mode === 'local') {
      if (!validIP(tunnel.local.fromIP) || !validPort(tunnel.local.fromPort) || !validIP(tunnel.local.toIP) || !validPort(tunnel.local.toPort)) {
        tunnelErr.value = '本地绑定/端口或远端目标 IP/端口格式无效'
        return
      }
      const from = `${tunnel.local.fromIP}:${tunnel.local.fromPort}`
      const to = `${tunnel.local.toIP}:${tunnel.local.toPort}`
      err = await startLocalForward(props.sessionId, from, to)
    } else if (tunnel.mode === 'remote') {
      if (!validIP(tunnel.remote.fromIP) || !validPort(tunnel.remote.fromPort) || !validIP(tunnel.remote.toIP) || !validPort(tunnel.remote.toPort)) {
        tunnelErr.value = '远程绑定/端口或本地目标 IP/端口格式无效'
        return
      }
      const from = `${tunnel.remote.fromIP}:${tunnel.remote.fromPort}`
      const to = `${tunnel.remote.toIP}:${tunnel.remote.toPort}`
      err = await startRemoteForward(props.sessionId, from, to)
    } else {
      if (!validAddr(tunnel.dynamic.bind)) {
        tunnelErr.value = '请输入有效的 SOCKS 监听地址'
        return
      }
      err = await startDynamicForward(props.sessionId, tunnel.dynamic.bind)
    }
    if (err) tunnelErr.value = err
    await refreshForwards()
  } catch (err) {
    const msg = (err && typeof err === 'object' && 'message' in (err as any))
      ? String((err as any).message)
      : String(err)
    tunnelErr.value = msg
  } finally {
    tunnelBusy.value = false
  }
}
// ---- Lifecycle & watchers ----
onMounted(() => {
  applyAutoIPDefaults()
  void refreshForwards()
})
watch(() => props.sessionId, () => { void refreshForwards() })
watch(() => props.active, (val, oldVal) => { if (val && !oldVal) void refreshForwards() })
watch(() => props.instances.length, () => { void refreshForwards() })
watch(() => props.activeInstanceId, applyAutoIPDefaults)
watch(() => tunnel.mode, applyAutoIPDefaults)
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
  width: 720px;
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
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  font-weight: 600;
  font-size: 15px;
  border-bottom: var(--sep-width) solid var(--sep-color);
}

.modal-header .close {
  appearance: none;
  background: transparent;
  border: none;
  color: rgba(229, 231, 235, 0.85);
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 6px;
}

.modal-header .close:hover {
  background: rgba(255, 255, 255, 0.06);
}

.modal-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px 20px;
}

.session-hint.inactive {
  border-color: rgba(248, 113, 113, 0.3);
  background: rgba(248, 113, 113, 0.12);
  color: rgba(254, 202, 202, 0.9);
}

.section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-title {
  font-weight: 600;
  font-size: 13px;
}

.section-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.mode-switch {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  width: 100%;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  padding: 4px;
  gap: 6px;
}

.seg-item {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  padding: 10px 12px;
  border: none;
  background: transparent;
  color: rgba(209, 213, 219, 0.88);
  font-size: 13px;
  font-weight: 500;
  border-radius: 8px;
  cursor: pointer;
  transition: background 0.2s ease, color 0.2s ease;
}

.seg-item.active {
  background: #2563eb;
  color: #fff;
}

.seg-item:not(.active):hover {
  background: rgba(255, 255, 255, 0.08);
}

.form-grid {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}

.form-field {
  width: 372px;
  max-width: 100%;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-field label {
  font-size: 12px;
  font-weight: 500;
  color: rgba(191, 219, 254, 0.85);
}

input {
  width: 372px;
  box-sizing: border-box;
  max-width: 100%;
  margin: 0 auto;
  padding: 8px 10px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  background: transparent;
  color: inherit;
  font-size: 13px;
  min-width: 0;
}

input:focus {
  outline: none;
  border-color: rgba(37, 99, 235, 0.6);
  background: rgba(37, 99, 235, 0.08);
}

input::placeholder {
  color: rgba(226, 232, 240, 0.6);
}

.actions-row {
  display: flex;
  gap: 10px;
  flex-wrap: nowrap;
  width: 372px;
  max-width: calc(100vw - 48px);
  margin: 0 auto;
  justify-content: flex-start;
}

.action-btn {
  flex: 0 0 calc((372px - 20px) / 3);
  width: calc((372px - 20px) / 3);
  min-width: 0;
  padding: 10px 16px;
  border-radius: 8px;
  border: 1px solid transparent;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s ease, color 0.2s ease, opacity 0.2s ease;
}

.action-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.action-start {
  background: #2563eb;
  color: #fff;
  border-color: rgba(37, 99, 235, 0.9);
}

.action-start:not(:disabled):hover {
  background: #1d4ed8;
  border-color: #1d4ed8;
}

.action-refresh {
  background: rgba(148, 163, 184, 0.25);
  border-color: rgba(148, 163, 184, 0.35);
  color: rgba(226, 232, 240, 0.95);
}

.action-refresh:not(:disabled):hover {
  background: rgba(148, 163, 184, 0.35);
  border-color: rgba(148, 163, 184, 0.45);
}

.action-stop {
  background: rgba(248, 113, 113, 0.22);
  border-color: rgba(248, 113, 113, 0.3);
  color: rgba(254, 226, 226, 0.95);
}

.action-stop:not(:disabled):hover {
  background: rgba(248, 113, 113, 0.32);
  border-color: rgba(248, 113, 113, 0.45);
}

.btn-secondary,
.btn-link {
  border-radius: 6px;
  padding: 7px 16px;
  font-size: 13px;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.2s ease;
}

.btn-secondary {
  background: transparent;
  border-color: rgba(255, 255, 255, 0.2);
  color: rgba(229, 231, 235, 0.85);
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.06);
}

.btn-link {
  background: transparent;
  padding: 4px 10px;
  color: #93c5fd;
}

.btn-link:hover {
  color: #bfdbfe;
}

.btn-link:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.err {
  color: #fca5a5;
  font-size: 11px;
}

.forward-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.forward-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(17, 24, 39, 0.7);
}

.forward-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.line {
  font-size: 12px;
}

.sub {
  font-size: 11px;
  color: rgba(148, 163, 184, 0.85);
}

.empty-card {
  margin: 0 auto;
  width: 372px;
  box-sizing: border-box;
  max-width: calc(100vw - 48px);
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 18px;
  border-radius: 12px;
  border: 1px solid rgba(148, 163, 184, 0.25);
  background: rgba(148, 163, 184, 0.12);
  color: rgba(226, 232, 240, 0.9);
}

.empty-title {
  font-size: 13px;
  font-weight: 600;
}

.empty-sub {
  margin: 0;
  font-size: 12px;
  line-height: 1.5;
  color: rgba(148, 163, 184, 0.88);
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 24px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

@media (max-width: 768px) {
  .form-grid {
    align-items: stretch;
    width: 100%;
  }
  .action-btn {
    flex: 1 1 100%;
    width: 100%;
    min-width: 0;
  }
}
.form-field select {
  width: 372px;
  box-sizing: border-box;
  max-width: 100%;
  margin: 0 auto;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(17, 24, 39, 0.7);
  color: rgba(229, 231, 235, 0.92);
}

/* Inline row for IP + Port inputs within a 372px container */
.inline {
  display: grid;
  grid-template-columns: 1fr 92px;
  gap: 8px;
  width: 372px;
  max-width: 100%;
  margin: 0 auto;
}
.inline input {
  width: 100%;
  box-sizing: border-box;
  margin: 0;
}
</style>












