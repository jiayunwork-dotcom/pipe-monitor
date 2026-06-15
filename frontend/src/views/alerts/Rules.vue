<template>
  <a-card>
    <template #title>
      <div style="display:flex;justify-content:space-between;">
        <span>告警规则管理</span>
        <a-button v-if="auth.isAdmin" type="primary" @click="openCreate"><plus-outlined />新建规则</a-button>
      </div>
    </template>
    <a-table :columns="cols" :data-source="rules" :loading="loading" row-key="id" size="small">
      <template #bodyCell="{column, record}">
        <template v-if="column.key==='type'"><a-tag color="blue">{{ typeText[record.ruleType] }}</a-tag></template>
        <template v-else-if="column.key==='pipe'">{{ record.pipelineId ? '指定管道' : '全局' }}</template>
        <template v-else-if="column.key==='sev'"><a-tag :color="sevColor[record.severity]">{{ sevText[record.severity] }}</a-tag></template>
        <template v-else-if="column.key==='en'"><a-switch :checked="record.enabled" disabled /></template>
        <template v-else-if="column.key==='op' && auth.isAdmin">
          <a-popconfirm title="确认删除?" @confirm="()=>del(record.id)"><a-button type="link" danger size="small">删除</a-button></a-popconfirm>
        </template>
      </template>
    </a-table>

    <a-modal v-model:open="visible" title="新建告警规则" width="640px" @ok="save" :confirm-loading="saving">
      <a-form :model="form" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="12"><a-form-item label="规则名称" required><a-input v-model:value="form.name" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="适用管道"><a-select v-model:value="form.pipelineId" allow-clear :options="pipeOptions" placeholder="留空=全局" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="规则类型" required>
            <a-select v-model:value="form.ruleType">
              <a-select-option value="consecutive_fail">连续失败N次</a-select-option>
              <a-select-option value="duration_over_p95">耗时超过历史P95倍数</a-select-option>
              <a-select-option value="sla_imminent">SLA即将违约</a-select-option>
              <a-select-option value="sla_breached">SLA已违约</a-select-option>
              <a-select-option value="data_delay">数据延迟超阈值</a-select-option>
              <a-select-option value="pipeline_down">管道运行失败</a-select-option>
            </a-select>
          </a-form-item></a-col>
          <a-col :span="12"><a-form-item label="告警级别" required>
            <a-select v-model:value="form.severity">
              <a-select-option value="info">提示</a-select-option>
              <a-select-option value="warning">警告</a-select-option>
              <a-select-option value="critical">严重</a-select-option>
            </a-select>
          </a-form-item></a-col>
          <a-col :span="8" v-if="form.ruleType==='consecutive_fail'"><a-form-item label="连续失败次数"><a-input-number v-model:value="form.consecutiveFailN" :min="1" style="width:100%;" /></a-form-item></a-col>
          <a-col :span="8" v-if="form.ruleType==='duration_over_p95'"><a-form-item label="P95倍数"><a-input-number v-model:value="form.durationP95Multi" :min="1" :step="0.5" style="width:100%;" /></a-form-item></a-col>
          <a-col :span="8" v-if="form.ruleType==='data_delay'"><a-form-item label="延迟秒数"><a-input-number v-model:value="form.dataDelaySec" :min="0" style="width:100%;" /></a-form-item></a-col>
          <a-col :span="24"><a-form-item label="通知通道" required>
            <a-select mode="multiple" v-model:value="form._channels" :options="channelOptions" />
          </a-form-item></a-col>
          <a-col :span="12"><a-form-item label="抑制窗口(分钟)"><a-input-number v-model:value="form.suppressWindowMin" :min="0" style="width:100%;" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="通知值班人"><a-switch v-model:checked="form.notifyOnCall" /></a-form-item></a-col>
          <a-col :span="24"><a-form-item label="描述"><a-textarea v-model:value="form.description" :rows="2" /></a-form-item></a-col>
        </a-row>
      </a-form>
    </a-modal>
  </a-card>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { alertApi, pipelineApi } from '@/api'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const loading = ref(false)
const saving = ref(false)
const rules = ref([])
const pipeOptions = ref([])
const visible = ref(false)
const form = reactive({
  name:'', pipelineId:undefined, ruleType:'consecutive_fail', severity:'warning',
  consecutiveFailN:3, durationP95Multi:2.0, dataDelaySec:1800,
  _channels:['feishu','email'], channels:'["feishu","email"]',
  suppressWindowMin:60, notifyOnCall:true, description:'', enabled:true
})

const typeText = {
  consecutive_fail:'连续失败', duration_over_p95:'耗时异常',
  sla_imminent:'SLA预警', sla_breached:'SLA违约',
  data_delay:'数据延迟', pipeline_down:'管道失败', custom:'自定义'
}
const sevColor = { info:'blue', warning:'orange', critical:'red' }
const sevText = { info:'提示', warning:'警告', critical:'严重' }
const channelOptions = [
  {label:'飞书', value:'feishu'},{label:'钉钉', value:'dingtalk'},
  {label:'Slack', value:'slack'},{label:'邮件', value:'email'},{label:'自定义Webhook', value:'custom_webhook'}
]
const cols = [
  { title:'名称', dataIndex:'name', ellipsis:true },
  { title:'类型', key:'type', width:110 },
  { title:'管道', key:'pipe', width:100 },
  { title:'级别', key:'sev', width:90 },
  { title:'抑制窗口', dataIndex:'suppressWindowMin', width:110, customRender:({text})=>`${text}分钟` },
  { title:'通知值班', dataIndex:'notifyOnCall', width:100, customRender:({text})=>text?'是':'否' },
  { title:'启用', key:'en', width:80 },
  { title:'操作', key:'op', width:90, fixed:'right' }
]

async function reload() {
  loading.value = true
  try {
    const r = await alertApi.rules()
    rules.value = r.data || []
  } finally { loading.value = false }
}
function openCreate() {
  Object.assign(form, { name:'', pipelineId:undefined, ruleType:'consecutive_fail', severity:'warning', _channels:['feishu','email'], description:'' })
  visible.value = true
}
async function save() {
  if (!form.name) { message.warning('请填写规则名称'); return }
  form.channels = JSON.stringify(form._channels)
  saving.value = true
  try {
    await alertApi.createRule(form)
    message.success('已创建')
    visible.value = false
    reload()
  } finally { saving.value = false }
}
async function del(id) {
  await alertApi.deleteRule(id)
  message.success('已删除')
  reload()
}

onMounted(async () => {
  const pr = await pipelineApi.list({ pageSize: 500 })
  pipeOptions.value = (pr.data||[]).map(p=>({label:p.name, value:p.id}))
  reload()
})
</script>
