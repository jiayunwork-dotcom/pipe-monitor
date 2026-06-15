<template>
  <a-config-provider :locale="zhCN" :theme="theme">
    <router-view />
  </a-config-provider>
</template>

<script setup>
import zhCN from 'ant-design-vue/es/locale/zh_CN'
import { theme as antTheme } from 'ant-design-vue'
import { onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useWebSocketStore } from '@/stores/ws'

const theme = {
  algorithm: antTheme.defaultAlgorithm,
  token: { colorPrimary: '#1677ff' }
}

const auth = useAuthStore()
const ws = useWebSocketStore()

onMounted(() => {
  if (auth.token && auth.user) {
    ws.connect(auth.token)
  }
})
</script>
