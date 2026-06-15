<template>
  <a-space direction="vertical" style="width:100%;" :size="16">
    <a-card>
      <a-row justify="space-between" align="middle">
        <a-col :span="18">
          <a-typography-title :level="3" style="margin:0;">
            {{ pipeline?.name }}
            <a-tag style="margin-left:8px;" :color="statusColor[pipeline?.status]">{{ statusText[pipeline?.status] }}</a-tag>
            <a-tag color="blue">{{ freqText[pipeline?.scheduleFreq] }}</a-tag>
            <a-tag v-if="pipeline?.dataDomain" color="purple">{{ pipeline.dataDomain }}</a-tag>
          </a-typography-title>
          <div style="color:#999;margin-top:4px;">编码: {{ pipeline?.code }} | 团队: {{ pipeline?.team || '-' }} | 负责人: {{ pipeline?.owner?.fullName || '-' }}</div>
          <div style="color:#666;margin-top:8px;">{{ pipeline?.description || '暂无描述' }}</div>
        </a-col>
        <a-col :span="6" style="text-align:right;">
          <a-space>
            <a-button @click="$router.back()"><left-outlined />返回</a-button>
            <a-button @click="$router.push({path:'/dag',query:{startPipelineId:id}})">依赖图</a-button>
          </a-space>
        </a-col>
      </a-row>
    </a-card>

    <a-row :gutter="16">
      <a-col :span="12">
        <a-card title="基础信息">
          <a-descriptions :column="1" bordered size="small">
            <a-descriptions-item label="数据源">{{ pipeline?.sourceDetail || '-' }}</a-descriptions-item>
            <a-descriptions-item label="写入目标">{{ pipeline?.targetDetail || '-' }}</a-descriptions-item>
            <a-descriptions-item label="调度表达式">{{ pipeline?.cronExpression || '使用频率默认配置' }}</a-descriptions-item>
            <a-descriptions-item label="预期运行时长">{{ pipeline?.expectedRunSec }} 秒</a-descriptions-item>
            <a-descriptions-item label="上游依赖">
              <a-tag v-for="d in dependencies" :key="d.id" color="blue">{{ d.upstream?.name || d.upstreamId }}</a-tag>
              <span v-if="!dependencies.length">-</span>
            </a-descriptions-item>
            <a-descriptions-item label="下游管道">
              <a-tag v-for="d in downstream" :key="d.id" color="orange">{{ d.pipeline?.name || d.pipelineId }}</a-tag>
              <span v-if="!downstream.length">-</span>
            </a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
      <a-col :span="12">
        <a-card title="SLA统计">
          <a-row :gutter="12">
            <a-col :span="8"><a-statistic title="总运行" :value="slaStats?.totalRuns || 0" /></a-col>
            <a-col :span="8"><a-statistic title="成功" :value="slaStats?.successCount || 0" :value-style="{color:'#52c41a'}" /></a-col>
            <a-col :span="8"><a-statistic title="达成率" :value="slaStats?.achievementRate || 0" suffix="%" :value-style="{color:'#1677ff'}" /></a-col>
            <a-col :span="8"><a-statistic title="平均耗时" :value="slaStats?.avgSec || 0" suffix="s" /></a-col>
            <a-col :span="8"><a-statistic title="P50" :value="slaStats?.p50Sec || 0" suffix="s" /></a-col>
            <a-col :span="8"><a-statistic title="P95" :value="slaStats?.p95Sec || 0" suffix="s" /></a-col>
          </a-row>
        </a-card>
      </a-col>
    </a-row>

    <a-card title="SLA规则列表" :extra="auth.isAdmin ? h(aButton, {type:'primary', onClick:openSLA}, ()=>'新增规则') : null">
      <a-table :columns="slaCols" :data-source="slaRules" row-key="id" size="small" :pagination="false">
        <template #bodyCell="{column, record}">
          <template v-if="column.key==='type'"><a-tag>{{ slaTypeText[record.ruleType] }}</a-tag></template>
          <template v-else-if="column.key==='threshold'">{{ formatThreshold(record) }}</template>
          <template v-else-if="column.key==='enabled'"><a-switch :checked="record.enabled" disabled /></template>
          <template v-else-if="column.key==='op' && auth.isAdmin">
            <a-popconfirm title="确认删除?" @confirm="()=>delSLA(record.id)"><a-button type="link" danger size="small">删除</a-button></a-popconfirm>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-card title="运行历史甘特图 (最近30天)">
      <div style="overflow-x:auto;padding:12px 0;">
        <div style="display:flex;min-width:1200px;">
          <div style="width:180px;flex-shrink:0;padding-right:12px;">
            <div v-for="g in ganttGroups" :key="g.day" style="height:40px;line-height:40px;font-weight:600;">{{ g.day }}</div>
          </div>
          <div style="flex:1;position:relative;border-left:1px solid #eee;">
            <div style="display:flex;position:sticky;top:0;background:#fff;border-bottom:1px solid #eee;z-index:2;">
              <div v-for="h in 24" :key="h" :style="{width:(100/24)+'%',textAlign:'center',height:30,lineHeight:'30px',fontSize:12,color:'#999',borderRight:'1px dashed #eee'}">{{ (h-1).toString().padStart(2,'0') }}:00</div>
            </div>
            <div v-for="g in ganttGroups" :key="g.day+'-rows'" style="position:relative;display:flex;align-items:center;height:40px;border-bottom:1px solid #f5f5f5;">
              <div v-for="(r,idx) in g.items" :key="r.id"
                class="gantt-bar"
                :class="`s-${r.status}`"
                :style="barStyle(r)"
                :title="`${r.status} | ${r.durationSec}秒 | ${formatTime(r.start)} - ${r.end ? formatTime(r.end):'进行中'}`">
              </div>
            </div>
          </div>
        </div>
      </div>
    </a-card>
  </a-space>

  <a-modal v-model:open="slaVisible" title="新增SLA规则" width="600px" @ok="saveSLA" :confirm-loading="saving">
    <a-form :model="slaForm" layout="vertical">
      <a-form-item label="规则名称" required><a-input v-model:value="slaForm.name" /></a-form-item>
      <a-form-item label="规则类型" required>
        <a-select v-model:value="slaForm.ruleType">
          <a-select-option value="finish_by_time">截止时间前完成</a-select-option>
          <a-select-option value="max_duration">单次最大耗时</a-select-option>
          <a-select-option value="max_delay">数据延迟阈值</a-select-option>
          <a-select-option value="max_consecutive_fail">连续失败次数</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="日期类型">
        <a-select v-model:value="slaForm.dateType">
          <a-select-option value="any">所有日期</a-select-option>
          <a-select-option value="workday">仅工作日</a-select-option>
          <a-select-option value="holiday">仅节假日</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item v-if="slaForm.ruleType==='finish_by_time'" label="截止时间" required><a-time-picker v-model:value="slaForm._deadline" format="HH:mm" /></a-form-item>
      <a-form-item v-if="slaForm.ruleType==='max_duration'" label="最大耗时(秒)" required><a-input-number v-model:value="slaForm.maxDurationSec" :min="0" style="width:100%;" /></a-form-item>
      <a-form-item v-if="slaForm.ruleType==='max_consecutive_fail'" label="连续失败次数" required><a-input-number v-model:value="slaForm.maxConsecutiveFail" :min="1" style="width:100%;" /></a-form-item>
      <a-form-item label="告警级别">
        <a-select v-model:value="slaForm.alertSeverity">
          <a-select-option value="info">提示</a-select-option>
          <a-select-option value="warning">警告</a-select-option>
          <a-select-option value="critical">严重</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="通知通道">
        <a-select mode="multiple" v-model:value="slaForm._channels" :options="channelOptions" />
      </a-form-item>
      <a-form-item label="描述"><a-textarea v-model:value="slaForm.description" :rows="2" /></a-form-item>
    </a-form>
  </a-modal>
</template>

<script setup>
import { ref, reactive, onMounted, computed, h } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import { LeftOutlined } from '@ant-design/icons-vue'
import { pipelineApi, slaApi } from '@/api'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const auth = useAuthStore()
const id = computed(() => +route.params.id)
const pipeline = ref(null)
const dependencies = ref([])
const downstream = ref([])
const slaStats = ref(null)
const slaRules = ref([])
const history = ref([])
const saving = ref(false)
const slaVisible = ref(false)
const slaForm = reactive({
  pipelineId: id.value, name:'', ruleType:'finish_by_time', dateType:'any',
  finishDeadlineTime:'08:00', maxDurationSec:3600, maxConsecutiveFail:3,
  alertSeverity:'warning', alertChannels:'["feishu","email"]', description:'',
  _channels:['feishu','email'], _deadline:dayjs('08:00','HH:mm').toDate()
})

const statusColor = { active:'green', paused:'orange', archived:'default' }
const statusText = { active:'启用', paused:'暂停', archived:'归档' }
const freqText = { hourly:'每小时', daily:'每天', weekly:'每周', monthly:'每月', custom:'自定义' }
const slaTypeText = { finish_by_time:'截止时间', max_duration:'最大耗时', max_delay:'最大延迟', max_consecutive_fail:'连续失败', min_success_rate:'成功率' }
const channelOptions = [
  {label:'飞书Webhook', value:'feishu'},{label:'钉钉Webhook', value:'dingtalk'},
  {label:'Slack', value:'slack'},{label:'邮件', value:'email'}
]
const slaCols = [
  {title:'名称', dataIndex:'name'},{title:'类型', key:'type', width:120},
  {title:'阈值', key:'threshold', width:200},{title:'日期类型', dataIndex:'dateType', width:100},
  {title:'级别', dataIndex:'alertSeverity', width:90},{title:'启用', key:'enabled', width:80},
  {title:'操作', key:'op', width:80}
]

function formatThreshold(r) {
  if (r.ruleType==='finish_by_time') return r.finishDeadlineTime + '前完成'
  if (r.ruleType==='max_duration') return `${r.maxDurationSec}秒`
  if (r.ruleType==='max_consecutive_fail') return `${r.maxConsecutiveFail}次`
  return '-'
}
function formatTime(t) { return t ? dayjs(t).format('HH:mm:ss') : '-' }

const dayStart = dayjs().startOf('day')
function barStyle(r) {
  const s = dayjs(r.start)
  const dayOfItem = s.startOf('day')
  const offsetMin = s.diff(dayOfItem, 'minute')
  const left = (offsetMin / 1440 * 100) + '%'
  let durationMin = 30
  if (r.end) durationMin = Math.max(10, dayjs(r.end).diff(s, 'minute'))
  const width = (durationMin / 1440 * 100) + '%'
  return { left, width, position:'absolute', top:'8px' }
}
const ganttGroups = computed(() => {
  const m = {}
  history.value.forEach(r => {
    const day = dayjs(r.start).format('YYYY-MM-DD')
    if (!m[day]) m[day] = { day, items: [] }
    m[day].items.push(r)
  })
  return Object.values(m).sort((a,b) => b.day.localeCompare(a.day)).slice(0, 30)
})

async function loadAll() {
  const [detail, slaR, slaS, hist] = await Promise.all([
    pipelineApi.get(id.value),
    slaApi.rules({ pipelineId: id.value }),
    slaApi.stats({ pipelineId: id.value, days: 30 }),
    pipelineApi.runHistory(id.value, { days: 30 })
  ])
  pipeline.value = detail.pipeline
  dependencies.value = detail.dependencies || []
  downstream.value = detail.downstream || []
  slaRules.value = slaR.data || []
  slaStats.value = slaS.data
  history.value = hist.data || []
}

function openSLA() {
  Object.assign(slaForm, { name:'', ruleType:'finish_by_time', _channels:['feishu','email'],
    finishDeadlineTime:'08:00', _deadline: dayjs('08:00','HH:mm').toDate() })
  slaVisible.value = true
}

async function saveSLA() {
  if (!slaForm.name) { message.warning('请填写规则名称'); return }
  if (slaForm.ruleType === 'finish_by_time') {
    slaForm.finishDeadlineTime = dayjs(slaForm._deadline).format('HH:mm')
  }
  slaForm.alertChannels = JSON.stringify(slaForm._channels)
  slaForm.pipelineId = id.value
  saving.value = true
  try {
    await slaApi.createRule(slaForm)
    message.success('已创建')
    slaVisible.value = false
    loadAll()
  } finally { saving.value = false }
}

async function delSLA(ruleId) {
  await slaApi.deleteRule(ruleId)
  message.success('已删除')
  loadAll()
}

onMounted(loadAll)
</script>
