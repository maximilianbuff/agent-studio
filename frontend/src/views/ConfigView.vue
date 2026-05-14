<template>
  <div class="space-y-8 max-w-3xl">
    <h1 class="text-sm font-semibold text-gray-400 uppercase tracking-wider">Configuration</h1>

    <!-- Schedule status -->
    <section class="card space-y-3">
      <h2 class="text-xs text-gray-500 uppercase tracking-wider font-semibold">Cron Schedule</h2>
      <div class="grid grid-cols-2 gap-4 text-sm">
        <div>
          <div class="text-xs text-gray-500 mb-1">Last scan</div>
          <div class="font-mono text-gray-300">{{ schedule.last_scan_at ? timeAgo(schedule.last_scan_at) : 'never' }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 mb-1">Next scan (approx)</div>
          <div class="font-mono text-gray-300">{{ nextScan }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 mb-1">Running / Max parallel</div>
          <div class="font-mono text-gray-300">{{ schedule.running_count }} / {{ schedule.max_parallel }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 mb-1">Queued tasks</div>
          <div class="font-mono text-gray-300">{{ schedule.queued_count }}</div>
        </div>
      </div>
      <div class="text-xs text-gray-600 pt-1 border-t border-gray-800">
        Scan every {{ schedule.scan_interval_minutes }}m · worker triggered every 2m via Claude Code cron
      </div>
    </section>

    <!-- Agent settings -->
    <section class="card space-y-4">
      <h2 class="text-xs text-gray-500 uppercase tracking-wider font-semibold">Agent Settings</h2>
      <div class="grid grid-cols-2 gap-4">
        <div v-for="(val, key) in agentConfig" :key="key">
          <label class="text-xs text-gray-500 block mb-1">{{ key }}</label>
          <div class="flex gap-2">
            <input v-model="agentConfig[key]" class="input" />
            <button @click="saveAgent(key, agentConfig[key])" class="btn-sm">Save</button>
          </div>
        </div>
      </div>
    </section>

    <!-- Label scores -->
    <section class="card space-y-3">
      <h2 class="text-xs text-gray-500 uppercase tracking-wider font-semibold">Label Scores</h2>
      <div class="space-y-2">
        <div v-for="l in labels" :key="l.label" class="flex items-center gap-2">
          <span class="text-sm flex-1">{{ l.label }}</span>
          <input v-model.number="l.score" type="number" class="input w-20" />
          <button @click="saveLabel(l)" class="btn-sm">Save</button>
          <button @click="deleteLabel(l.label)" class="text-red-500 hover:text-red-400 text-xs">✕</button>
        </div>
      </div>
      <div class="flex gap-2 mt-2">
        <input v-model="newLabel.label" placeholder="label" class="input" />
        <input v-model.number="newLabel.score" type="number" placeholder="score" class="input w-20" />
        <button @click="addLabel" class="btn-sm">Add</button>
      </div>
    </section>

    <!-- Keyword scores -->
    <section class="card space-y-3">
      <h2 class="text-xs text-gray-500 uppercase tracking-wider font-semibold">Keyword Scores</h2>
      <div class="space-y-2">
        <div v-for="k in keywords" :key="k.keyword" class="flex items-center gap-2">
          <span class="text-sm flex-1">{{ k.keyword }}</span>
          <input v-model.number="k.score" type="number" class="input w-20" />
          <button @click="saveKeyword(k)" class="btn-sm">Save</button>
          <button @click="deleteKeyword(k.keyword)" class="text-red-500 hover:text-red-400 text-xs">✕</button>
        </div>
      </div>
      <div class="flex gap-2 mt-2">
        <input v-model="newKeyword.keyword" placeholder="keyword" class="input" />
        <input v-model.number="newKeyword.score" type="number" placeholder="score" class="input w-20" />
        <button @click="addKeyword" class="btn-sm">Add</button>
      </div>
    </section>

    <!-- Repo priorities -->
    <section class="card space-y-3">
      <h2 class="text-xs text-gray-500 uppercase tracking-wider font-semibold">Repo Priority Overrides</h2>
      <div class="space-y-2">
        <div v-for="r in repos" :key="r.repo" class="flex items-center gap-2">
          <span class="text-sm flex-1 truncate">{{ r.repo }}</span>
          <input v-model.number="r.priority_weight" type="number" placeholder="weight" class="input w-20" />
          <select v-model="r.enabled" class="input w-24">
            <option value="true">enabled</option>
            <option value="false">disabled</option>
          </select>
          <button @click="saveRepo(r)" class="btn-sm">Save</button>
        </div>
      </div>
      <div class="flex gap-2 mt-2">
        <input v-model="newRepo.repo" placeholder="owner/repo" class="input" />
        <input v-model.number="newRepo.priority_weight" type="number" placeholder="weight" class="input w-20" />
        <button @click="addRepo" class="btn-sm">Add</button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'

const agentConfig = ref<Record<string, string>>({})
const labels = ref<{ label: string; score: number }[]>([])
const keywords = ref<{ keyword: string; score: number }[]>([])
const repos = ref<{ repo: string; priority_weight: number; enabled: string }[]>([])
const schedule = ref({ last_scan_at: '', scan_interval_minutes: '120', running_count: 0, queued_count: 0, max_parallel: '2' })

const newLabel = ref({ label: '', score: 5 })
const newKeyword = ref({ keyword: '', score: 5 })
const newRepo = ref({ repo: '', priority_weight: 0 })

const nextScan = computed(() => {
  if (!schedule.value.last_scan_at) return 'unknown'
  const last = new Date(schedule.value.last_scan_at).getTime()
  const interval = parseInt(schedule.value.scan_interval_minutes) * 60000
  const next = new Date(last + interval)
  const diff = next.getTime() - Date.now()
  if (diff <= 0) return 'due now'
  const m = Math.round(diff / 60000)
  return m < 60 ? `in ${m}m` : `in ${Math.round(m / 60)}h`
})

function timeAgo(ts: string) {
  const diff = Date.now() - new Date(ts).getTime()
  const m = Math.floor(diff / 60000)
  if (m < 1) return 'just now'
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ago`
  return `${Math.floor(h / 24)}d ago`
}

async function load() {
  const [configRes, schedRes] = await Promise.all([
    fetch('/api/config'),
    fetch('/api/schedule'),
  ])
  const data = await configRes.json()
  agentConfig.value = data.agent
  labels.value = data.labels
  keywords.value = data.keywords
  repos.value = data.repos ?? []
  schedule.value = await schedRes.json()
}

async function saveAgent(key: string, value: string) {
  await fetch('/api/config/agent', {
    method: 'PUT', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, value }),
  })
}

async function saveLabel(l: { label: string; score: number }) {
  await fetch('/api/config/labels', {
    method: 'PUT', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(l),
  })
}

async function deleteLabel(label: string) {
  await fetch(`/api/config/labels/${encodeURIComponent(label)}`, { method: 'DELETE' })
  labels.value = labels.value.filter(l => l.label !== label)
}

async function addLabel() {
  if (!newLabel.value.label) return
  await saveLabel(newLabel.value)
  labels.value.push({ ...newLabel.value })
  newLabel.value = { label: '', score: 5 }
}

async function saveKeyword(k: { keyword: string; score: number }) {
  await fetch('/api/config/keywords', {
    method: 'PUT', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(k),
  })
}

async function deleteKeyword(keyword: string) {
  await fetch(`/api/config/keywords/${encodeURIComponent(keyword)}`, { method: 'DELETE' })
  keywords.value = keywords.value.filter(k => k.keyword !== keyword)
}

async function addKeyword() {
  if (!newKeyword.value.keyword) return
  await saveKeyword(newKeyword.value)
  keywords.value.push({ ...newKeyword.value })
  newKeyword.value = { keyword: '', score: 5 }
}

async function saveRepo(r: { repo: string; priority_weight: number; enabled: string }) {
  await fetch('/api/config/repos', {
    method: 'PUT', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(r),
  })
}

async function addRepo() {
  if (!newRepo.value.repo) return
  const r = { ...newRepo.value, enabled: 'true' }
  await saveRepo(r)
  repos.value.push(r)
  newRepo.value = { repo: '', priority_weight: 0 }
}

let interval: ReturnType<typeof setInterval>
onMounted(() => { load(); interval = setInterval(load, 15000) })
onUnmounted(() => clearInterval(interval))
</script>
