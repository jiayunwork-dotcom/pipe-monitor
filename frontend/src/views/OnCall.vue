<template>
  <a-space direction="vertical" style="width:100%;" :size="16">
    <a-card>
      <template #title>
        <div style="display:flex;justify-content:space-between;">
          <span>值班组管理</span>
          <a-button v-if="auth.isAdmin" type="primary" @click="openCreate"><plus-outlined />新建值班组</a-button>
        </div>
      </template>
      <a-row :gutter="16">
        <a-col :span="8" v-for="g in groups" :key="g.id">
          <a-card size="small" :title="g.name" :extra="h('a-button', {type:'link', size:'small', onClick:()=>selectGroup(g)}, ['管理'])">
            <a-descriptions :column="1" size="small">
              <a-descriptions-item label="轮转方式">{{ modeText[g.rotationMode] }}</a-descriptions-item>
              <a-descriptions-item label="时区">{{ g.timezone }}</a-descriptions-item>
              <a-descriptions-item label="描述">{{ g.description || '-' }}</a-descriptions-item>
            </a-descriptions>
          </a-card>
        </a-col>
      </a-row>
    </a-card>

    <a-card v-if="selectedGroup" :title="'值班表 - ' + selectedGroup.name">
      <a-row :gutter="16">
        <a-col :span="16">
          <a-table :columns="occCols" :data-source="assignments" row-key="id" size="small" :pagination="false">
            <template #bodyCell="{column, record}">
              <template v-if="column.key==='user'">{{ record.user?.fullName || record.user?.username }} <a-tag v-if="record.isBackup" color="orange">备份</a-tag></template>
              <template v-if="column.key==='date'">{{ formatDate(record.startDate) }} - {{ formatDate(record.endDate) }}</template>
            </template>
          </a-table>
        </a-col>
        <a-col :span="8">
          <a-descriptions :column="1" bordered title="当前值班">
            <a-descriptions-item label="值班人">{{ currentOcc?.user?.fullName || '-' }}</a-descriptions-item>
            <a-descriptions-item label="角色">{{ currentOcc?.isBackup ? '备份' : '主值班' }}</a-descriptions-item>
            <a-descriptions-item label="时间段">{{ formatDate(currentOcc?.startDate) }} ~ {{ formatDate(currentOcc?.endDate) }}</a-descriptions-item>
            <a-descriptions-item label="交接备注">{{ currentOcc?.handoverNote || '-' }}</a-descriptions-item>
          </a-descriptions>
          <a-divider />
          <a-space direction="vertical" style="width:100%;">
            <a-button type="primary" block @click="openHandover" v-if="auth.isAdmin"><swap-outlined />生成交接</a-button>
          </a-space>
        </a-col>
      </a-row>
    </a-card>

    <a-modal v-model:open="createVisible" title="新建值班组" @ok="saveGroup" :confirm-loading="saving" width="560px">
      <a-form :model="form" layout="vertical">
        <a-form-item label="名称" required><a-input v-model:value="form.name" /></a-form-item>
        <a-form-item label="轮转方式">
          <a-select v-model:value="form.rotationMode">
            <a-select-option value="daily">每天轮转</a-select-option>
            <a-select-option value="weekly">每周轮转</a-select-option>
            <a-select-option value="custom">自定义</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="成员(按顺序轮转)" required>
          <a-select mode="multiple" v-model:value="form.memberIds" :options="userOptions" />
        </a-form-item>
        <a-form-item label="开始日期"><a-date-picker v-model:value="form._start" style="width:100%;" /></a-form-item>
        <a-form-item label="描述"><a-textarea v-model:value="form.description" :rows="2" /></a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="handoverVisible" title="生成交接摘要" @ok="doHandover" width="600px">
      <a-form layout="vertical">
        <a-form-item label="下一值班人">
          <a-select v-model:value="handover.toUserId" :options="userOptions" />
        </a-form-item>
        <a-form-item label="交接备注"><a-textarea v-model:value="handover.notes" :rows="4" placeholder="需要特别说明的事项" /></a-form-item>
      </a-form>
    </a-modal>
  </a-space>
</template>

<script setup>
import { ref, reactive, onMounted, h } from 'vue'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import { PlusOutlined, SwapOutlined } from '@ant-design/icons-vue'
import { oncallApi, userApi } from '@/api'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const groups = ref([])
const userOptions = ref([])
const selectedGroup = ref(null)
const assignments = ref([])
const currentOcc = ref(null)
const createVisible = ref(false)
const handoverVisible = ref(false)
const saving = ref(false)
const form = reactive({ name:'', rotationMode:'weekly', memberIds:[], description:'', startDate:new Date(), _start:new Date() })
const handover = reactive({ toUserId:null, notes:'' })

const modeText = { daily:'每天', weekly:'每周', custom:'自定义' }
const occCols = [
  { title:'用户', key:'user' },
  { title:'角色', dataIndex:'shiftType', width:100, customRender:({text})=> text==='primary'?'主值班':'备份' },
  { title:'时间', key:'date' }
]

function formatDate(d) { return d ? dayjs(d).format('YYYY-MM-DD') : '-' }

async function loadGroups() {
  const r = await oncallApi.groups()
  groups.value = r.data || []
  if (groups.value.length && !selectedGroup.value) selectGroup(groups.value[0])
}
async function loadUsers() {
  const r = await userApi.list()
  userOptions.value = (r.data||[]).map(u=>({label:u.fullName||u.username, value:u.id}))
}
async function selectGroup(g) {
  selectedGroup.value = g
  const [o, c] = await Promise.all([
    oncallApi.assignments(g.id, { days: 60 }),
    oncallApi.current(g.id).catch(() => ({ data: null }))
  ])
  assignments.value = o.data || []
  currentOcc.value = c.data
}

function openCreate() {
  Object.assign(form, { name:'', rotationMode:'weekly', memberIds:[], _start:new Date(), description:'' })
  createVisible.value = true
}
async function saveGroup() {
  if (!form.name || !form.memberIds.length) { message.warning('请填写必填项'); return }
  form.startDate = dayjs(form._start).toDate()
  saving.value = true
  try {
    await oncallApi.createGroup(form)
    message.success('已创建')
    createVisible.value = false
    loadGroups()
  } finally { saving.value = false }
}
function openHandover() {
  handover.toUserId = null; handover.notes = ''
  handoverVisible.value = true
}
async function doHandover() {
  if (!handover.toUserId) { message.warning('请选择接班人'); return }
  await oncallApi.handover(selectedGroup.value.id, handover)
  message.success('已生成交接摘要')
  handoverVisible.value = false
}

onMounted(async () => { await loadUsers(); loadGroups() })
</script>
