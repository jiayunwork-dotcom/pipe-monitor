<template>
  <a-card>
    <template #title>
      <div style="display:flex;justify-content:space-between;">
        <span>管道列表</span>
        <a-space>
          <a-input-search placeholder="搜索名称/编码" allow-clear style="width:220px;" v-model:value="keyword" @change="reload" />
          <a-select v-model:value="filter.status" allow-clear placeholder="状态" style="width:120px;" @change="reload">
            <a-select-option value="active">启用</a-select-option>
            <a-select-option value="paused">暂停</a-select-option>
            <a-select-option value="archived">归档</a-select-option>
          </a-select>
          <a-button v-if="auth.isAdmin" type="primary" @click="openCreate"><plus-outlined />新建</a-button>
        </a-space>
      </div>
    </template>
    <a-table :columns="cols" :data-source="list" :loading="loading" :pagination="pagination" @change="onPageChange" row-key="id">
      <template #bodyCell="{column, record}">
        <template v-if="column.key==='status'">
          <a-tag :color="statusColor[record.status]">{{ statusText[record.status] }}</a-tag>
        </template>
        <template v-else-if="column.key==='freq'">
          <a-tag color="blue">{{ freqText[record.scheduleFreq] || record.scheduleFreq }}</a-tag>
        </template>
        <template v-else-if="column.key==='owner'">{{ record.owner?.fullName || '-' }}</template>
        <template v-else-if="column.key==='action'">
          <a-space>
            <a-button size="small" type="link" @click="$router.push(`/pipelines/${record.id}`)">详情</a-button>
            <a-button size="small" type="link" @click="$router.push({path:'/dag',query:{startPipelineId:record.id}})">依赖图</a-button>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal v-model:open="createVisible" title="新建管道" width="720px" @ok="doCreate" :confirm-loading="saving">
      <a-form :model="form" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="12"><a-form-item label="名称" required><a-input v-model:value="form.name" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="编码(唯一)" required><a-input v-model:value="form.code" placeholder="如ods_order_extract" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="调度频率">
            <a-select v-model:value="form.scheduleFreq" @change="onFreqChange">
              <a-select-option value="hourly">每小时</a-select-option>
              <a-select-option value="daily">每天</a-select-option>
              <a-select-option value="weekly">每周</a-select-option>
              <a-select-option value="custom">自定义Cron</a-select-option>
            </a-select>
          </a-form-item></a-col>
          <a-col :span="12"><a-form-item label="Cron表达式" v-if="form.scheduleFreq==='custom'"><a-input v-model:value="form.cronExpression" placeholder="0 0 3 * * ?" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="所属团队"><a-input v-model:value="form.team" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="数据域"><a-input v-model:value="form.dataDomain" placeholder="如 交易域/用户域" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="负责人">
            <a-select v-model:value="form.ownerId" :options="userOptions" />
          </a-form-item></a-col>
          <a-col :span="12"><a-form-item label="预期运行时长(秒)"><a-input-number v-model:value="form.expectedRunSec" :min="0" style="width:100%;" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="数据源说明"><a-input v-model:value="form.sourceDetail" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="目标说明"><a-input v-model:value="form.targetDetail" /></a-form-item></a-col>
          <a-col :span="24"><a-form-item label="上游依赖"><a-select mode="multiple" v-model:value="form.upstreamIds" :options="pipeOptions" placeholder="选择先完成的管道" /></a-form-item></a-col>
          <a-col :span="24"><a-form-item label="描述"><a-textarea v-model:value="form.description" :rows="3" /></a-form-item></a-col>
        </a-row>
      </a-form>
    </a-modal>
  </a-card>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { pipelineApi, userApi } from '@/api'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const loading = ref(false)
const saving = ref(false)
const keyword = ref('')
const filter = reactive({ status: '' })
const list = ref([])
const userOptions = ref([])
const pipeOptions = ref([])
const pagination = reactive({ current:1, pageSize:20, total:0 })
const createVisible = ref(false)
const form = reactive({
  name:'', code:'', scheduleFreq:'daily', cronExpression:'', team:'', dataDomain:'',
  ownerId:null, expectedRunSec:300, sourceDetail:'', targetDetail:'', description:'', upstreamIds:[], status:'active'
})

const statusColor = { active:'green', paused:'orange', archived:'default' }
const statusText = { active:'启用', paused:'暂停', archived:'归档' }
const freqText = { hourly:'每小时', daily:'每天', weekly:'每周', monthly:'每月', custom:'自定义' }

const cols = [
  { title:'名称', dataIndex:'name', key:'name', ellipsis:true },
  { title:'编码', dataIndex:'code', key:'code', width:160 },
  { title:'状态', key:'status', width:90 },
  { title:'调度', key:'freq', width:100 },
  { title:'团队', dataIndex:'team', width:120 },
  { title:'数据域', dataIndex:'dataDomain', width:120 },
  { title:'负责人', key:'owner', width:110 },
  { title:'创建时间', dataIndex:'createdAt', width:170, customRender:({text})=> text?.slice(0,16).replace('T',' ') },
  { title:'操作', key:'action', fixed:'right', width:140 }
]

async function reload() {
  loading.value = true
  try {
    const params = { page: pagination.current, pageSize: pagination.pageSize, ...filter }
    if (keyword.value) params.search = keyword.value
    const res = await pipelineApi.list(params)
    list.value = res.data || []
    pagination.total = res.total || 0
  } finally { loading.value = false }
}

onPageChange(p) { pagination.current = p.current; pagination.pageSize = p.pageSize; reload() }

async function loadUsers() {
  const r = await userApi.list()
  userOptions.value = (r.data||[]).map(u=>({label:u.fullName||u.username, value:u.id}))
}

async function loadPipes() {
  const r = await pipelineApi.list({ pageSize: 500 })
  pipeOptions.value = (r.data||[]).map(p=>({label:`${p.name}(${p.code})`, value:p.id}))
}

function openCreate() {
  Object.assign(form, { name:'', code:'', scheduleFreq:'daily', team:'', dataDomain:'', ownerId:null, upstreamIds:[], description:'' })
  loadUsers(); loadPipes()
  createVisible.value = true
}
function onFreqChange() { if (form.scheduleFreq !== 'custom') form.cronExpression = '' }

async function doCreate() {
  if (!form.name || !form.code) { message.warning('请填写必填项'); return }
  saving.value = true
  try {
    await pipelineApi.create(form)
    message.success('创建成功')
    createVisible.value = false
    reload()
  } finally { saving.value = false }
}

onMounted(reload)
</script>
