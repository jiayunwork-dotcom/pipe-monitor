<template>
  <div>
    <a-row :gutter="16" style="margin-bottom:24px;">
      <a-col :span="4"><a-card class="stat-card"><a-statistic title="管道总数" :value="stats.totalPipelines" :value-style="{color:'#1677ff'}"><template #prefix><apartment-outlined /></template></a-statistic></a-card></a-col>
      <a-col :span="4"><a-card class="stat-card"><a-statistic title="今日运行" :value="stats.todayRuns" /></a-card></a-col>
      <a-col :span="4"><a-card class="stat-card"><a-statistic title="成功" :value="stats.todaySuccess" :value-style="{color:'#52c41a'}"><template #prefix><check-circle-outlined /></template></a-statistic></a-card></a-col>
      <a-col :span="4"><a-card class="stat-card"><a-statistic title="失败" :value="stats.todayFailed" :value-style="{color:'#ff4d4f'}"><template #prefix><close-circle-outlined /></template></a-statistic></a-card></a-col>
      <a-col :span="4"><a-card class="stat-card"><a-statistic title="SLA违约" :value="stats.todaySLABreach" :value-style="{color:'#fa8c16'}" /></a-card></a-col>
      <a-col :span="4"><a-card class="stat-card"><a-statistic title="未处理告警" :value="stats.openAlerts" :value-style="{color:'#ff4d4f'}"><template #prefix><warning-outlined /></template></a-statistic></a-card></a-col>
    </a-row>

    <a-card style="margin-bottom:24px;">
      <template #title>
        <div style="display:flex;justify-content:space-between;align-items:center;">
          <span>管道健康大盘</span>
          <a-space>
            <a-select v-model:value="filterTeam" placeholder="团队" allow-clear style="width:140px;" @change="applyFilter">
              <a-select-option v-for="t in teams" :key="t" :value="t">{{ t }}</a-select-option>
            </a-select>
            <a-select v-model:value="filterDomain" placeholder="数据域" allow-clear style="width:140px;" @change="applyFilter">
              <a-select-option v-for="d in domains" :key="d" :value="d">{{ d }}</a-select-option>
            </a-select>
            <a-select v-model:value="filterFreq" placeholder="调度频率" allow-clear style="width:140px;" @change="applyFilter">
              <a-select-option value="hourly">每小时</a-select-option>
              <a-select-option value="daily">每天</a-select-option>
              <a-select-option value="weekly">每周</a-select-option>
            </a-select>
            <a-button type="primary" @click="loadData"><reload-outlined :spin="loading" />刷新</a-button>
          </a-space>
        </div>
      </template>
      <a-empty v-if="filteredPipelines.length === 0" description="暂无管道数据" />
      <a-row v-else :gutter="[16,16]">
        <a-col :xs="24" :sm="12" :md="8" :lg="6" v-for="p in filteredPipelines" :key="p.pipelineId">
          <a-card class="dashboard-pipeline-card" :class="`h-${p.health}`" size="small" @click="goDetail(p.pipelineId)">
            <a-row justify="space-between" align="middle">
              <a-col :span="20">
                <a-typography-text strong>{{ p.name }}</a-typography-text>
                <div style="font-size:12px;color:#999;">{{ p.code }}</div>
              </a-col>
              <a-col :span="4" style="text-align:right;">
                <a-badge :status="statusMap[p.health]?.color || 'default'" :text="statusMap[p.health]?.text" />
              </a-col>
            </a-row>
            <a-divider style="margin:8px 0;" />
            <a-row :gutter="8">
              <a-col :span="12"><div style="font-size:12px;color:#666;">团队</div><div style="font-size:13px;">{{ p.team || '-' }}</div></a-col>
              <a-col :span="12"><div style="font-size:12px;color:#666;">数据域</div><div style="font-size:13px;">{{ p.dataDomain || '-' }}</div></a-col>
              <a-col :span="24" style="margin-top:8px;">
                <div style="font-size:12px;color:#666;">最近运行: {{ p.lastStatus }} @ {{ p.lastRunAt ? formatTime(p.lastRunAt) : '无记录' }}</div>
              </a-col>
              <a-col :span="24"><a-tag v-if="p.sla === 'breached'" color="red">SLA违约</a-tag><a-tag v-else-if="p.sla === 'predicted_breach'" color="orange">预测违约</a-tag></a-col>
            </a-row>
          </a-card>
        </a-col>
      </a-row>
    </a-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, reactive } from 'vue'
import { useRouter } from 'vue-router'
import dayjs from 'dayjs'
import { dashboardApi } from '@/api'
import { useWebSocketStore } from '@/stores/ws'
import {
  ApartmentOutlined, CheckCircleOutlined, CloseCircleOutlined, WarningOutlined, ReloadOutlined
} from '@ant-design/icons-vue'

const router = useRouter()
const ws = useWebSocketStore()
const loading = ref(false)
const stats = reactive({ totalPipelines:0, todayRuns:0, todaySuccess:0, todayFailed:0, todaySLABreach:0, openAlerts:0, runningNow:0, pipelines: [] })
const filterTeam = ref('')
const filterDomain = ref('')
const filterFreq = ref('')

const statusMap = {
  green: { color: 'success', text: '正常' },
  yellow: { color: 'warning', text: '警告' },
  red: { color: 'error', text: '告警' },
  blue: { color: 'processing', text: '运行中' },
  gray: { color: 'default', text: '未调度' }
}

const teams = computed(() => [...new Set((stats.pipelines||[]).map(p=>p.team).filter(Boolean))])
const domains = computed(() => [...new Set((stats.pipelines||[]).map(p=>p.dataDomain).filter(Boolean))])
const filteredPipelines = computed(() => (stats.pipelines||[]).filter(p =>
  (!filterTeam.value || p.team === filterTeam.value) &&
  (!filterDomain.value || p.dataDomain === filterDomain.value)
))

async function loadData() {
  loading.value = true
  try {
    const res = await dashboardApi.overview()
    Object.assign(stats, res.data)
  } finally {
    loading.value = false
  }
}

function applyFilter() {}
function goDetail(id) { router.push(`/pipelines/${id}`) }
function formatTime(t) { return t ? dayjs(t).format('MM-DD HH:mm') : '-' }

const onStatusChange = payload => {
  const idx = stats.pipelines.findIndex(p => p.pipelineId === payload.pipelineId)
  if (idx >= 0) {
    stats.pipelines[idx] = { ...stats.pipelines[idx], ...payload }
    if (payload.health === 'red' || payload.health === 'yellow') {
      stats.todayFailed = (stats.todayFailed || 0) + 0
    }
  }
}
const onRunChange = payload => { stats.runningNow = (stats.runningNow||0) + 0 }
const onAlert = payload => { stats.openAlerts = (stats.openAlerts||0) + 1 }

onMounted(() => {
  loadData()
  ws.on('pipeline_status_change', onStatusChange)
  ws.on('run_status_change', onRunChange)
  ws.on('new_alert', onAlert)
  ws.on('connected', () => {
    loadData()
  })
})
</script>
