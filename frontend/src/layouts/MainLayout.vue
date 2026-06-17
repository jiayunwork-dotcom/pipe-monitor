<template>
  <a-layout style="min-height:100vh;">
    <a-layout-sider v-model:collapsed="collapsed" collapsible theme="dark" width="220">
      <div style="height:64px;display:flex;align-items:center;justify-content:center;color:#fff;font-size:16px;font-weight:600;border-bottom:1px solid rgba(255,255,255,0.1);">
        <pie-chart-outlined style="margin-right:8px;font-size:22px;color:#1890ff;" />
        <span v-show="!collapsed">Pipe Monitor</span>
      </div>
      <a-menu theme="dark" mode="inline" :selected-keys="[$route.path]" @click="onMenuClick">
        <a-menu-item key="/dashboard"><dashboard-outlined /><span>全局大盘</span></a-menu-item>
        <a-menu-item key="/pipelines"><apartment-outlined /><span>管道管理</span></a-menu-item>
        <a-menu-item key="/dag"><cluster-outlined /><span>依赖拓扑</span></a-menu-item>
        <a-menu-item key="/lineage"><share-alt-outlined /><span>数据血缘</span></a-menu-item>
        <a-menu-item key="/runs"><sync-outlined /><span>运行记录</span></a-menu-item>
        <a-menu-item key="/sla"><schedule-outlined /><span>SLA中心</span></a-menu-item>
        <a-menu-item key="/alerts"><bell-outlined /><span>告警中心</span></a-menu-item>
        <a-menu-item key="/alert-rules"><setting-outlined /><span>告警规则</span></a-menu-item>
        <a-menu-item key="/oncall"><usergroup-add-outlined /><span>值班管理</span></a-menu-item>
        <a-menu-item v-if="auth.isAdmin" key="/users"><team-outlined /><span>成员管理</span></a-menu-item>
      </a-menu>
    </a-layout-sider>
    <a-layout>
      <a-layout-header style="background:#fff;padding:0 24px;display:flex;align-items:center;justify-content:space-between;box-shadow:0 1px 4px rgba(0,21,41,.08);z-index:10;">
        <a-typography-title :level="4" style="margin:0;">{{ $route.meta.title || '管道监控' }}</a-typography-title>
        <div style="display:flex;align-items:center;gap:16px;">
          <a-badge :count="unreadAlerts" dot><a-button type="text" @click="$router.push('/alerts')"><bell-outlined style="font-size:18px;" /></a-button></a-badge>
          <a-badge :status="ws.connected ? 'success' : 'error'" :text="ws.connected ? '实时连接' : '连接断开'" />
          <a-dropdown>
            <span style="cursor:pointer;display:flex;align-items:center;gap:8px;">
              <a-avatar style="background-color:#1677ff;">{{ auth.user?.fullName?.[0] || auth.user?.username?.[0] }}</a-avatar>
              <span>{{ auth.user?.fullName || auth.user?.username }}</span>
              <a-tag v-if="auth.user?.role === 'super_admin'" color="magenta">超管</a-tag>
              <a-tag v-else-if="auth.user?.role === 'admin'" color="blue">管理员</a-tag>
              <a-tag v-else color="default">成员</a-tag>
              <down-outlined />
            </span>
            <template #overlay>
              <a-menu @click="onProfileClick">
                <a-menu-item key="info"><user-outlined />个人信息</a-menu-item>
                <a-menu-item key="tenant"><team-outlined />所属: {{ auth.user?.tenantDisplayName }}</a-menu-item>
                <a-menu-divider />
                <a-menu-item key="logout"><logout-outlined />退出登录</a-menu-item>
              </a-menu>
            </template>
          </a-dropdown>
        </div>
      </a-layout-header>
      <a-layout-content style="padding:24px;background:#f0f2f5;min-height:calc(100vh - 64px);overflow:auto;">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { useAuthStore } from '@/stores/auth'
import { useWebSocketStore } from '@/stores/ws'
import {
  DashboardOutlined, ApartmentOutlined, ClusterOutlined, SyncOutlined, ScheduleOutlined,
  BellOutlined, SettingOutlined, UsergroupAddOutlined, TeamOutlined, PieChartOutlined,
  UserOutlined, LogoutOutlined, DownOutlined, ShareAltOutlined
} from '@ant-design/icons-vue'

const $route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const ws = useWebSocketStore()

const collapsed = ref(false)
const unreadAlerts = ref(0)

const onNewAlert = () => { unreadAlerts.value++ }

onMounted(() => {
  ws.on('new_alert', onNewAlert)
})
onUnmounted(() => {
  ws.off('new_alert', onNewAlert)
})

function onMenuClick({ key }) {
  if (key !== $route.path) router.push(key)
}

function onProfileClick({ key }) {
  if (key === 'logout') {
    auth.logout()
    ws.disconnect()
    message.success('已退出')
    router.push('/login')
  }
}
</script>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
