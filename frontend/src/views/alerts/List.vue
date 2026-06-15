<template>
  <a-card>
    <template #title>
      <div style="display:flex;justify-content:space-between;">
        <span>告警事件</span>
        <a-space>
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
    <a-table :columns="cols" :data-source="list" :loading="loading" :pagination="pagination" @change="onPage" row-key="id" :expanded-row-keys="expandedKeys" @expand="onExpand">
      <template #expandedRowRender="{ record }">
        <a-descriptions :column="2" bordered size="small">
          <a-descriptions-item label="详细信息">{{ record.detail || '-' }}</a-descriptions-item>
          <a-descriptions-item label="SLA违约原因">{{ record.slaBreachReason || '-' }}</a-descriptions-item>
          <a-descriptions-item label="认领备注">{{ record.ackNote || '-' }}</a-descriptions-item>
          <a-descriptions-item label="解决备注">{{ record.resolveNote || '-' }}</a-descriptions-item>
        </a-descriptions>
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

    <a-modal v-model:open="ackVisible" title="认领告警" @ok="doAck">
      <a-textarea v-model:value="ackNote" :rows="4" placeholder="处理备注" />
    </a-modal>
    <a-modal v-model:open="resolveVisible" title="关闭告警" @ok="doResolve">
      <a-textarea v-model:value="resolveNote" :rows="4" placeholder="解决备注" />
    </a-modal>
  </a-card>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { message } from 'ant-design-vue'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { alertApi } from '@/api'
import { useWebSocketStore } from '@/stores/ws'

const loading = ref(false)
const list = ref([])
const filter = reactive({ status: '', severity: '', days: 30 })
const pagination = reactive({ current:1, pageSize:20, total:0 })
const expandedKeys = ref([])
const ackVisible = ref(false)
const resolveVisible = ref(false)
const ackNote = ref('')
const resolveNote = ref('')
let curAlert = null

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
  { title:'触发时间', dataIndex:'triggeredAt', width:170, customRender:({text})=>text?.slice(0,16).replace('T',' ') },
  { title:'认领人', dataIndex:'acknowledgedBy', width:110, customRender:({text})=> text?.fullName || '-' },
  { title:'操作', key:'op', width:120, fixed:'right' }
]

async function reload() {
  loading.value = true
  try {
    const r = await alertApi.list({ ...filter, page: pagination.current, pageSize: pagination.pageSize })
    list.value = r.data || []
    pagination.total = r.total || 0
  } finally { loading.value = false }
}
function onPage(p) { pagination.current = p.current; pagination.pageSize = p.pageSize; reload() }
function onExpand(keys) { expandedKeys.value = keys }

function ack(a) { curAlert = a; ackNote.value = ''; ackVisible.value = true }
function resolve(a) { curAlert = a; resolveNote.value = ''; resolveVisible.value = true }
async function doAck() {
  await alertApi.acknowledge(curAlert.id, { note: ackNote.value })
  message.success('已认领')
  ackVisible.value = false
  reload()
}
async function doResolve() {
  await alertApi.resolve(curAlert.id, { note: resolveNote.value })
  message.success('已关闭')
  resolveVisible.value = false
  reload()
}

const onNewAlert = () => reload()

onMounted(() => {
  reload()
  ws.on('new_alert', onNewAlert)
})
onUnmounted(() => ws.off('new_alert', onNewAlert))
</script>
