import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import Dashboard from './views/Dashboard.vue'
import TasksView from './views/TasksView.vue'
import ConfigView from './views/ConfigView.vue'
import './style.css'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: Dashboard },
    { path: '/tasks', component: TasksView },
    { path: '/config', component: ConfigView },
  ],
})

createApp(App).use(createPinia()).use(router).mount('#app')
