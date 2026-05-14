<template>
  <div class="card flex flex-col gap-2 hover:border-gray-700 transition-colors">
    <div class="flex items-start justify-between gap-2">
      <div class="flex items-center gap-2 min-w-0">
        <span :class="statusBadge">{{ task.status }}</span>
        <span class="text-xs text-gray-500 uppercase">{{ task.type === 'pr_review' ? 'PR Review' : 'Issue' }}</span>
      </div>
      <span class="text-xs text-gray-600 shrink-0">{{ task.score }}pts</span>
    </div>

    <div>
      <a :href="task.url" target="_blank" class="text-sm font-medium hover:text-green-400 transition-colors line-clamp-2">
        {{ task.title }}
      </a>
      <p class="text-xs text-gray-500 mt-0.5">{{ task.repo }} #{{ task.number }}</p>
    </div>

    <div class="flex items-center justify-between mt-1">
      <span class="text-xs text-gray-600">{{ timeAgo(task.created_at) }}</span>
      <div class="flex gap-2">
        <button v-if="task.status === 'running' || task.status === 'queued'"
          @click="$emit('cancel', task.id)"
          class="btn-danger text-xs px-2 py-1">
          Cancel
        </button>
        <button v-if="task.status === 'failed' || task.status === 'cancelled'"
          @click="$emit('retry', task.id)"
          class="btn text-xs px-2 py-1">
          Retry
        </button>
        <button @click="$emit('view-logs', task.id)"
          class="btn-sm">
          Logs
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Task } from '../stores/tasks'

defineProps<{ task: Task }>()
defineEmits<{ cancel: [id: string]; retry: [id: string]; 'view-logs': [id: string] }>()

const statusBadge = (task: Task) => {
  const map: Record<string, string> = {
    queued: 'badge-gray',
    running: 'badge-green animate-pulse',
    done: 'badge-blue',
    failed: 'badge-red',
    cancelled: 'badge-yellow',
  }
  return 'badge ' + (map[task.status] ?? 'badge-gray')
}

function timeAgo(ts: string) {
  const diff = Date.now() - new Date(ts).getTime()
  const m = Math.floor(diff / 60000)
  if (m < 1) return 'just now'
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ago`
  return `${Math.floor(h / 24)}d ago`
}
</script>

<style scoped>
.badge { @apply text-xs px-2 py-0.5 rounded-full font-medium; }
.badge-gray { @apply bg-gray-700 text-gray-300; }
.badge-green { @apply bg-green-900 text-green-300; }
.badge-blue { @apply bg-blue-900 text-blue-300; }
.badge-red { @apply bg-red-900 text-red-300; }
.badge-yellow { @apply bg-yellow-900 text-yellow-300; }
</style>
