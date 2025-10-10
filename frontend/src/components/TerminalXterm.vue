<template>
  <div class="termx-wrap">
    <div class="terminal-area">
      <div ref="container" class="termx-body" aria-live="polite" aria-label="Terminal"></div>
      <!-- Connecting overlay -->
      <div v-if="(!props.connected) && status === 'connecting'" class="overlay info">
        <div class="overlay-center">
          <div class="spinner" aria-hidden="true"></div>
          <div class="msg">connecting.....</div>
        </div>
      </div>
      <!-- Error overlay -->
      <div v-if="errorText" class="overlay error">{{ errorText }}</div>
    </div>

    <!-- SFTP Area -->
    <div class="sftp-area">
      <!-- SFTP Connecting overlay -->
      <div v-if="sftpStatus === 'connecting'" class="sftp-overlay">
        <div class="spinner" aria-hidden="true"></div>
        <div class="msg">Connecting SFTP...</div>
      </div>
      <!-- SFTP Error -->
      <div v-else-if="sftpError" class="sftp-error">
        {{ sftpError }}
      </div>
      <!-- SFTP Content -->
      <div v-else-if="sftpStatus === 'connected'" class="sftp-content">
        <div class="sftp-header">
          
        </div>
        <div class="sftp-list">
          <div v-if="sftpEntries.length === 0" class="empty-message">Empty directory</div>
          <div
            v-for="entry in sftpEntries"
            :key="entry.name"
            class="sftp-entry"
            :class="{ 'is-dir': entry.isDir }"
            @dblclick="handleEntryDoubleClick(entry)"
          >
            <span class="entry-icon">{{ entry.isDir ? '📁' : '📄' }}</span>
            <span class="entry-name">{{ entry.name }}</span>
            <span class="entry-size">{{ formatSize(entry.size) }}</span>
          </div>
        </div>
      </div>
      <div v-else class="sftp-idle">
        <span>SFTP not initialized</span>
      </div>
    </div>
  </div>

</template>
<script setup lang="ts">
import { onMounted, onUnmounted, ref, nextTick, watch, onActivated, computed } from 'vue'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { Terminal } from 'xterm'
import type { ITheme } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import 'xterm/css/xterm.css'

// SFTP Entry interface
interface SFTPEntry {
  name: string
  size: number
  mode: string
  modTime: string
  isDir: boolean
}

const props = defineProps<{ sessionId: string; title?: string; connected?: boolean; status?: 'idle'|'connecting'|'connected'; errorText?: string }>()
const status = computed(() => props.status)
const errorText = computed(() => props.errorText)
const container = ref<HTMLElement | null>(null)

// Terminal state
let term: Terminal | null = null
let fit: FitAddon | null = null
let offOutput: (() => void) | null = null
let offEnded: (() => void) | null = null
let ro: ResizeObserver | null = null
let rafId: number | null = null
let resizeTimer: any = null
let lastSize = { w: -1, h: -1 }
let onWinResize: (() => void) | null = null

// SFTP state
const sftpStatus = ref<'idle' | 'connecting' | 'connected'>('idle')
const sftpError = ref<string>('')
const sftpEntries = ref<SFTPEntry[]>([])
const currentPath = ref<string>('~')

function bridge() {
  return (window as any).go?.main?.SSHBridge
}

// Format file size
function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i]
}

// Load SFTP directory listing
async function loadSFTPList(path: string = '.') {
  const b = bridge()
  const sessionId = props.sessionId
  if (!b || !sessionId) return

  sftpStatus.value = 'connecting'
  sftpError.value = ''

  try {
    const result = await b.SFTPList(sessionId, path)

    if (result.error) {
      sftpError.value = result.error
      sftpStatus.value = 'idle'
    } else {
      sftpEntries.value = result.entries || []
      sftpStatus.value = 'connected'
      currentPath.value = path
    }
  } catch (err: any) {
    sftpError.value = err.message || 'Failed to load SFTP list'
    sftpStatus.value = 'idle'
  }
}

// Handle double-click on entry (navigate into directory)
function handleEntryDoubleClick(entry: SFTPEntry) {
  if (entry.isDir) {
    const newPath = currentPath.value === '.' || currentPath.value === '~'
      ? entry.name
      : `${currentPath.value}/${entry.name}`
    loadSFTPList(newPath)
  }
}

function refreshSessionEvents(sessionId: string | undefined) {
  if (offOutput) { offOutput(); offOutput = null }
  if (offEnded) { offEnded(); offEnded = null }
  if (!sessionId) return
  offOutput = EventsOn(`ssh:output:${sessionId}`, (chunk: string) => {
    // Always follow bottom after each write
    if (!term) return
    term.write(chunk, () => {
      try { term?.scrollToBottom() } catch {}
    })
  })
  offEnded = EventsOn(`ssh:ended:${sessionId}`, () => {
    term?.writeln('\r\n[Session ended]')
  })
}

function applyResize() {
  if (!term) return
  // Fit terminal to the container size (both width and height)
  try {
    fit?.fit() 
    } catch {}
  // Reflect to backend
  const b = bridge()
  const sessionId = props.sessionId
  if (b && sessionId) b.Resize(sessionId, term.rows, term.cols)
}

function scheduleFit(immediate = false) {
  if (rafId) { cancelAnimationFrame(rafId); rafId = null }
  if (resizeTimer) { clearTimeout(resizeTimer); resizeTimer = null }
  const run = () => {
    const el = container.value
    if (!el) return
    const { width, height } = el.getBoundingClientRect()
    if (width === 0 || height === 0) {
      // If hidden/collapsed, retry on next frame
      rafId = requestAnimationFrame(run)
      return
    }
    // Skip if size unchanged to avoid redundant work
    if (Math.round(width) === lastSize.w && Math.round(height) === lastSize.h) {
      return
    }
    lastSize = { w: Math.round(width), h: Math.round(height) }
    applyResize()
  }
  if (immediate) {
    // Next frame ensures layout has settled after container resize
    rafId = requestAnimationFrame(run)
  } else {
    resizeTimer = setTimeout(run, 50)
  }
}

watch(() => props.sessionId, (sid, prev) => {
  if (sid === prev) return
  refreshSessionEvents(sid)
  if (term) {
    term.reset()
    nextTick(() => scheduleFit(true))
  }
}, { immediate: true })

onMounted(() => {
  term = new Terminal({
    fontSize: 13,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Sarasa Mono SC", "Noto Sans Mono CJK SC", monospace',
    cursorBlink: true,
    lineHeight: 1.0,
    scrollback: 1000,
    convertEol: true,
    scrollOnUserInput: true,
    // Black background for terminal
    theme: { background: '#000000' },
  })
  fit = new FitAddon()
  term.loadAddon(fit)
  if (container.value) {
    term.open(container.value)
  }
  // Initial fit to set columns from width
  try { fit.fit() } catch {}
  // Force wraparound mode explicitly (DECAWM)
  try { term.write('\u001b[?7h') } catch {}
  term.onData((data: string) => {
    const b = bridge()
    const sessionId = props.sessionId
    if (b && sessionId) b.Write(sessionId, data)
  })
  nextTick(() => scheduleFit(true))
  // Window resize (maximise/restore) fallback
  onWinResize = () => scheduleFit(true)
  window.addEventListener('resize', onWinResize)
  // Observe container size changes (e.g. sidebar toggle, grid changes)
  if ('ResizeObserver' in window && container.value) {
    ro = new ResizeObserver(() => scheduleFit(true))
    ro.observe(container.value)
  }
})

onUnmounted(() => {
  if (offOutput) offOutput()
  if (offEnded) offEnded()
  if (onWinResize) { window.removeEventListener('resize', onWinResize); onWinResize = null }
  if (ro) { try { ro.disconnect() } catch {} ; ro = null }
  if (rafId) { cancelAnimationFrame(rafId); rafId = null }
  if (resizeTimer) { clearTimeout(resizeTimer); resizeTimer = null }
  term?.dispose()
  term = null
  fit = null
})

watch(() => props.connected, (isConnected) => {
  if (term) {
    const newTheme: ITheme = { background: '#000000' }
    term.options.theme = newTheme
  }
  nextTick(() => scheduleFit(true))

  // Load SFTP list when connected
  if (isConnected && props.sessionId) {
    loadSFTPList('.')
  } else {
    // Reset SFTP state when disconnected
    sftpStatus.value = 'idle'
    sftpError.value = ''
    sftpEntries.value = []
    currentPath.value = '~'
  }
})

onActivated(() => {
  refreshSessionEvents(props.sessionId)
  nextTick(() => scheduleFit(true))
})
</script>
<style scoped>
/* Fixed wrapper - split into terminal (2/3) and sftp (1/3) areas */
.termx-wrap {
  width: 100%;
  height: 100%;
  min-height: 0;
  margin: 0;
  display: grid;
  grid-template-rows: 2fr 1fr;
  gap: 0;
  position: relative;
}

/* Terminal area takes 2/3 of height */
.terminal-area {
  width: 100%;
  height: 100%;
  min-height: 0;
  position: relative;
  display: grid;
  grid-template-rows: minmax(0, 1fr);
}

.termx-body {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-height: 0;
  position: relative;
} 
.termx-header { display: flex; justify-content: space-between; padding: 8px 12px; border-bottom: 1px solid rgba(255,255,255,0.08); }
.status { opacity: 0.7; font-size: 12px; }
.status.on { color: #22c55e; opacity: 1; }

/* SFTP area takes 1/3 of height */
.sftp-area {
  width: 100%;
  height: 100%;
  min-height: 0;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(0, 0, 0, 0.3);
  position: relative;
  overflow: hidden;
}

.sftp-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  background: rgba(0, 0, 0, 0.5);
  color: #e5e7eb;
  z-index: 10;
}

.sftp-error {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(239, 68, 68, 0.2);
  color: #fecaca;
  font-size: 13px;
  padding: 20px;
  text-align: center;
}

.sftp-idle {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(255, 255, 255, 0.4);
  font-size: 13px;
}

.sftp-content {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.sftp-header {
  padding: 8px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(0, 0, 0, 0.2);
}

.sftp-title {
  color: rgba(255, 255, 255, 0.8);
  font-size: 13px;
  font-weight: 500;
}

.sftp-list {
  flex: 1;
  overflow-y: auto;
  padding: 4px;
  /* Scrollbar styling matching xterm-viewport */
  scrollbar-gutter: stable;
  /* Firefox: keep thin width; hide via transparent colors */
  scrollbar-width: thin;
  scrollbar-color: transparent transparent;
}

/* Firefox reveal on hover via color only */
.sftp-list:hover {
  scrollbar-color: #6c6765 #110d0a;
}

/* WebKit: keep size constant */
.sftp-list::-webkit-scrollbar {
  width: 10px;
  height: 10px;
}

.sftp-list::-webkit-scrollbar-track {
  background: transparent;
  border-radius: 6px;
}

.sftp-list::-webkit-scrollbar-thumb {
  background-color: transparent;
  border-radius: 6px;
  border: 2px solid transparent;
}

.sftp-list:hover::-webkit-scrollbar-track {
  background: #000000;
}

.sftp-list:hover::-webkit-scrollbar-thumb {
  background-color: #000000;
  border-color: #e3946c;
}

.sftp-list:hover::-webkit-scrollbar-thumb:hover {
  background-color: #5b5552;
}

.empty-message {
  padding: 20px;
  text-align: center;
  color: rgba(255, 255, 255, 0.4);
  font-size: 13px;
}

.sftp-entry {
  display: grid;
  grid-template-columns: 24px 1fr auto;
  gap: 8px;
  padding: 6px 8px;
  cursor: pointer;
  border-radius: 4px;
  align-items: center;
  transition: background 0.15s;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.85);
}

.sftp-entry:hover {
  background: rgba(255, 255, 255, 0.08);
}

.sftp-entry.is-dir {
  font-weight: 500;
}

.entry-icon {
  font-size: 16px;
}

.entry-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.entry-size {
  color: rgba(255, 255, 255, 0.5);
  font-size: 12px;
  text-align: right;
  min-width: 60px;
}

.sftp-entry.is-dir .entry-size {
  opacity: 0.5;
}

/* overlays */
.overlay { position: absolute; inset: 0; font-size: 14px; z-index: 2; }
.overlay.info { background: rgba(0,0,0,0.35); color: #e5e7eb; }
.overlay-center { position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); display: flex; flex-direction: column; align-items: center; gap: 12px; }
.overlay.error { background: rgba(239,68,68,0.2); color: #fecaca; font-weight: 600; }
/* simple spinner */
.spinner { width: 28px; height: 28px; border: 3px solid rgba(255,255,255,0.35); border-top-color: #fff; border-radius: 50%; animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
/* Scoped deep selectors to reach xterm's internal DOM */
:deep(.xterm) { width: 100%; height: 100%; flex: 1 1 auto; min-height: 0; margin: 0; }
:deep(.xterm-rows) { text-align: left !important; }
/* Enforce JetBrains Mono within xterm */
/* Scrollbar behavior: keep size stable; reveal via color on hover */
:deep(.xterm-viewport) {
  overflow-y: auto;
  height: 100%;
  flex: 1 1 auto;
  min-height: 0;
  /* Reserve space to avoid layout shift */
  scrollbar-gutter: stable;
  /* Firefox: keep thin width; hide via transparent colors */
  scrollbar-width: thin;
  scrollbar-color: transparent transparent;
}

/* Firefox reveal on hover via color only */
:deep(.xterm-viewport:hover) { scrollbar-color: #6c6765 #110d0a; }

/* WebKit: keep size constant */
:deep(.xterm-viewport::-webkit-scrollbar) { width: 10px; height: 10px; }
:deep(.xterm-viewport::-webkit-scrollbar-track) { background: transparent; border-radius: 6px; }
:deep(.xterm-viewport::-webkit-scrollbar-thumb) { background-color: transparent; border-radius: 6px; border: 2px solid transparent; }
:deep(.xterm-viewport:hover::-webkit-scrollbar-track) { background: #000000; }
:deep(.xterm-viewport:hover::-webkit-scrollbar-thumb) { background-color: #000000; border-color: #e3946c; }
:deep(.xterm-viewport:hover::-webkit-scrollbar-thumb:hover) { background-color: #5b5552; }
</style>







