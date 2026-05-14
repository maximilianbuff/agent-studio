<template>
  <div class="space-y-4">
    <div class="flex items-center gap-3">
      <h1 class="text-sm font-semibold text-gray-400 uppercase tracking-wider">All Tasks</h1>
      <div class="flex gap-1 ml-auto">
        <button v-for="s in ['all', 'queued', 'running', 'done', 'failed', 'cancelled']" :key="s"
          @click="filter = s"
          :class="['text-xs px-3 py-1 rounded transition-colors',
            filter === s ? 'bg-green-700 text-white' : 'bg-gray-800 text-gray-400 hover:bg-gray-700']">
          {{ s }}
        </button>
      </div>
    </div>

    <div v-if="store.loading" class="text-sm text-gray-600">Loading...</div>
    <div v-else-if="filtered.length === 0" class="text-sm text-gray-600">No tasks.</div>
    <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      <TaskCard v-for="t in filtered" :key="t.id" :task="t"
        @cancel="store.cancel" @retry="store.retry" @view-logs="openLogs" />
    </div>

    <div v-if="activeLogTask"
      class="fixed inset-x-0 bottom-0 bg-gray-900 border-t border-gray-700 p-4 z-50 flex flex-col"
      style="max-height: 60vh">
      <LogViewer :task-id="activeLogTask" @close="activeLogTask = null" class="flex-1 min-h-0" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useTaskStore } from '../stores/tasks'
import TaskCard from '../components/TaskCard.vue'
import LogViewer from '../components/LogViewer.vue'

const store = useTaskStore()
const filter = ref('all')
const activeLogTask = ref<string | null>(null)

const filtered = computed(() =>
  filter.value === 'all' ? store.tasks : store.tasks.filter(t => t.status === filter.value)
)

function openLogs(taskId: string) { activeLogTask.value = taskId }
onMounted(store.fetchTasks)
</script>
