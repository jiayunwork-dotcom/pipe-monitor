<template>
  <div style="min-height:100vh;display:flex;flex-direction:column;align-items:center;justify-content:center;background:linear-gradient(135deg,#667eea 0%,#764ba2 100%);">
    <a-card style="width:420px;box-shadow:0 20px 60px rgba(0,0,0,0.3);border-radius:12px;">
      <div style="text-align:center;margin-bottom:32px;">
        <a-typography-title :level="2" style="margin:0 0 8px;color:#1677ff;">Pipeline Monitor</a-typography-title>
        <a-typography-text type="secondary">数据管道统一监控平台</a-typography-text>
      </div>
      <a-form :model="form" layout="vertical" @finish="onLogin">
        <a-form-item label="用户名" name="username" :rules="[{required:true,message:'请输入用户名'}]">
          <a-input v-model:value="form.username" size="large" placeholder="用户名/邮箱">
            <template #prefix><user-outlined /></template>
          </a-input>
        </a-form-item>
        <a-form-item label="密码" name="password" :rules="[{required:true,message:'请输入密码'}]">
          <a-input-password v-model:value="form.password" size="large" placeholder="密码">
            <template #prefix><lock-outlined /></template>
          </a-input-password>
        </a-form-item>
        <a-button type="primary" html-type="submit" size="large" block :loading="loading">登录</a-button>
      </a-form>
      <a-divider>默认账户</a-divider>
      <a-descriptions :column="1" size="small">
        <a-descriptions-item label="超级管理员">superadmin / Super@2024!</a-descriptions-item>
        <a-descriptions-item label="管理员">bi_admin / Admin@2024!</a-descriptions-item>
        <a-descriptions-item label="普通成员">bi_member / User@2024!</a-descriptions-item>
      </a-descriptions>
    </a-card>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { UserOutlined, LockOutlined } from '@ant-design/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { useWebSocketStore } from '@/stores/ws'

const form = reactive({ username: 'superadmin', password: 'Super@2024!' })
const loading = ref(false)
const router = useRouter()
const auth = useAuthStore()
const ws = useWebSocketStore()

async function onLogin() {
  loading.value = true
  try {
    await auth.login(form.username, form.password)
    message.success('登录成功')
    ws.connect(auth.token)
    router.push('/')
  } finally {
    loading.value = false
  }
}
</script>
