<template>
  <div class="ssh-list">
    <header class="list-header">
      <span>SSH 实例</span>
      <div class="actions">
        <button class="fwd" title="端口转发" @click="$emit('open-forward')">
          <svg version="1.1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px" viewBox="0 0 512 512" enable-background="new 0 0 512 512" xml:space="preserve"><g><path d="M336.6,157.5l-33.6-33.4c-3.5-3.5-8.5-4.9-13.6-3.6c-1.2,0.3-2.4,0.8-3.5,1.5c-4.7,2.9-7.2,7.8-6.8,13.1c0.2,3.4,1.9,6.6,4.3,9.1l16,15.9H142c-20.8,0-40.3,8.1-55.1,22.9C72.1,197.7,64,217.2,64,238v16c0,7.7,6.3,14,14,14l0,0c7.7,0,14-6.3,14-14v-16c0-13.3,5.2-25.8,14.7-35.3c9.5-9.5,22-14.7,35.3-14.7h155.4l-16,15.9c-2.4,2.4-4,5.4-4.3,8.7c-0.4,5.3,2.1,10.2,6.8,13.1c1.1,0.7,2.3,1.2,3.5,1.5c5,1.3,10.1-0.1,13.6-3.6l33.6-33.4c4.2-4.1,6.5-9.7,6.5-15.5C343,167.1,340.7,161.6,336.6,157.5z"/><path d="M434,244L434,244c-7.7,0-14,6.3-14,14v16c0,13.3-5.2,25.8-14.7,35.3c-9.5,9.5-22,14.7-35.3,14.7H214.6l16-15.9c2.4-2.4,4-5.4,4.3-8.8c0.4-5.3-2.1-10.2-6.8-13.1c-1.1-0.7-2.3-1.2-3.5-1.5c-5-1.3-10.1,0.1-13.6,3.6l-35.6,35.4c-4.2,4.1-6.5,9.7-6.5,15.5c0,5.9,2.3,11.4,6.5,15.5l33.6,33.4c3.5,3.5,8.5,4.9,13.6,3.6c1.2-0.3,2.4-0.8,3.5-1.5c4.7-2.9,7.2-7.8,6.8-13.1c-0.2-3.4-1.9-6.6-4.3-9.1l-16-15.9H370c43,0,78-35,78-78v-16C448,250.3,441.7,244,434,244z"/></g></svg>
        </button>
        <button class="add" title="新建 SSH" @click="$emit('add')">
          <svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 512 512"><path fill="none" stroke="currentColor" stroke-linecap="square" stroke-linejoin="round" stroke-width="32" d="M256 112v288"></path><path fill="none" stroke="currentColor" stroke-linecap="square" stroke-linejoin="round" stroke-width="32" d="M400 256H112"></path></svg>
        </button>
      </div>
    </header>
    <div class="list-body">
      <div
        v-for="it in props.instances"
        :key="it.id"
        class="item"
        :class="statusClasses(it.id)"
        @click="onClick(it.id)"
        @dblclick="onDblClick(it.id)"
        @contextmenu.prevent="onContextMenu($event, it.id)"
      >
        <div class="title">
          <span class="status-dot" :class="stateFor(it.id)"></span>
          {{ it.label || it.host || '未命名' }}
        </div>
        <div class="meta">{{ it.user || 'user' }}@{{ it.host || 'host:22' }}</div>
      </div>
    </div>

    <!-- Context menu -->
    <div v-if="menu.show" class="ctx" :style="{ left: menu.x + 'px', top: menu.y + 'px' }" @click.stop>
      <button class="ctx-item" :disabled="locked" @click="emitEdit">编辑</button>
      <button class="ctx-item danger" :disabled="locked" @click="emitDelete">删除</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, onMounted, onBeforeUnmount, computed, watch } from 'vue'

const props = defineProps<{
  instances: Array<{ id: string; label?: string; host?: string; user?: string; status?: 'idle' | 'connecting' | 'connected' }>
  selectedId?: string
  status?: 'idle' | 'connecting' | 'connected'
  connectedId?: string
  connectingId?: string
  connectedIds?: string[]
  connectingIds?: string[]
}>()

const emit = defineEmits<{
  (e: 'select', id: string): void
  (e: 'add'): void
  (e: 'open-forward'): void
  (e: 'edit', id: string): void
  (e: 'delete', id: string): void
  (e: 'connect', id: string): void
}>()

const menu = reactive({ show: false, x: 0, y: 0, id: '' as string })

type SessionState = 'idle' | 'connecting' | 'connected'

function stateFor(id: string): SessionState {
  if (!id) return 'idle'
  // Prefer multi-connection arrays when provided
  if (Array.isArray(props.connectedIds) && props.connectedIds.includes(id)) return 'connected'
  if (Array.isArray(props.connectingIds) && props.connectingIds.includes(id)) return 'connecting'
  // Backward compatibility with single ids
  if (props.connectedId === id) return 'connected'
  if (props.connectingId === id) return 'connecting'
  const item = props.instances.find(x => x.id === id)
  if (item?.status === 'connected' || item?.status === 'connecting') {
    return item.status
  }
  return 'idle'
}

function statusClasses(id: string) {
  const state = stateFor(id)
  return {
    active: id === props.selectedId,
    connected: state === 'connected',
    connecting: state === 'connecting'
  }
}

function isInteractionLocked(id: string) {
  const state = stateFor(id)
  return state === 'connected' || state === 'connecting'
}

function onClick(id: string) { emit('select', id) }

function onContextMenu(e: MouseEvent, id: string) {
  if (isInteractionLocked(id)) return
  const PADDING = 8
  const ITEM_H = 36
  const HEIGHT = PADDING * 2 + ITEM_H * 2
  const WIDTH = 180
  let x = e.clientX
  let y = e.clientY
  const vw = window.innerWidth
  const vh = window.innerHeight
  if (x + WIDTH > vw) x = vw - WIDTH - 6
  if (y + HEIGHT > vh) y = vh - HEIGHT - 6
  menu.show = true
  menu.x = Math.max(6, x)
  menu.y = Math.max(6, y)
  menu.id = id
}

function closeMenu() { menu.show = false; menu.id = '' }

const locked = computed(() => !!menu.id && isInteractionLocked(menu.id))

function emitEdit() { if (menu.id && !isInteractionLocked(menu.id)) emit('edit', menu.id); closeMenu() }

function emitDelete() { if (menu.id && !isInteractionLocked(menu.id)) emit('delete', menu.id); closeMenu() }

function onGlobalClick() { if (menu.show) closeMenu() }

function onEsc(e: KeyboardEvent) { if (e.key === 'Escape') closeMenu() }

onMounted(() => { window.addEventListener('click', onGlobalClick); window.addEventListener('keydown', onEsc) })
onBeforeUnmount(() => { window.removeEventListener('click', onGlobalClick); window.removeEventListener('keydown', onEsc) })

function onDblClick(id: string) { emit('connect', id) }

watch(() => stateFor(menu.id), (state) => { if (!menu.show) return; if (state !== 'idle') closeMenu() })

</script>

<style scoped>
.ssh-list { height: 100%; display: flex; flex-direction: column; position: relative; }
.list-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
  margin-bottom: 8px;
}
.add {
  padding: 4px 8px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.add svg {
  width: 16px;
  height: 16px;
  display: block;
}
.actions { display: flex; gap: 8px; align-items: center; }
.fwd {
  padding: 4px 8px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.fwd svg {
  width: 16px;
  height: 16px;
  display: block;
  fill: currentColor;
}
.list-body {
  overflow-y: auto;
  height: 100%;
  /* Reserve space to avoid layout shift */
  scrollbar-gutter: stable;
  /* Firefox: keep thin width; hide via transparent colors */
  scrollbar-width: thin;
  scrollbar-color: transparent transparent;
}
/* Firefox reveal on hover via color only */
.list-body:hover {
  scrollbar-color: #6c6765 #110d0a;
}
/* WebKit: keep size constant */
.list-body::-webkit-scrollbar {
  width: 10px;
  height: 10px;
}
.list-body::-webkit-scrollbar-track {
  background: transparent;
  border-radius: 6px;
}
.list-body::-webkit-scrollbar-thumb {
  background-color: transparent;
  border-radius: 6px;
  border: 2px solid transparent;
}
.list-body:hover::-webkit-scrollbar-track {
  background: #000000;
}
.list-body:hover::-webkit-scrollbar-thumb {
  background-color: #000000;
  border-color: #e3946c;
}
.list-body:hover::-webkit-scrollbar-thumb:hover {
  background-color: #5b5552;
}
.item { background: rgba(255,255,255,0.06); padding: 10px; border-radius: 8px; cursor: pointer; transition: background 0.2s ease; }
.item + .item { margin-top: 10px; }
.item:hover { background: rgba(36,99,235,0.25); }
.item.active { background: rgba(36,99,235,0.25); outline: 1px solid rgba(36,99,235,0.35); }
.item.connecting { opacity: 0.85; }
.title{ display:flex; align-items:center; justify-content:center; gap:6px;}
.status-dot { width: 8px; height: 8px; border-radius: 50%; background: rgba(156,163,175,0.6); box-shadow: 0 0 0 1px rgba(255,255,255,0.12) inset; display: inline-block; transition: background 0.2s ease, box-shadow 0.2s ease; }
.status-dot.connected { background: rgba(16,185,129,0.95); box-shadow: 0 0 0 1px rgba(16,185,129,0.55) inset; }
.status-dot.connecting { background: rgba(250,204,21,0.9); box-shadow: 0 0 0 1px rgba(250,204,21,0.55) inset; animation: pulse 1.2s ease-in-out infinite; }
@keyframes pulse { 0%,100% { transform: scale(1); opacity: .9 } 50% { transform: scale(0.8); opacity: .6 } }
.meta { opacity: 0.7; font-size: 12px; margin-top: 2px; }
/* Context menu */
.ctx { position: fixed; z-index: 2000; min-width: 180px; padding: 8px; background: #1f2937; border: 1px solid rgba(255,255,255,0.08); border-radius: 12px; box-shadow: 0 20px 40px rgba(0,0,0,0.45); }
.ctx-item { display: flex; align-items: center; gap: 10px; width: 100%; height: 36px; text-align: left; padding: 0 12px; background: transparent; color: #e5e7eb; border: none; border-radius: 8px; cursor: pointer; font-size: 13px; }
.ctx-item:hover { background: rgba(255,255,255,0.06); }
.ctx-item.danger { color: #fda4af; }
.ctx-item.danger:hover { background: rgba(239,68,68,0.12); }
.ctx-item:disabled { opacity: 0.5; cursor: not-allowed; }
</style>

