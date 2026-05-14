<template>
  <div class="space-y-6">
    <!-- Stats row -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
      <div v-for="s in stats" :key="s.label" class="card text-center">
        <div class="text-2xl font-bold" :class="s.color">{{ s.value }}</div>
        <div class="text-xs text-gray-500 mt-1">{{ s.label }}</div>
      </div>
    </div>

    <!-- Active tasks -->
    <section>
      <h2 class="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">Running</h2>
      <div v-if="running.length === 0" class="text-sm text-gray-600">No tasks running.</div>
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <TaskCard v-for="t in running" :key="t.id" :task="t"
          @cancel="store.cancel" @retry="store.retry" @view-logs="openLogs" />
      </div>
    </section>

    <!-- Queued tasks -->
    <section>
      <h2 class="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">Queued</h2>
      <div v-if="queued.length === 0" class="text-sm text-gray-600">Queue empty.</div>
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <TaskCard v-for="t in queued" :key="t.id" :task="t"
          @cancel="store.cancel" @retry="store.retry" @view-logs="openLogs" />
      </div>
    </section>

    <!-- Log drawer -->
    <div v-if="activeLogTask"
      class="fixed inset-x-0 bottom-0 bg-gray-900 border-t border-gray-700 p-4 z-50 flex flex-col"
      style="max-height: 60vh">
      <LogViewer :task-id="activeLogTask" @close="activeLogTask = null" class="flex-1 min-h-0" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useTaskStore } from '../stores/tasks'
import TaskCard from '../components/TaskCard.vue'
import LogViewer from '../components/LogViewer.vue'

const store = useTaskStore()
const activeLogTask = ref<string | null>(null)

const running = computed(() => store.tasks.filter(t => t.status === 'running'))
const queued  = computed(() => store.tasks.filter(t => t.status === 'queued'))

const stats = computed(() => [
  { label: 'Running',   value: store.tasks.filter(t => t.status === 'running').length,   color: 'text-green-400' },
  { label: 'Queued',    value: store.tasks.filter(t => t.status === 'queued').length,    color: 'text-yellow-400' },
  { label: 'Done',      value: store.tasks.filter(t => t.status === 'done').length,      color: 'text-blue-400' },
  { label: 'Failed',    value: store.tasks.filter(t => t.status === 'failed').length,    color: 'text-red-400' },
])

function openLogs(taskId: string) { activeLogTask.value = taskId }

let interval: ReturnType<typeof setInterval>
onMounted(() => { store.fetchTasks(); interval = setInterval(store.fetchTasks, 5000) })
onUnmounted(() => clearInterval(interval))
</script>
