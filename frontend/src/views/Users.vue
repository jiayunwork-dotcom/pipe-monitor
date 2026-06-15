<template>
  <a-card title="成员管理">
    <template #extra>
      <a-space>
        <a-input-search allow-clear placeholder="搜索" style="width:220px;" />
        <a-button v-if="auth.isAdmin" type="primary"><plus-outlined />邀请成员</a-button>
      </a-space>
    </template>
    <a-table :columns="cols" :data-source="users" row-key="id" :loading="loading">
      <template #bodyCell="{column, record}">
        <template v-if="column.key==='avatar'">
          <a-avatar style="background-color:#1677ff;">{{ record.fullName?.[0] || record.username?.[0] }}</a-avatar>
          <span style="margin-left:8px;">{{ record.fullName }}</span>
        </template>
        <template v-else-if="column.key==='role'">
          <a-tag v-if="record.role==='super_admin'" color="magenta">超级管理员</a-tag>
          <a-tag v-else-if="record.role==='admin'" color="blue">管理员</a-tag>
          <a-tag v-else color="default">普通成员</a-tag>
        </template>
        <template v-else-if="column.key==='st'">
          <a-badge :status="record.status==='active'?'success':'default'" :text="record.status==='active'?'启用':'禁用'" />
        </template>
        <template v-else-if="column.key==='last'">{{ record.lastLoginAt?.slice(0,16).replace('T',' ') || '-' }}</template>
      </template>
    </a-table>
  </a-card>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { userApi } from '@/api'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const loading = ref(false)
const users = ref([])
const cols = [
  { title:'用户', key:'avatar', width:200 },
  { title:'用户名', dataIndex:'username', width:150 },
  { title:'邮箱', dataIndex:'email', width:220 },
  { title:'手机号', dataIndex:'phone', width:140 },
  { title:'角色', key:'role', width:130 },
  { title:'状态', key:'st', width:100 },
  { title:'最后登录', key:'last', width:170 }
]

onMounted(async () => {
  loading.value = true
  try {
    const r = await userApi.list()
    users.value = r.data || []
  } finally { loading.value = false }
})
</script>
