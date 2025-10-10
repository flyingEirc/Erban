<template>
  <div class="app">
    <!-- Custom Title Bar (draggable) -->
    <div class="window-bar" @dblclick="onToggleMaximise">
      <div class="title-drag">Erban</div>
      <div class="window-controls">
        <button class="win-btn min"  @click.stop="onMinimise"></button>
        <button class="win-btn max" @click.stop="onToggleMaximise"></button>
        <button class="win-btn close" @click.stop="onClose"></button>
      </div>
    </div>
    <div class="layout">
    <!-- Left: SSH instances list -->
    <section class="sidebar">
      <SshList
        :instances="instances"
        :selectedId="selectedId"
        :connectedIds="connectedInstanceIds"
        :connectingIds="connectingInstanceIds"
        @select="onSelect"
        @add="onAddInstance"
        @open-forward="onOpenForward"
        @edit="onEditInstance"
        @delete="onDeleteInstance"
        @connect="onConnectInstance"
      />
    </section>
    <section class="content">
      <div class="tabs">
        <div
          v-for="t in tabs"
          :key="t.tabId"
          class="tab"
          :class="{ active: t.tabId === activeTabId }"
          @click="activateTab(t.tabId)"
        >
          <span class="tab-title">{{ tabTitle(t) }}</span>
          <button class="tab-close" @click.stop="closeTab(t.tabId)">×</button>
        </div>
        <div v-if="tabs.length === 0">双击左侧条目以打开会话</div>
      </div>
      <div class="terminal" :class="{ fill: isMax }">
        <div class="terminal-split">
          <div class="term-main">
            <template v-if="activeTab && (activeTab.status !== 'idle' || activeTab.connected || activeTab.lastError)">
              <KeepAlive>
                <TerminalXterm
                  :key="activeTab.tabId"
                  :sessionId="activeTab.sessionId"
                  :connected="activeTab.connected"
                  :status="activeTab.status"
                  :errorText="activeTab.lastError"
                  :title="tabTitle(activeTab)"
                />
              </KeepAlive>
            </template>
            <div v-else class="brand-right">Erban</div>
          </div>
          <!-- Right: AI Chat panel (25%) -->
          <aside class="chat-panel">
            <div class="chat-header">
              <button class="config-btn" @click="showAiConfig = true" title="AI 配置">
                <svg version="1.1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px" viewBox="0 0 512 512" enable-background="new 0 0 512 512" xml:space="preserve"><path d="M416.3,256c0-21,13.1-38.9,31.7-46.1c-4.9-20.5-13-39.7-23.7-57.1c-6.4,2.8-13.2,4.3-20.1,4.3c-12.6,0-25.2-4.8-34.9-14.4c-14.9-14.9-18.2-36.8-10.2-55C341.8,77,322.5,68.9,302.1,64C295,82.5,277,95.7,256,95.7c-21,0-39-13.2-46.1-31.7c-20.5,4.9-39.7,13-57.1,23.7c8.1,18.1,4.7,40.1-10.2,55c-9.6,9.6-22.3,14.4-34.9,14.4c-6.9,0-13.7-1.4-20.1-4.3C77,170.3,68.9,189.5,64,210c18.5,7.1,31.7,25,31.7,46.1c0,21-13.1,38.9-31.6,46.1c4.9,20.5,13,39.7,23.7,57.1c6.4-2.8,13.2-4.2,20-4.2c12.6,0,25.2,4.8,34.9,14.4c14.8,14.8,18.2,36.8,10.2,54.9c17.4,10.7,36.7,18.8,57.1,23.7c7.1-18.5,25-31.6,46-31.6c21,0,38.9,13.1,46,31.6c20.5-4.9,39.7-13,57.1-23.7c-8-18.1-4.6-40,10.2-54.9c9.6-9.6,22.2-14.4,34.9-14.4c6.8,0,13.7,1.4,20,4.2c10.7-17.4,18.8-36.7,23.7-57.1C429.5,295,416.3,277.1,416.3,256z M256.9,335.9c-44.3,0-80-35.9-80-80c0-44.1,35.7-80,80-80s80,35.9,80,80C336.9,300,301.2,335.9,256.9,335.9z"/></svg>
              </button>
            </div>
            <div class="chat-body" ref="chatBodyRef">
              <div v-if="chatMessages.length === 0" class="chat-placeholder">开启SSH实例后进行对话</div>
              <div v-else class="chat-messages">
                <div
                  v-for="(msg, idx) in chatMessages"
                  :key="idx"
                  class="chat-message"
                  :class="msg.role"
                >
                  <div class="message-role">{{ msg.role === 'user' ? '你' : 'AI' }}</div>
                  <div class="message-content">
                    <template v-if="msg.role === 'assistant' && msg.content === '' && isStreaming">
                      <div class="typing-indicator">
                        <span></span>
                        <span></span>
                        <span></span>
                      </div>
                    </template>
                    <template v-else>
                      {{ msg.content }}
                    </template>
                  </div>
                </div>
              </div>
            </div>
            <form class="chat-input-bar" @submit.prevent="onSendChat">
              <textarea
                ref="chatInputRef"
                v-model="chatText"
                class="chat-input"
                rows="1"
                autocomplete="off"
                placeholder="输入消息…"
                @input="onChatInput"
              />
              <div class="chat-actions">
                <div class="actions-spacer"></div>
                <div class="actions-right">
                  <button
                    v-if="!isStreaming"
                    type="button"
                    class="chat-action-btn send"
                    aria-label="Send"
                    @click="onClickSend"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 16 16" width="16" height="16"><g fill="none"><path d="M7.146 2.146a.5.5 0 0 1 .708 0l3 3a.5.5 0 0 1-.708.708L8 3.707V13.5a.5.5 0 0 1-1 0V3.707L4.854 5.854a.5.5 0 1 1-.708-.708l3-3z" fill="currentColor"></path></g></svg>
                  </button>
                  <button
                    v-else
                    type="button"
                    class="chat-action-btn stop"
                    aria-label="Stop"
                    @click="onClickStop"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 24 24" width="16" height="16"><path d="M6 6h12v12H6V6z" fill="currentColor"></path></svg>
                  </button>
                </div>
              </div>
            </form>
          </aside>
        </div>
      </div>
    </section>
    <!-- Bottom-right resize handle (frameless window) -->
    <div class="resize-handle br" aria-hidden="true"></div>
    <!-- Add Instance Modal -->
    <AddSshModal
      v-if="showAdd"
      :initial="initialForModal || undefined"
      @close="() => { showAdd = false; editingId = null }"
      @create="onCreateInstance"
    />
    <!-- Port Forward Modal -->
    <PortForwardModal
      v-if="showForward"
      :instances="instances"
      :active="!!(activeTab && activeTab.status==='connected')"
      :sessionId="activeTab ? activeTab.sessionId : ''"
      :activeInstanceId="activeTab ? activeTab.instanceId : ''"
      @close="() => { showForward = false }"
    />
    <!-- AI Config Modal -->
    <AiConfigModal
      v-if="showAiConfig"
      :initialConfig="loadAiConfigFromStorage()"
      @close="() => { showAiConfig = false }"
      @save="onSaveAiConfig"
    />
    </div>
  </div>

</template>
<script setup lang="ts">
import { computed, reactive, ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { EventsOn, WindowMinimise, WindowToggleMaximise, Quit } from '../wailsjs/runtime/runtime'
import { stopAllForwards } from './services/portforward'
import * as ChatBridge from '../wailsjs/go/main/ChatBridge'
import SshList from './components/SshList.vue'
import TerminalXterm from './components/TerminalXterm.vue'
import AddSshModal from './components/AddSshModal.vue'
import PortForwardModal from './components/PortForwardModal.vue'
import AiConfigModal from './components/AiConfigModal.vue'
type SSHInstance = {
  id: string
  label: string
  host: string
  user: string
  password?: string
  pemId?: string
  pemDataB64?: string
  proxy?: string
}
const instances = reactive<SSHInstance[]>([])
const selectedId = ref<string>('')
type Tab = {
  tabId: string
  sessionId: string
  instanceId: string
  status: 'idle' | 'connecting' | 'connected'
  connected: boolean
  lastError: string
}
const tabs = reactive<Tab[]>([])
const sessionListeners = new Map<string, () => void>()
const activeTabId = ref<string>('')
const activeTab = computed(() => tabs.find(t => t.tabId === activeTabId.value) || null)
const connectedInstanceIds = computed(() => Array.from(new Set(tabs.filter(t => t.status === 'connected').map(t => t.instanceId))))
const connectingInstanceIds = computed(() => Array.from(new Set(tabs.filter(t => t.status === 'connecting').map(t => t.instanceId))))
function tabTitle(t: Tab) {
  const i = instances.find(x => x.id === t.instanceId)
  return (i?.label || i?.host || 'Erban')
}
const showAdd = ref(false)
const showForward = ref(false)
const showAiConfig = ref(false)
const editingId = ref<string | null>(null)
const currentInstance = computed(() => instances.find(i => i.id === selectedId.value))
function onSelect(id: string) {
  selectedId.value = id
}
function onAddInstance() { editingId.value = null; showAdd.value = true }
function onOpenForward() { showForward.value = true }
function onEditInstance(id: string) {
  if (activeTab.value && activeTab.value.status !== 'idle') return
  editingId.value = id; showAdd.value = true
}
function onDeleteInstance(id: string) {
  if (activeTab.value && activeTab.value.status !== 'idle') return
  const idx = instances.findIndex(i => i.id === id)
  if (idx >= 0) {
    instances.splice(idx, 1)
    if (selectedId.value === id) {
      selectedId.value = instances[0]?.id || ''
    }
  }
}
function onCreateInstance(payload: { label: string; host: string; user: string; auth: 'password'|'key'; password?: string; pemId?: string; pemDataB64?: string; proxy?: string }) {
  if (editingId.value) {
    const idx = instances.findIndex(i => i.id === editingId.value)
    if (idx >= 0) {
      const base: any = { ...instances[idx], label: payload.label, host: payload.host, user: payload.user }
      delete base.password
      delete base.pemDataB64
      if (payload.auth === 'password') base.password = payload.password || ''
      if (payload.auth === 'key') { base.pemId = payload.pemId || ''; base.pemDataB64 = payload.pemDataB64 || '' }
      base.proxy = payload.proxy || ''
      instances[idx] = base
      selectedId.value = base.id
    }
    editingId.value = null
  } else {
    const id = Date.now().toString()
    const base: any = { id, label: payload.label, host: payload.host, user: payload.user }
    if (payload.auth === 'password') base.password = payload.password || ''
    if (payload.auth === 'key') { base.pemId = payload.pemId || ''; base.pemDataB64 = payload.pemDataB64 || '' }
    if (payload.proxy) base.proxy = payload.proxy
    instances.push(base)
    selectedId.value = id
  }
  showAdd.value = false
}
function bridge() { return (window as any).go?.main?.SSHBridge }
function ensureSessionListener(t: Tab) {
  if (sessionListeners.has(t.sessionId)) return
  const off = EventsOn(`ssh:ended:${t.sessionId}`, () => {
    t.connected = false
    t.status = 'idle'
    if (!t.lastError) {
      t.lastError = 'Session ended'
    }
    // Auto-close tab after 1 second
    setTimeout(() => {
      closeTab(t.tabId)
    }, 250)
  })
  sessionListeners.set(t.sessionId, off)
}
async function connectForTab(t: Tab) {
  if (t.connected || t.status === 'connecting' || t.status === 'connected') return
  const inst = instances.find(i => i.id === t.instanceId)
  const b = bridge()
  if (!inst || !b) return
  try {
    ensureSessionListener(t)
    t.status = 'connecting'
    t.lastError = ''
    const sessionId = t.sessionId
    if (inst.pemDataB64 && inst.pemDataB64.length > 0) {
      await b.InitWithPem(sessionId, inst.host, inst.user, inst.pemDataB64)
    } else if (inst.pemId && b.KeyGet) {
      const k = await b.KeyGet(inst.pemId)
      if (k && k.length > 0) {
        await b.InitWithPem(sessionId, inst.host, inst.user, k)
      } else {
        await b.InitWithPasswd(sessionId, inst.host, inst.user, inst.password || '')
      }
    } else {
      await b.InitWithPasswd(sessionId, inst.host, inst.user, inst.password || '')
    }
    if (inst.proxy && inst.proxy.trim().length > 0) {
      const err = await b.SetProxy(sessionId, inst.proxy)
      if (err) console.error('SetProxy error:', err)
    }
    const cerr = await b.Connect(sessionId)
    if (cerr) {
      console.error('Connect error:', cerr)
      t.lastError = cerr
      t.status = 'idle'
      t.connected = false
    } else {
      t.connected = true
      t.status = 'connected'
    }
  } catch (e: any) {
    console.error(e)
    t.lastError = (e && e.message) ? e.message : String(e)
    t.status = 'idle'
    t.connected = false
  }
}
async function closeBackend(target?: Tab | null) {
  const t = target ?? activeTab.value
  if (!t) return
  const b = bridge()
  if (!b?.Close) return
  try {
    await b.Close(t.sessionId)
  } catch (err) {
    console.warn('Close error:', err)
  }
  t.connected = false
  t.status = 'idle'
}

// Close all port-forward connections and all SSH sessions
async function closeall() {
  try {
    // First, stop all port forwards for every session
    const sessionIds = Array.from(new Set(tabs.map(t => t.sessionId)))
    await Promise.all(sessionIds.map(async (sid) => {
      try { await stopAllForwards(sid) } catch (e) { /* ignore */ }
    }))

    // Then close each SSHBridge session
    for (const t of [...tabs]) {
      await closeBackend(t)
    }

    // Detach all event listeners
    sessionListeners.forEach(off => off())
    sessionListeners.clear()
  } catch (e) {
    console.warn('closeall error:', e)
  }
}
function openTab(instanceId: string) {
  const sessionId = `sess_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`
  const t: Tab = { tabId: sessionId, sessionId, instanceId, status: 'idle', connected: false, lastError: '' }
  ensureSessionListener(t)
  tabs.push(t)
  activateTab(t.tabId)
}
async function activateTab(tabId: string) {
  if (activeTabId.value === tabId) return
  activeTabId.value = tabId
  const t = tabs.find(x => x.tabId === tabId)
  if (t) {
    await connectForTab(t)
}
}
async function closeTab(tabId: string) { 
  const idx = tabs.findIndex(t => t.tabId === tabId)
  if (idx < 0) return
  const tab = tabs[idx]
  const wasActive = activeTabId.value === tabId
  let next: Tab | null = null
  if (wasActive) {
    next = tabs[idx - 1] || tabs[idx + 1] || null
    activeTabId.value = next?.tabId || ''
  }

  const off = sessionListeners.get(tab.sessionId)
  if (off) {
    off()
    sessionListeners.delete(tab.sessionId)
  }
  const chatOff = chatListeners.get(tab.sessionId)
  if (chatOff) {
    chatOff()
    chatListeners.delete(tab.sessionId)
  }

  await closeBackend(tab)
  tabs.splice(idx, 1)
  if (wasActive && next) {
    await connectForTab(next)
  }
}
function onConnectInstance(id: string) {
  // Double-click opens a new tab and activates it
  selectedId.value = id
  openTab(id)
}
// ---- Persistence (localStorage) ----
const LS_KEY = 'ssh:instances'
function loadInstances() {
  try {
    const raw = localStorage.getItem(LS_KEY)
    if (raw) {
      const data = JSON.parse(raw) as SSHInstance[]
      instances.splice(0, instances.length, ...data)
      if (instances.length > 0) selectedId.value = instances[0].id
    } else {
      // Keep empty by default as requested
      selectedId.value = ''
    }
  } catch (e) {
    console.warn('loadInstances error', e)
  }
}
function saveInstances() {
  try {
    // Avoid persisting private key material into localStorage
    const sanitized = instances.map(i => {
      const { pemDataB64, ...rest } = i as any
      return rest
    })
    localStorage.setItem(LS_KEY, JSON.stringify(sanitized))
  } catch {}
}
watch(() => JSON.stringify(instances), saveInstances)
// ---- Backend events ----
onMounted(() => {
  loadInstances()
  // Auto-apply saved AI config globally
  const saved = loadAiConfigFromStorage()
  if (saved) {
    applyAiConfig(saved).catch(() => {})
  }
})
onUnmounted(() => {
  sessionListeners.forEach(off => off())
  sessionListeners.clear()
  chatListeners.forEach(off => off())
  chatListeners.clear()
})
// Prefill for modal when editing
const initialForModal = computed(() => {
  if (!editingId.value) return null
  const i = instances.find(x => x.id === editingId.value)
  if (!i) return null
  return {
    label: i.label,
    host: i.host,
    user: i.user,
    auth: ((i.pemId && i.pemId.trim().length > 0)) ? 'key' as const : 'password' as const,
    password: i.password || '',
    proxy: i.proxy || ''
  }
})
// ----- Window controls -----
function onMinimise() { try { WindowMinimise() } catch {} }
const isMax = ref(false)
function onToggleMaximise() { try { WindowToggleMaximise(); isMax.value = !isMax.value } catch {} }
async function onClose() {
  try { await closeall() } catch {}
  try { Quit() } catch {}
}

// --- AI Chat ---
const chatText = ref('')
const chatInputRef = ref<HTMLTextAreaElement | null>(null)
const chatBodyRef = ref<HTMLDivElement | null>(null)
const MAX_CHAT_ROWS = 6
const isStreaming = ref(false) // 控制按钮状态（false: 显示发送；true: 显示中断）

type ChatMessage = {
  role: 'user' | 'assistant'
  content: string
}
const chatMessages = reactive<ChatMessage[]>([])
const chatListeners = new Map<string, () => void>()
function resizeChatInput() {
  const el = chatInputRef.value
  if (!el) return
  const cs = window.getComputedStyle(el)
  const lineH = parseFloat(cs.lineHeight || '0') || 20
  const padTop = parseFloat(cs.paddingTop || '0') || 0
  const padBottom = parseFloat(cs.paddingBottom || '0') || 0
  const borderTop = parseFloat(cs.borderTopWidth || '0') || 0
  const borderBottom = parseFloat(cs.borderBottomWidth || '0') || 0
  const maxH = lineH * MAX_CHAT_ROWS + padTop + padBottom + borderTop + borderBottom
  const minH = parseFloat(cs.minHeight || '0') || 0

  // Reset height to compute new scrollHeight, then clamp
  el.style.height = 'auto'
  const needed = Math.max(el.scrollHeight, minH)
  if (needed > maxH) {
    el.style.overflowY = 'auto'
    el.style.height = maxH + 'px'
  } else {
    el.style.overflowY = 'hidden'
    el.style.height = needed + 'px'
  }
}
function onChatInput() {
  resizeChatInput()
}

function scrollChatToBottom() {
  const el = chatBodyRef.value
  if (!el) return
  setTimeout(() => {
    el.scrollTop = el.scrollHeight
  }, 0)
}

function onSendChat() {
  onClickSend()
}

async function onClickSend() {
  const text = chatText.value.trim()
  if (!text || isStreaming.value) return

  if (!activeTab.value) {
    console.warn('[AI Chat] activeTab not available')
    return
  }

  const sessionId = activeTab.value.sessionId

  // 添加用户消息到对话
  chatMessages.push({ role: 'user', content: text })
  chatText.value = ''
  await nextTick()
  resizeChatInput()
  scrollChatToBottom()

  // 创建空的助手消息用于流式更新
  const assistantMsgIndex = chatMessages.length
  chatMessages.push({ role: 'assistant', content: '' })

  // 设置事件监听器
  ensureChatListener(sessionId, assistantMsgIndex)

  // 开始流式响应
  isStreaming.value = true
  try {
    const err = await ChatBridge.Start(sessionId, text)
    if (err) {
      console.error('[AI Chat] Start error:', err)
      chatMessages[assistantMsgIndex].content = '502 error'
      isStreaming.value = false
    }
  } catch (e: any) {
    console.error('[AI Chat] Start exception:', e)
    chatMessages[assistantMsgIndex].content = '502 error'
    isStreaming.value = false
  }
}

async function onClickStop() {
  if (!isStreaming.value) return

  if (!activeTab.value) return

  const sessionId = activeTab.value.sessionId

  try {
    await ChatBridge.Cancel(sessionId)
    console.log('[AI Chat] Cancelled')
  } catch (e) {
    console.error('[AI Chat] Cancel exception:', e)
  }

  isStreaming.value = false
}

function ensureChatListener(sessionId: string, messageIndex: number) {
  if (chatListeners.has(sessionId)) return

  const outputListener = EventsOn(`chat:output:${sessionId}`, (data: string) => {
    if (messageIndex < chatMessages.length) {
      chatMessages[messageIndex].content += data
      scrollChatToBottom()
    }
  })

  const endedListener = EventsOn(`chat:ended:${sessionId}`, () => {
    isStreaming.value = false
    // 如果流结束但消息仍为空，显示502 error
    if (messageIndex < chatMessages.length && chatMessages[messageIndex].content === '') {
      chatMessages[messageIndex].content = '502 error'
    }
    // 清理监听器
    const cleanup = chatListeners.get(sessionId)
    if (cleanup) {
      cleanup()
      chatListeners.delete(sessionId)
    }
  })

  // 合并清理函数
  chatListeners.set(sessionId, () => {
    outputListener()
    endedListener()
  })
}
onMounted(() => {
  resizeChatInput()
})

// --- AI Config (Global) ---
type AiConfig = {
  proxy: string
  model: string
  reason: string
  provider: string
  apikey: string
  baseurl: string
}

const AI_CONFIG_KEY = 'ai:config'
const aiConfigured = ref(false)

function saveAiConfigToStorage(config: AiConfig) {
  try {
    localStorage.setItem(AI_CONFIG_KEY, JSON.stringify(config))
  } catch (e) {
    console.warn('Failed to save AI config:', e)
  }
}

function loadAiConfigFromStorage(): AiConfig | null {
  try {
    const raw = localStorage.getItem(AI_CONFIG_KEY)
    if (raw) {
      const config = JSON.parse(raw)
      // Ensure all fields have default values for backward compatibility
      return {
        proxy: config.proxy || '',
        model: config.model || 'gpt-4o',
        reason: config.reason || 'medium',
        provider: config.provider || 'openai',
        apikey: config.apikey || '',
        baseurl: config.baseurl || ''
      }
    }
  } catch (e) {
    console.warn('Failed to load AI config:', e)
  }
  return null
}

async function applyAiConfig(config: AiConfig) {
  try {
    const err = await ChatBridge.OpenAI(config.proxy, config.model, config.reason, config.provider, config.apikey, config.baseurl)
    if (err) {
      console.error('[AI Config] Error:', err)
      return false
    } else {
      console.log('[AI Config] Applied globally')
      aiConfigured.value = true
      return true
    }
  } catch (e) {
    console.error('[AI Config] Exception:', e)
    return false
  }
}

async function onSaveAiConfig(config: AiConfig) {
  const success = await applyAiConfig(config)

  // Persist to localStorage if successful
  if (success) {
    saveAiConfigToStorage(config)
  }
}
</script>
<style scoped>
.app { height: 100vh; display: flex; flex-direction: column;}
.window-bar {
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 8px;
  background: #0f172a;
  color: #e5e7eb;
  border-bottom: 1px solid rgba(255,255,255,0.08);
  --wails-draggable: drag; 
}
.title-drag { font-size: 12px; opacity: 0.8; }
.window-controls { display: flex; gap: 10px; padding-right: 2px;}
.win-btn {
  width: 36px; height: 28px;
  display: grid; place-items: center;
  background: transparent; color: #e5e7eb;
  border: none;
  cursor: pointer;
  position: relative;
}
.win-btn::before,
.win-btn::after { content: ""; position: absolute; background: #e5e7eb; opacity: .9; }
/* Minimise: a thin horizontal line */
.win-btn.min::before { width: 14px; height: 2px; border-radius: 1px; }
/* Maximise: a small hollow square */
.win-btn.max::before {
  width: 12px; height: 12px; background: transparent; border: 2px solid #e5e7eb; border-radius: 2px;
}
/* Close: an X made of two diagonal lines */
.win-btn.close::before { width: 14px; height: 2px; transform: rotate(45deg); border-radius: 1px; }
.win-btn.close::after  { width: 14px; height: 2px; transform: rotate(-45deg); border-radius: 1px; }
.win-btn:hover { background: rgba(255,255,255,0.06); }
.win-btn.close:hover { background: rgba(239, 68, 68, 0.25); }
.layout {
  display: grid;
  grid-template-columns: 280px 1fr;
  grid-template-rows: calc(100vh - 32px);
  height: calc(100vh - 32px);
  overflow: hidden;
}
.sidebar {
  border-right: 1px solid rgba(255,255,255,0.08);
  background: rgba(0,0,0,0.2);
  padding: 12px;
}
.content {
  display: grid;
  grid-template-rows: auto 1fr;
  height: 100%;
  font-family: 'Source Code Pro','SourceCodePro',Consolas,'Courier New',monospace;
  font-weight: 500;
}
.brand-right { height: 100%; display: grid; place-items: center; font-weight: 700; font-size: 22px; color: rgba(229,231,235,0.8); letter-spacing: 0.5px; }
.terminal {
  min-height: 0;
  overflow: hidden;
  padding-left: 0; /* flush with SshList */
  box-sizing: border-box;
  /* Terminal area background */
  background: rgba(27, 38, 54, 1);
}
/* Split the terminal area: 75% terminal, 25% chat */
.terminal-split { display: grid; grid-template-columns: 3.5fr 1.5fr; height: 100%; min-height: 0; width: 100%; gap: 0; }
.term-main { min-width: 0; min-height: 0; overflow: hidden; }
.chat-panel { min-width: 0; min-height: 0; display: flex; flex-direction: column; background: rgba(0,0,0,0.18); border-left: 1px solid rgba(255,255,255,0.08); }
.chat-header {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 8px 10px;
  border-bottom: 1px solid rgba(255,255,255,0.05);
}
.config-btn {
  padding: 4px 8px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  color: #9ca3af;
  cursor: pointer;
  border-radius: 6px;
  transition: background 0.2s;
}
.config-btn svg {
  width: 16px;
  height: 16px;
  display: block;
  fill: currentColor;
}
.config-btn:hover { background: rgba(255,255,255,0.08); color: #e5e7eb; }
.chat-body {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  padding: 10px;
  /* Reserve space to avoid layout shift */
  scrollbar-gutter: stable;
  /* Firefox: keep thin width; hide via transparent colors */
  scrollbar-width: thin;
  scrollbar-color: transparent transparent;
}
/* Firefox reveal on hover via color only */
.chat-body:hover {
  scrollbar-color: #6c6765 #110d0a;
}
/* WebKit: keep size constant */
.chat-body::-webkit-scrollbar {
  width: 10px;
  height: 10px;
}
.chat-body::-webkit-scrollbar-track {
  background: transparent;
  border-radius: 6px;
}
.chat-body::-webkit-scrollbar-thumb {
  background-color: transparent;
  border-radius: 6px;
  border: 2px solid transparent;
}
.chat-body:hover::-webkit-scrollbar-track {
  background: #000000;
}
.chat-body:hover::-webkit-scrollbar-thumb {
  background-color: #000000;
  border-color: #e3946c;
}
.chat-body:hover::-webkit-scrollbar-thumb:hover {
  background-color: #5b5552;
}
.chat-placeholder { opacity: .6; font-size: 13px; color: #e5e7eb; }

.chat-messages {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.chat-message {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 10px;
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.5;
  font-family: 'Source Code Pro','SourceCodePro',Consolas,'Courier New',monospace;
}

.chat-message.user {
  background: rgba(167, 167, 167, 0.15);
  align-self: flex-end;
  margin-left: auto;
  max-width: 80%;
  text-align: left;
}

.chat-message.assistant {
  align-self: flex-start;
  margin-right: auto;
  text-align: left;
}

.message-role {
  font-size: 11px;
  font-weight: 600;
  opacity: 0.7;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.message-content {
  color: #e5e7eb;
  white-space: pre-wrap;
  word-wrap: break-word;
}

/* Typing indicator animation */
.typing-indicator {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 0;
}

.typing-indicator span {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background-color: #9ca3af;
  opacity: 0.6;
  animation: typing-bounce 1.4s infinite ease-in-out;
}

.typing-indicator span:nth-child(1) {
  animation-delay: 0s;
}

.typing-indicator span:nth-child(2) {
  animation-delay: 0.2s;
}

.typing-indicator span:nth-child(3) {
  animation-delay: 0.4s;
}

@keyframes typing-bounce {
  0%, 60%, 100% {
    transform: translateY(0);
    opacity: 0.6;
  }
  30% {
    transform: translateY(-10px);
    opacity: 1;
  }
}
.chat-input-bar {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-left: 6px; 
  margin-right: 6px;
  margin-bottom: 6px;
  background: rgba(15, 15, 27, 0.897); /* unify background with input */
  border-radius: 10px;             /* rounded parent container */
}
.chat-input {
  display: block;
  width: 100%;
  min-height: 36px;
  height: auto; /* controlled via JS scrollHeight */
  box-sizing: border-box;
  padding: 8px 10px; /* no overlay button; standard padding */
  border-radius: 6px;
  border: none;              /* remove border */
  background: transparent;         /* inherit parent background */
  color: #e5e7eb;
  outline: none;
  line-height: 1.35;
  resize: none;            /* user can't drag handle; we auto-size */
  overflow: hidden;        /* hide scrollbar while auto-resizing */
  white-space: pre-wrap;   /* keep newlines, wrap long lines */
  overflow-wrap: anywhere; /* break long words/URLs */
}
.chat-input:focus { box-shadow: none; }
/* Button bar under textarea */
.chat-actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; }
.chat-action-btn {
  width: 30px;
  height: 30px;
  margin-right: 5px;
  margin-bottom: 5px;
  border-radius: 50%;
  border: 1px solid #d1d5db;         /* solid grey border */
  background: #e5e7eb;                /* solid grey-white fill */
  color: #050608;                     /* dark icon color */
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  outline: none;
}
.chat-action-btn:hover { background: #f3f4f6; }
.chat-action-btn:active { background: #d1d5db; }
.chat-action-btn .icon { font-size: 14px; line-height: 1; opacity: 1; }
.chat-action-btn svg { display: block; }
.chat-action-btn.stop { color: #111827; border-color: #d1d5db; background: #e5e7eb; }
/* Chat input (textarea) when overflowing */
.chat-input { scrollbar-width: none; scrollbar-color: #6c6765 #110d0a; }
.chat-input:hover { scrollbar-width: thin; }
.chat-input::-webkit-scrollbar { width: 0px; height: 0px; }
.chat-input:hover::-webkit-scrollbar { width: 10px; height: 10px; }
.chat-input:hover::-webkit-scrollbar-track { background: #000000; border-radius: 6px; }
.chat-input:hover::-webkit-scrollbar-thumb { background-color: #000000; border-radius: 6px; border: 2px solid #e3946c; }
.chat-input:hover::-webkit-scrollbar-thumb:hover { background-color: #5b5552; }
/* When maximised, let terminal fill the available space */
.terminal.fill { display: grid; }
.terminal.fill :deep(.termx-wrap) { width: 100% !important; height: 100% !important; }
.terminal.fill :deep(.termx-body) { width: 100% !important; height: 100% !important; }
/* 容器：不固定高度，底部分隔线统一在这里画 */
.tabs {
  display: flex; align-items: center; gap: 8px;
  overflow-x: auto;
  background: transparent;
  position: relative;
  min-height: 30px;
}
.tabs > div {
  padding: 5px 10px;
}
/* pill */
.tab {
  position: relative;
  display: inline-flex; align-items: center; gap: 8px;
  max-width: 260px;
  padding: 5px 10px;
  /* Rounded rectangle: all corners rounded, straight edges */
  border-radius: 8px;
  background: rgba(105, 126, 119, 0.35);
  color: rgba(143, 159, 189, 0.9);
  cursor: pointer; user-select: none;
  backdrop-filter: saturate(120%);
  -webkit-backdrop-filter: saturate(120%);
  border: 1px solid transparent;
  z-index: 1;
}
.tab:hover { background: rgba(105,126,119,0.45); }


.tab.active {
  background: rgba(16,185,129,0.16);        
  color: #a7f3d0;
  border-color: rgba(16,185,129,0.35);
  box-shadow: 0 -1px 0 rgba(16,185,129,0.16), inset 0 0 0 1px rgba(16,185,129,0.35);
  z-index: 2;
}

/* 标题与关闭按钮 */
.tab-title { font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.tab-close {
  order: -1;
  background: transparent; border: none; color: currentColor;
  opacity: .85; cursor: pointer;
  padding: 0 4px; font-size: 14px; line-height: 1;
  min-width: 18px; height: 18px; display: inline-flex; align-items: center; justify-content: center;
}
.tab-close:hover { opacity: 1; background: rgba(255,255,255,.06); border-radius: 4px; }


/* Frameless window resize handle: bottom-right */
.resize-handle.br {
  position: fixed;
  right: 0;
  bottom: 0;
  width: 16px;
  height: 16px;
  /* Wails hint for hit testing */
  --wails-resize: bottom-right;
  cursor: nwse-resize;
  /* keep above content & invisible */
  z-index: 9999;
  background: transparent;
}
</style>
