<template>
  <div class="modal-overlay">
    <div class="modal-card">
      <div class="modal-header">
        <h2>AI 配置</h2>
        <button class="close-btn" @click="$emit('close')">×</button>
      </div>
      <div class="modal-body">
        <div class="form-group">
          <label>代理地址</label>
          <input
            v-model="form.proxy"
            type="text"
            placeholder="http://127.0.0.1:10808"
          />
        </div>
        <div class="form-group">
          <label>模型名称</label>
          <input
            v-model="form.model"
            type="text"
            placeholder="gpt-4o"
          />
        </div>
        <div class="form-group">
          <label>思考模式</label>
          <select v-model="form.reason">
            <option value="low">Low</option>
            <option value="medium">Medium</option>
            <option value="high">High</option>
          </select>
        </div>
        <div class="form-group">
          <label>API 格式支持</label>
          <select v-model="form.provider">
            <option value="openai">OpenAI</option>
            <option value="anthropic">Anthropic</option>
            <option value="gemini">Gemini</option>
          </select>
        </div>
        <div class="form-group">
          <label>自定义 API URL（可选）</label>
          <input
            v-model="form.baseurl"
            type="text"
            placeholder="留空使用官方API地址"
          />
        </div>
        <div class="form-group">
          <label>API Key</label>
          <input
            v-model="form.apikey"
            type="password"
            placeholder="sk-..."
          />
        </div>
      </div>
      <div class="modal-footer">
        <button class="btn-secondary" @click="$emit('close')">取消</button>
        <button class="btn-primary" @click="onSave">保存</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive } from 'vue'

const props = defineProps<{
  initialConfig?: { proxy: string; model: string; reason: string; provider: string; apikey: string; baseurl: string } | null
}>()

const emit = defineEmits<{
  close: []
  save: [config: { proxy: string; model: string; reason: string; provider: string; apikey: string; baseurl: string }]
}>()

const form = reactive({
  proxy: props.initialConfig?.proxy || '',
  model: props.initialConfig?.model || 'gpt-5',
  reason: props.initialConfig?.reason || 'medium',
  provider: props.initialConfig?.provider || 'openai',
  apikey: props.initialConfig?.apikey || '',
  baseurl: props.initialConfig?.baseurl || ''
})

function onSave() {
  emit('save', {
    proxy: form.proxy,
    model: form.model,
    reason: form.reason,
    provider: form.provider,
    apikey: form.apikey,
    baseurl: form.baseurl
  })
  emit('close')
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.6);
  display: grid;
  place-items: center;
  z-index: 1000;
  backdrop-filter: blur(2px);
}

.modal-card {
  background: #1e293b;
  border-radius: 12px;
  width: 90%;
  max-width: 480px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.modal-header h2 {
  margin: 0;
  font-size: 18px;
  color: #e5e7eb;
}

.close-btn {
  background: transparent;
  border: none;
  color: #9ca3af;
  font-size: 24px;
  cursor: pointer;
  width: 32px;
  height: 32px;
  display: grid;
  place-items: center;
  border-radius: 6px;
}

.close-btn:hover {
  background: rgba(255, 255, 255, 0.06);
  color: #e5e7eb;
}

.modal-body {
  padding: 20px;
  max-height: 60vh;
  overflow-y: auto;
}

.form-group {
  margin-bottom: 16px;
}

.form-group label {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  color: #9ca3af;
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 10px 12px;
  background: rgba(0, 0, 0, 0.25);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 6px;
  color: #e5e7eb;
  font-size: 14px;
  box-sizing: border-box;
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: rgba(16, 185, 129, 0.5);
}

.form-group select {
  cursor: pointer;
}

.form-group select option {
  background: #1e293b;
  color: #e5e7eb;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 20px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.btn-primary,
.btn-secondary {
  padding: 8px 16px;
  border-radius: 6px;
  border: none;
  cursor: pointer;
  font-size: 14px;
  font-weight: 500;
  transition: background 0.2s;
}

.btn-primary {
  background: #10b981;
  color: white;
}

.btn-primary:hover {
  background: #059669;
}

.btn-secondary {
  background: rgba(255, 255, 255, 0.08);
  color: #e5e7eb;
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.12);
}
</style>
