import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface Task {
  id: string
  type: 'issue' | 'pr_review'
  status: 'queued' | 'running' | 'done' | 'failed' | 'cancelled'
  repo: string
  number: number
  title: string
  url: string
  score: number
  created_at: string
  started_at: string | null
  finished_at: string | null
}

export const useTaskStore = defineStore('tasks', () => {
  const tasks = ref<Task[]>([])
  const loading = ref(false)

  async function fetchTasks(status?: string) {
    loading.value = true
    const url = status ? `/api/tasks?status=${status}` : '/api/tasks'
    const res = await fetch(url)
    tasks.value = await res.json()
    loading.value = false
  }

  async function cancel(taskId: string) {
    await fetch(`/api/tasks/${encodeURIComponent(taskId)}/cancel`, { method: 'POST' })
    await fetchTasks()
  }

  async function retry(taskId: string) {
    await fetch(`/api/tasks/${encodeURIComponent(taskId)}/retry`, { method: 'POST' })
    await fetchTasks()
  }

  return { tasks, loading, fetchTasks, cancel, retry }
})
