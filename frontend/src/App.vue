<template>
  <div class="min-h-screen flex flex-col">
    <nav class="border-b border-gray-800 px-6 py-3 flex items-center gap-8">
      <span class="text-green-400 font-bold tracking-wider">AGENT STUDIO</span>
      <RouterLink to="/" class="nav-link">Dashboard</RouterLink>
      <RouterLink to="/tasks" class="nav-link">Tasks</RouterLink>
      <RouterLink to="/config" class="nav-link">Config</RouterLink>
      <div class="ml-auto flex items-center gap-3">
        <span class="text-xs text-gray-500">{{ scanStatus }}</span>
        <button @click="triggerScan" class="btn-sm">Scan Now</button>
      </div>
    </nav>
    <main class="flex-1 p-6">
      <RouterView />
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const scanStatus = ref('idle')

async function triggerScan() {
  scanStatus.value = 'scanning...'
  await fetch('/api/scan', { method: 'POST' })
  scanStatus.value = 'scan started'
  setTimeout(() => { scanStatus.value = 'idle' }, 3000)
}
</script>

<style>
.nav-link {
  @apply text-gray-400 hover:text-white text-sm transition-colors;
}
.router-link-active {
  @apply text-green-400;
}
.btn-sm {
  @apply px-3 py-1 text-xs bg-green-700 hover:bg-green-600 rounded transition-colors;
}
.btn {
  @apply px-4 py-2 text-sm bg-green-700 hover:bg-green-600 rounded transition-colors;
}
.btn-danger {
  @apply px-4 py-2 text-sm bg-red-800 hover:bg-red-700 rounded transition-colors;
}
.card {
  @apply bg-gray-900 border border-gray-800 rounded-lg p-4;
}
.input {
  @apply bg-gray-800 border border-gray-700 rounded px-3 py-1.5 text-sm focus:outline-none focus:border-green-500 w-full;
}
</style>
