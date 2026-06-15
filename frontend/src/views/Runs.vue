<template>
  <a-card>
    <template #title>
      <div style="display:flex;justify-content:space-between;">
        <span>运行记录</span>
        <a-space>
          <a-select v-model:value="filter.pipelineId" allow-clear placeholder="管道" style="width:220px;" @change="reload">
            <a-select-option v-for="p in pipes" :key="p.id" :value="p.id">{{ p.name }} ({{ p.code }})</a-select-option>
          </a-select>
          <a-select v-model:value="filter.status" allow-clear placeholder="状态" style="width:140px;" @change="reload">
            <a-select-option value="success">成功</a-select-option>
            <a-select-option value="running">运行中</a-select-option>
            <a-select-option value="failed">失败</a-select-option>
            <a-select-option value="timeout">超时</a-select-option>
            <a-select-option value="pending">等待中</a-select-option>
            <a-select-option value="cancelled">取消</a-select-option>
          </a-select>
          <a-select v-model:value="filter.days" style="width:140px;" @change="reload">
            <a-select-option :value="7">最近7天</a-select-option>
            <a-select-option :value="30">最近30天</a-select-option>
            <a-select-option :value="90">最近90天</a-select-option>
          </a-select>
          <a-button @click="reload"><reload-outlined :spin="loading" /></a-button>
        </a-space>
      </div>
    </template>
    <a-table :columns="cols" :data-source="list" :loading="loading" :pagination="pagination" @change="onPage" row-key="id">
      <template #bodyCell="{column, record}">
        <template v-if="column.key==='pipe'">
          <a @click="$router.push(`/pipelines/${record.pipelineId}`)">{{ record.pipeline?.name || '-' }}</a>
          <div style="font-size:12px;color:#999;">{{ record.pipeline?.code }}</div>
        </template>
        <template v-else-if="column.key==='status'">
          <a-badge :status="statusMap[record.status]?.color" :text="statusMap[record.status]?.text" />
        </template>
        <template v-else-if="column.key==='sla'">
          <a-tag v-if="record.slaResult==='achieved'" color="green">达成</a-tag>
          <a-tag v-else-if="record.slaResult==='breached'" color="red">违约</a-tag>
          <a-tag v-else-if="record.slaResult==='predicted_breach'" color="orange">预测违约</a-tag>
          <a-tag v-else-if="record.slaResult==='running'" color="blue">评估中</a-tag>
          <a-tag v-else>未知</a-tag>
        </template>
        <template v-else-if="column.key==='dur'">
          <template v-if="record.durationSec">{{ record.durationSec }} 秒</template>
          <template v-else>-</template>
        </template>
        <template v-else-if="column.key==='time'">
          <div>开始: {{ format(record.actualStart) }}</div>
          <div>结束: {{ format(record.actualEnd) }}</div>
        </template>
        <template v-else-if="column.key==='err'">
          <a-tooltip v-if="record.errorMessage" :title="record.errorMessage">
            <span style="color:#ff4d4f;">{{ record.errorMessage.slice(0,40) }}{{ record.errorMessage.length>40?'...':'' }}</span>
          </a-tooltip>
          <span v-else>-</span>
        </template>
      </template>
    </a-table>
  </a-card>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import dayjs from 'dayjs'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { pipelineApi, runApi } from '@/api'

const loading = ref(false)
const pipes = ref([])
const list = ref([])
const filter = reactive({ pipelineId: undefined, status: '', days: 7 })
const pagination = reactive({ current: 1, pageSize: 20, total: 0 })
const statusMap = {
  success: { color: 'success', text: '成功' },
  running: { color: 'processing', text: '运行中' },
  failed: { color: 'error', text: '失败' },
  timeout: { color: 'error', text: '超时' },
  pending: { color: 'default', text: '等待中' },
  cancelled: { color: 'warning', text: '取消' },
  skipped: { color: 'warning', text: '跳过' }
}
const cols = [
  { title:'管道', key:'pipe', ellipsis:true },
  { title:'运行ID', dataIndex:'runId', width:200 },
  { title:'状态', key:'status', width:100 },
  { title:'SLA', key:'sla', width:100 },
  { title:'耗时', key:'dur', width:100 },
  { title:'数据量', dataIndex:'dataVolume', width:110, customRender:({text})=> text ? text.toLocaleString() : '-' },
  { title:'时间', key:'time', width:260 },
  { title:'错误', key:'err', width:220, ellipsis:true }
]

function format(t) { return t ? dayjs(t).format('MM-DD HH:mm:ss') : '-' }

async function reload() {
  loading.value = true
  try {
    const r = await runApi.list({ ...filter, page: pagination.current, pageSize: pagination.pageSize })
    list.value = r.data || []
    pagination.total = r.total || 0
  } finally { loading.value = false }
}
function onPage(p) { pagination.current = p.current; pagination.pageSize = p.pageSize; reload() }

onMounted(async () => {
  const pr = await pipelineApi.list({ pageSize: 500 })
  pipes.value = pr.data || []
  reload()
})
</script>
