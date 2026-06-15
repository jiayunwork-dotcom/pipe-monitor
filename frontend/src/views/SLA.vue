<template>
  <a-space direction="vertical" style="width:100%;" :size="16">
    <a-card title="SLA概览">
      <a-row :gutter="16">
        <a-col :span="6"><a-statistic title="监控管道数" :value="pipelines.length" /></a-col>
        <a-col :span="6"><a-statistic title="规则总数" :value="slaRules.length" /></a-col>
        <a-col :span="6"><a-statistic title="本月违约次数" :value="monthlyBreach" :value-style="{color:'#ff4d4f'}" /></a-col>
        <a-col :span="6"><a-statistic title="整体达成率" :value="overallRate" suffix="%" :value-style="{color:'#52c41a'}" /></a-col>
      </a-row>
    </a-card>

    <a-card title="SLA月度报告">
      <a-table :columns="reportCols" :data-source="reports" row-key="id" size="small" :pagination="{pageSize:10}">
        <template #bodyCell="{column, record}">
          <template v-if="column.key==='pipe'">{{ record.pipeline?.name || record.pipelineId }}</template>
          <template v-else-if="column.key==='rate'">
            <a-progress :percent="record.achievementRate" :status="record.achievementRate>=98?'success':record.achievementRate>=90?'normal':'exception'" size="small" />
          </template>
          <template v-else-if="column.key==='durs'">
            <div>平均: {{ record.avgDurationSec }}s | P50: {{ record.p50DurationSec }}s</div>
            <div style="color:#999;">P95: {{ record.p95DurationSec }}s | 最大: {{ record.maxDurationSec }}s</div>
          </template>
        </template>
      </a-table>
    </a-card>
  </a-space>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { slaApi, pipelineApi } from '@/api'

const pipelines = ref([])
const slaRules = ref([])
const reports = ref([])
const monthlyBreach = ref(0)

const reportCols = [
  { title:'月份', dataIndex:'reportMonth', width:100 },
  { title:'管道', key:'pipe', ellipsis:true },
  { title:'总运行', dataIndex:'totalRuns', width:80 },
  { title:'成功', dataIndex:'successCount', width:80 },
  { title:'违约', dataIndex:'breachCount', width:80 },
  { title:'达成率', key:'rate', width:200 },
  { title:'耗时统计', key:'durs', width:320 },
  { title:'平均延迟', dataIndex:'avgDelaySec', width:100, customRender:({text})=>text?`${text}s`:'-' }
]

const overallRate = computed(() => {
  if (!reports.value.length) return 0
  const sum = reports.value.reduce((a,r)=>a + (r.achievementRate||0), 0)
  return Math.round(sum / reports.value.length * 100) / 100
})

onMounted(async () => {
  const [p, r] = await Promise.all([
    pipelineApi.list({ pageSize: 500 }),
    slaApi.rules({}),
    slaApi.monthlyReports({})
  ])
  pipelines.value = p.data || []
  slaRules.value = r.data || []
  const mr = await slaApi.monthlyReports({ month: new Date().toISOString().slice(0,7) })
  reports.value = mr.data || []
  monthlyBreach.value = reports.value.reduce((a,r)=>a + (r.breachCount||0), 0)
})
</script>
