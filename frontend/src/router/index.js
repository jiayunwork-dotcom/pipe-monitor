import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes = [
  { path: '/login', name: 'Login', component: () => import('@/views/Login.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'Dashboard', component: () => import('@/views/Dashboard.vue'), meta: { title: '全局大盘' } },
      { path: 'pipelines', name: 'PipelineList', component: () => import('@/views/pipelines/List.vue'), meta: { title: '管道管理' } },
      { path: 'pipelines/:id', name: 'PipelineDetail', component: () => import('@/views/pipelines/Detail.vue'), meta: { title: '管道详情' } },
      { path: 'dag', name: 'DAGView', component: () => import('@/views/DAG.vue'), meta: { title: '依赖图' } },
      { path: 'runs', name: 'RunList', component: () => import('@/views/Runs.vue'), meta: { title: '运行记录' } },
      { path: 'sla', name: 'SLAView', component: () => import('@/views/SLA.vue'), meta: { title: 'SLA中心' } },
      { path: 'alerts', name: 'AlertList', component: () => import('@/views/alerts/List.vue'), meta: { title: '告警中心' } },
      { path: 'alert-rules', name: 'AlertRules', component: () => import('@/views/alerts/Rules.vue'), meta: { title: '告警规则' } },
      { path: 'oncall', name: 'OnCall', component: () => import('@/views/OnCall.vue'), meta: { title: '值班管理' } },
      { path: 'users', name: 'UserList', component: () => import('@/views/Users.vue'), meta: { title: '成员管理' } }
    ]
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' }
]

const router = createRouter({ history: createWebHistory(), routes })

router.beforeEach((to, from, next) => {
  const auth = useAuthStore()
  if (to.meta.public) return next()
  if (!auth.token) return next('/login')
  next()
})

export default router
