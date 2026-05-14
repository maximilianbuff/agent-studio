<template>
  <div class="flex flex-col h-full">
    <div class="flex items-center justify-between mb-3">
      <h3 class="text-sm font-medium text-gray-300">Logs — {{ taskId }}</h3>
      <div class="flex gap-2">
        <label class="flex items-center gap-1 text-xs text-gray-500 cursor-pointer">
          <input type="checkbox" v-model="autoScroll" class="accent-green-500" />
          Auto-scroll
        </label>
        <button @click="$emit('close')" class="text-gray-500 hover:text-white text-lg leading-none">×</button>
      </div>
    </div>
    <div ref="logEl"
      class="flex-1 overflow-y-auto bg-black rounded p-3 text-xs font-mono leading-relaxed min-h-0"
      style="max-height: 500px">
      <div v-for="(line, i) in lines" :key="i" class="whitespace-pre-wrap break-all">
        <span class="text-gray-600 select-none mr-2">{{ line.ts }}</span>
        <span :class="lineColor(line.message)">{{ line.message }}</span>
      </div>
      <div v-if="lines.length === 0" class="text-gray-600">No logs yet...</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue'

const props = defineProps<{ taskId: string }>()
defineEmits(['close'])

const lines = ref<{ ts: string; message: string }[]>([])
const autoScroll = ref(true)
const logEl = ref<HTMLElement>()
let ws: WebSocket | null = null

function lineColor(msg: string) {
  if (/error|failed|fatal/i.test(msg)) return 'text-red-400'
  if (/warn/i.test(msg)) return 'text-yellow-400'
  if (/phase|════/i.test(msg)) return 'text-green-400 font-bold'
  if (/done|success|complete/i.test(msg)) return 'text-blue-400'
  return 'text-gray-300'
}

function scrollToBottom() {
  if (autoScroll.value && logEl.value) {
    logEl.value.scrollTop = logEl.value.scrollHeight
  }
}

watch(lines, () => nextTick(scrollToBottom), { deep: true })

function connect() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/ws/logs/${props.taskId}`)
  ws.onmessage = (e) => {
    const data = JSON.parse(e.data)
    lines.value.push({ ts: data.ts?.slice(11, 19) ?? '', message: data.message })
  }
  ws.onclose = () => setTimeout(connect, 3000)
}

onMounted(connect)
onUnmounted(() => ws?.close())
</script>
