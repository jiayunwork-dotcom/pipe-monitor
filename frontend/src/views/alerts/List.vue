<template>
  <div style="display:flex;flex-direction:column;gap:16px;">
    <a-card>
      <template #title>
        <span>告警趋势</span>
      </template>
      <div style="display:flex;gap:24px;align-items:flex-start;">
        <div style="flex:1;height:240px;">
          <v-chart :option="chartOption" autoresize />
        </div>
        <div style="width:200px;padding-top:20px;">
          <div style="text-align:center;">
            <div style="font-size:12px;color:#999;margin-bottom:8px;">本周 vs 上周</div>
            <div style="font-size:32px;font-weight:bold;color:#333;">
              {{ trendData.comparison?.thisWeekCount ?? 0 }}
            </div>
            <div style="font-size:12px;color:#999;margin-top:4px;">本周告警总数</div>
            <div style="margin-top:16px;display:flex;align-items:center;justify-content:center;gap:8px;">
              <span style="font-size:12px;color:#999;">环比</span>
              <a-tag :color="trendData.comparison?.isIncrease ? 'red' : 'green'">
                <template v-if="trendData.comparison?.lastWeekCount > 0">
                  {{ trendData.comparison?.isIncrease ? '↑' : '↓' }}
                  {{ Math.abs(trendData.comparison?.changePercent || 0).toFixed(1) }}%
                </template>
                <template v-else>
                  --
                </template>
              </a-tag>
            </div>
            <div style="font-size:12px;color:#999;margin-top:8px;">
              上周: {{ trendData.comparison?.lastWeekCount ?? 0 }} 条
            </div>
          </div>
        </div>
      </div>
    </a-card>

    <a-card>
      <template #title>
        <div style="display:flex;justify-content:space-between;align-items:center;">
          <span>告警事件</span>
          <a-space>
            <a-select v-model:value="viewMode" style="width:120px;" @change="reload">
              <a-select-option value="aggregated">聚合视图</a-select-option>
              <a-select-option value="list">列表视图</a-select-option>
            </a-select>
            <a-select v-model:value="filter.status" allow-clear placeholder="状态" style="width:140px;" @change="reload">
              <a-select-option value="triggered">已触发</a-select-option>
              <a-select-option value="acknowledged">已认领</a-select-option>
              <a-select-option value="resolved">已解决</a-select-option>
              <a-select-option value="closed">已关闭</a-select-option>
            </a-select>
            <a-select v-model:value="filter.severity" allow-clear placeholder="级别" style="width:120px;" @change="reload">
              <a-select-option value="info">提示</a-select-option>
              <a-select-option value="warning">警告</a-select-option>
              <a-select-option value="critical">严重</a-select-option>
            </a-select>
            <a-button @click="reload"><reload-outlined :spin="loading" /></a-button>
          </a-space>
        </div>
      </template>

      <template v-if="viewMode === 'aggregated'">
        <div v-if="batchSelected.length > 0" style="margin-bottom:12px;padding:8px 12px;background:#f5f5f5;border-radius:4px;display:flex;align-items:center;gap:12px;">
          <span style="color:#666;font-size:13px;">已选 {{ batchSelected.length }} 组</span>
          <a-button size="small" type="primary" @click="batchAck">批量认领</a-button>
          <a-button size="small" @click="batchResolve">批量关闭</a-button>
          <a-button size="small" type="link" @click="clearSelection">取消选择</a-button>
        </div>

        <div v-if="loading" style="text-align:center;padding:40px;">
          <a-spin />
        </div>

        <div v-else style="display:flex;flex-direction:column;gap:8px;">
          <div v-for="group in groupList" :key="group.groupKey"
               class="alert-group-card"
               :class="{ 'group-selected': selectedGroupKeys.has(group.groupKey) }">
            <div style="display:flex;align-items:center;gap:12px;padding:12px;cursor:pointer;"
                 @click="toggleGroup(group.groupKey)">
              <a-checkbox :checked="selectedGroupKeys.has(group.groupKey)"
                          @click.stop
                          @change="e => toggleGroupSelect(group, e.target.checked)" />
              <span style="font-size:16px;">
                {{ expandedGroups.has(group.groupKey) ? '▼' : '▶' }}
              </span>
              <a-tag :color="sevColor[group.severity]">{{ sevText[group.severity] }}</a-tag>
              <div style="flex:1;">
                <span style="font-weight:500;">{{ group.latestAlert?.title }}</span>
                <a-badge v-if="group.count > 1" :count="`共${group.count}条`"
                         style="margin-left:8px;" />
              </div>
              <span style="font-size:12px;color:#999;">
                {{ formatTime(group.latestAlert?.triggeredAt) }}
              </span>
              <a-tag v-if="group.count > 1" color="blue" style="margin-left:8px;">
                聚合组
              </a-tag>
              <a-space v-if="group.count === 1" style="margin-left:8px;">
                <a-button v-if="group.latestAlert?.status==='triggered'"
                          size="small" type="link"
                          @click.stop="ack(group.latestAlert)">认领</a-button>
                <a-button v-if="['triggered','acknowledged'].includes(group.latestAlert?.status)"
                          size="small" type="link"
                          @click.stop="resolve(group.latestAlert)">关闭</a-button>
              </a-space>
            </div>

            <div v-if="expandedGroups.has(group.groupKey)"
                 style="border-top:1px solid #f0f0f0;padding:12px 12px 12px 48px;background:#fafafa;">
              <div v-if="group.count > 1" style="margin-bottom:12px;">
                <a-space>
                  <a-button size="small" type="primary" @click="ackGroup(group)">一键认领全部</a-button>
                  <a-button size="small" @click="resolveGroup(group)">一键关闭全部</a-button>
                </a-space>
              </div>
              <div v-for="alert in group.alerts" :key="alert.id"
                   style="display:flex;align-items:center;gap:12px;padding:8px 0;border-bottom:1px dashed #e8e8e8;">
                <a-checkbox :checked="selectedAlertIds.has(alert.id)"
                            @change="e => toggleAlertSelect(alert.id, e.target.checked)" />
                <a-tag :color="sevColor[alert.severity]" style="zoom:0.9;">{{ sevText[alert.severity] }}</a-tag>
                <div style="flex:1;">
                  <div>{{ alert.title }}</div>
                  <div style="font-size:12px;color:#999;">
                    {{ alert.pipeline?.name || '-' }} · {{ formatTime(alert.triggeredAt) }}
                  </div>
                </div>
                <a-badge :status="statusMap[alert.status]?.color"
                         :text="statusMap[alert.status]?.text" />
                <a-space>
                  <a-button v-if="alert.status==='triggered'"
                            size="small" type="link"
                            @click="ack(alert)">认领</a-button>
                  <a-button v-if="['triggered','acknowledged'].includes(alert.status)"
                            size="small" type="link"
                            @click="resolve(alert)">关闭</a-button>
                  <a-button size="small" type="link" @click="showDetail(alert)">详情</a-button>
                </a-space>
              </div>
            </div>
          </div>
        </div>

        <a-pagination v-model:current="pagination.current"
                      v-model:pageSize="pagination.pageSize"
                      :total="pagination.total"
                      style="margin-top:16px;text-align:right;"
                      @change="onPage"
                      show-size-changer />
      </template>

      <a-table v-else :columns="cols" :data-source="list" :loading="loading"
               :pagination="pagination" @change="onPage"
               row-key="id" :expanded-row-keys="expandedKeys" @expand="onExpand">
        <template #expandedRowRender="{ record }">
          <a-descriptions :column="2" bordered size="small">
            <a-descriptions-item label="详细信息">{{ record.detail || '-' }}</a-descriptions-item>
            <a-descriptions-item label="SLA违约原因">{{ record.slaBreachReason || '-' }}</a-descriptions-item>
            <a-descriptions-item label="认领备注">{{ record.ackNote || '-' }}</a-descriptions-item>
            <a-descriptions-item label="解决备注">{{ record.resolveNote || '-' }}</a-descriptions-item>
          </a-descriptions>
          <div v-if="escalations[record.id]?.length" style="margin-top:12px;">
            <div style="font-weight:500;margin-bottom:8px;">升级记录</div>
            <a-timeline size="small">
              <a-timeline-item v-for="esc in escalations[record.id]" :key="esc.id">
                <div>
                  <a-tag :color="sevColor[esc.toSeverity]">{{ sevText[esc.fromSeverity] }} → {{ sevText[esc.toSeverity] }}</a-tag>
                  <span style="margin-left:8px;font-size:12px;color:#666;">{{ esc.reason }}</span>
                </div>
                <div style="font-size:12px;color:#999;margin-top:4px;">
                  触发者: {{ esc.triggeredBy }} · {{ formatTime(esc.triggeredAt) }}
                </div>
              </a-timeline-item>
            </a-timeline>
          </div>
        </template>
        <template #bodyCell="{column, record}">
          <template v-if="column.key==='sev'">
            <a-tag :color="sevColor[record.severity]">{{ sevText[record.severity] }}</a-tag>
          </template>
          <template v-else-if="column.key==='status'">
            <a-badge :status="statusMap[record.status]?.color" :text="statusMap[record.status]?.text" />
            <div style="font-size:11px;color:#999;">通知{{ record.notifyCount }}次</div>
          </template>
          <template v-else-if="column.key==='pipe'">
            <a v-if="record.pipelineId" @click="$router.push(`/pipelines/${record.pipelineId}`)">{{ record.pipeline?.name || '-' }}</a>
            <span v-else>-</span>
          </template>
          <template v-else-if="column.key==='op'">
            <template v-if="record.status==='triggered'">
              <a-button size="small" type="link" @click="ack(record)">认领</a-button>
            </template>
            <template v-if="record.status==='triggered'||record.status==='acknowledged'">
              <a-button size="small" type="link" @click="resolve(record)">关闭</a-button>
            </template>
          </template>
        </template>
      </a-table>
    </a-card>

    <a-modal v-model:open="ackVisible" title="认领告警" @ok="doAck">
      <a-textarea v-model:value="ackNote" :rows="4" placeholder="处理备注" />
    </a-modal>
    <a-modal v-model:open="resolveVisible" title="关闭告警" @ok="doResolve">
      <a-textarea v-model:value="resolveNote" :rows="4" placeholder="解决备注" />
    </a-modal>
    <a-modal v-model:open="detailVisible" title="告警详情" width="640px">
      <template v-if="currentAlert">
        <a-descriptions :column="2" bordered size="small">
          <a-descriptions-item label="标题">{{ currentAlert.title }}</a-descriptions-item>
          <a-descriptions-item label="级别">
            <a-tag :color="sevColor[currentAlert.severity]">{{ sevText[currentAlert.severity] }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="状态">
            <a-badge :status="statusMap[currentAlert.status]?.color" :text="statusMap[currentAlert.status]?.text" />
          </a-descriptions-item>
          <a-descriptions-item label="触发时间">{{ formatTime(currentAlert.triggeredAt) }}</a-descriptions-item>
          <a-descriptions-item label="管道">{{ currentAlert.pipeline?.name || '-' }}</a-descriptions-item>
          <a-descriptions-item label="通知次数">{{ currentAlert.notifyCount }}</a-descriptions-item>
          <a-descriptions-item label="认领人">{{ currentAlert.acknowledgedBy?.fullName || '-' }}</a-descriptions-item>
          <a-descriptions-item label="认领时间">{{ formatTime(currentAlert.acknowledgedAt) || '-' }}</a-descriptions-item>
          <a-descriptions-item label="详细内容" :span="2">{{ currentAlert.message }}</a-descriptions-item>
        </a-descriptions>

        <div v-if="escalations[currentAlert.id]?.length" style="margin-top:20px;">
          <a-divider orientation="left">升级记录</a-divider>
          <a-timeline>
            <a-timeline-item v-for="esc in escalations[currentAlert.id]" :key="esc.id"
                             :color="sevColor[esc.toSeverity]">
              <div style="font-weight:500;">
                {{ sevText[esc.fromSeverity] }} → {{ sevText[esc.toSeverity] }}
              </div>
              <div style="font-size:13px;color:#666;margin-top:4px;">{{ esc.reason }}</div>
              <div style="font-size:12px;color:#999;margin-top:4px;">
                触发: {{ esc.triggeredBy }}
                <span v-if="esc.notifiedLeader"> · 已通知主管</span>
              </div>
              <div style="font-size:12px;color:#999;">
                时间: {{ formatTime(esc.triggeredAt) }}
              </div>
            </a-timeline-item>
          </a-timeline>
        </div>
      </template>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted, computed, watch } from 'vue'
import { message } from 'ant-design-vue'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { alertApi } from '@/api'
import { useWebSocketStore } from '@/stores/ws'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent
} from 'echarts/components'

use([
  CanvasRenderer,
  LineChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent
])

const loading = ref(false)
const list = ref([])
const groupList = ref([])
const filter = reactive({ status: '', severity: '', days: 30 })
const pagination = reactive({ current:1, pageSize:20, total:0 })
const expandedKeys = ref([])
const expandedGroups = ref(new Set())
const selectedGroupKeys = ref(new Set())
const selectedAlertIds = ref(new Set())
const viewMode = ref('aggregated')
const ackVisible = ref(false)
const resolveVisible = ref(false)
const detailVisible = ref(false)
const ackNote = ref('')
const resolveNote = ref('')
const escalations = ref({})
const trendData = ref({ trend: {}, comparison: {} })
let curAlert = null
let currentAlert = ref(null)
let batchAction = ''

const ws = useWebSocketStore()

const sevColor = { info:'blue', warning:'orange', critical:'red' }
const sevText = { info:'提示', warning:'警告', critical:'严重' }
const statusMap = {
  triggered: { color:'error', text:'已触发' },
  acknowledged: { color:'processing', text:'已认领' },
  resolved: { color:'success', text:'已解决' },
  closed: { color:'default', text:'已关闭' },
  suppressed: { color:'warning', text:'抑制中' }
}
const cols = [
  { title:'级别', key:'sev', width:90 },
  { title:'状态', key:'status', width:120 },
  { title:'标题', dataIndex:'title', ellipsis:true },
  { title:'管道', key:'pipe', width:160, ellipsis:true },
  { title:'触发时间', dataIndex:'triggeredAt', width:170, customRender:({text})=>formatTime(text) },
  { title:'认领人', dataIndex:'acknowledgedBy', width:110, customRender:({text})=> text?.fullName || '-' },
  { title:'操作', key:'op', width:120, fixed:'right' }
]

const chartOption = computed(() => {
  const data = trendData.value.trend || {}
  return {
    tooltip: {
      trigger: 'axis',
      formatter: function(params) {
        let result = params[0].axisValue + '<br/>'
        let total = 0
        params.forEach(p => {
          result += `${p.marker}${p.seriesName}: ${p.value} 条<br/>`
          total += p.value
        })
        result += `<strong>总计: ${total} 条</strong>`
        return result
      }
    },
    legend: {
      data: ['提示', '警告', '严重'],
      bottom: 0
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '12%',
      top: '5%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: data.dates || []
    },
    yAxis: {
      type: 'value',
      minInterval: 1
    },
    series: [
      {
        name: '提示',
        type: 'line',
        stack: 'Total',
        smooth: true,
        data: data.infoCounts || [],
        itemStyle: { color: '#1890ff' },
        areaStyle: { opacity: 0.1 }
      },
      {
        name: '警告',
        type: 'line',
        stack: 'Total',
        smooth: true,
        data: data.warnCounts || [],
        itemStyle: { color: '#fa8c16' },
        areaStyle: { opacity: 0.1 }
      },
      {
        name: '严重',
        type: 'line',
        stack: 'Total',
        smooth: true,
        data: data.critCounts || [],
        itemStyle: { color: '#f5222d' },
        areaStyle: { opacity: 0.1 }
      }
    ]
  }
})

const batchSelected = computed(() => {
  const ids = new Set()
  for (const group of groupList.value) {
    if (selectedGroupKeys.value.has(group.groupKey)) {
      for (const alert of group.alerts) {
        ids.add(alert.id)
      }
    }
  }
  for (const id of selectedAlertIds.value) {
    ids.add(id)
  }
  return Array.from(ids)
})

function formatTime(t) {
  if (!t) return '-'
  return t.slice(0, 16).replace('T', ' ')
}

async function loadTrend() {
  try {
    const r = await alertApi.trend()
    trendData.value = r.data || {}
  } catch (e) {
    console.error('load trend failed', e)
  }
}

async function reload() {
  loading.value = true
  try {
    if (viewMode.value === 'aggregated') {
      const r = await alertApi.list({ ...filter, page: pagination.current, pageSize: pagination.pageSize, aggregated: true })
      groupList.value = r.data || []
      pagination.total = r.total || 0
    } else {
      const r = await alertApi.list({ ...filter, page: pagination.current, pageSize: pagination.pageSize })
      list.value = r.data || []
      pagination.total = r.total || 0
    }
  } finally { loading.value = false }
}

function onPage(p) {
  pagination.current = p.current
  pagination.pageSize = p.pageSize
  reload()
}

function onExpand(keys) { expandedKeys.value = keys }

function toggleGroup(key) {
  const set = new Set(expandedGroups.value)
  if (set.has(key)) {
    set.delete(key)
  } else {
    set.add(key)
  }
  expandedGroups.value = set
}

function toggleGroupSelect(group, checked) {
  const set = new Set(selectedGroupKeys.value)
  if (checked) {
    set.add(group.groupKey)
  } else {
    set.delete(group.groupKey)
  }
  selectedGroupKeys.value = set
}

function toggleAlertSelect(id, checked) {
  const set = new Set(selectedAlertIds.value)
  if (checked) {
    set.add(id)
  } else {
    set.delete(id)
  }
  selectedAlertIds.value = set
}

function clearSelection() {
  selectedGroupKeys.value = new Set()
  selectedAlertIds.value = new Set()
}

function ack(a) {
  curAlert = a
  ackNote.value = ''
  ackVisible.value = true
}

function resolve(a) {
  curAlert = a
  resolveNote.value = ''
  resolveVisible.value = true
}

function ackGroup(group) {
  const ids = group.alerts.map(a => a.id)
  batchAckIds(ids)
}

function resolveGroup(group) {
  const ids = group.alerts.map(a => a.id)
  batchResolveIds(ids)
}

function batchAck() {
  if (batchSelected.value.length === 0) {
    message.warning('请先选择告警')
    return
  }
  batchAction = 'ack'
  ackNote.value = ''
  ackVisible.value = true
}

function batchResolve() {
  if (batchSelected.value.length === 0) {
    message.warning('请先选择告警')
    return
  }
  batchAction = 'resolve'
  resolveNote.value = ''
  resolveVisible.value = true
}

async function batchAckIds(ids) {
  try {
    await alertApi.batchAcknowledge({ ids, note: ackNote.value })
    message.success('批量认领成功')
    reload()
  } catch (e) {
    message.error('操作失败')
  }
}

async function batchResolveIds(ids) {
  try {
    await alertApi.batchResolve({ ids, note: resolveNote.value })
    message.success('批量关闭成功')
    reload()
  } catch (e) {
    message.error('操作失败')
  }
}

async function doAck() {
  if (batchAction === 'ack') {
    await batchAckIds(batchSelected.value)
    batchAction = ''
    clearSelection()
  } else {
    await alertApi.acknowledge(curAlert.id, { note: ackNote.value })
    message.success('已认领')
  }
  ackVisible.value = false
  reload()
}

async function doResolve() {
  if (batchAction === 'resolve') {
    await batchResolveIds(batchSelected.value)
    batchAction = ''
    clearSelection()
  } else {
    await alertApi.resolve(curAlert.id, { note: resolveNote.value })
    message.success('已关闭')
  }
  resolveVisible.value = false
  reload()
}

async function showDetail(alert) {
  currentAlert.value = alert
  detailVisible.value = true
  try {
    const r = await alertApi.escalations(alert.id)
    escalations.value = { ...escalations.value, [alert.id]: r.data || [] }
  } catch (e) {
    console.error(e)
  }
}

const onNewAlert = () => {
  reload()
  loadTrend()
}

watch(viewMode, () => {
  pagination.current = 1
})

onMounted(() => {
  reload()
  loadTrend()
  ws.on('new_alert', onNewAlert)
  ws.on('alert_escalated', onNewAlert)
})
onUnmounted(() => {
  ws.off('new_alert', onNewAlert)
  ws.off('alert_escalated', onNewAlert)
})
</script>

<style scoped>
.alert-group-card {
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  background: #fff;
  transition: all 0.2s;
}
.alert-group-card:hover {
  border-color: #d9d9d9;
  box-shadow: 0 2px 8px rgba(0,0,0,0.06);
}
.group-selected {
  border-color: #1890ff;
  background: #e6f7ff;
}
</style>
