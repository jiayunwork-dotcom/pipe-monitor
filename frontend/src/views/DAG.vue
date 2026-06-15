<template>
  <a-card>
    <template #title>
      <div style="display:flex;justify-content:space-between;align-items:center;">
        <span>管道依赖图</span>
        <a-space>
          <a-button-group>
            <a-button :type="viewMode==='graph'?'primary':'default'" @click="viewMode='graph'">拓扑图</a-button>
            <a-button :type="viewMode==='table'?'primary':'default'" @click="viewMode='table'">层级表</a-button>
          </a-button-group>
          <a-select v-model:value="startPipelineId" allow-clear placeholder="从某个管道开始" style="width:280px;" @change="reload">
            <a-select-option v-for="p in allPipes" :key="p.id" :value="p.id">{{ p.name }} ({{ p.code }})</a-select-option>
          </a-select>
          <a-button type="primary" @click="reload"><reload-outlined :spin="loading" />刷新</a-button>
        </a-space>
      </div>
    </template>

    <a-row :gutter="16" style="margin-bottom:16px;" v-if="criticalPath">
      <a-col :span="12">
        <a-alert type="info" show-icon>
          <template #message>关键路径 ({{ criticalPath.totalDuration }}秒)</template>
          <template #description>
            <a-tag v-for="(p,i) in criticalPath.pathDetails" :key="i" color="blue" style="margin:2px;">
              {{ p }} <span v-if="i<criticalPath.pathDetails.length-1">→</span>
            </a-tag>
          </template>
        </a-alert>
      </a-col>
      <a-col :span="12">
        <a-alert v-if="affected.length" type="warning" show-icon>
          <template #message>受当前管道影响的下游 ({{ affected.length }}条)</template>
          <template #description>
            <a-tag v-for="a in affected" :key="a.pipelineId" color="orange" style="margin:2px;">
              {{ a.code }} (深度{{ a.depth }})
            </a-tag>
          </template>
        </a-alert>
      </a-col>
    </a-row>

    <div v-show="viewMode==='graph'" ref="graphEl" class="dag-container"></div>

    <a-table v-show="viewMode==='table'" :data-source="levelTable" row-key="id" size="small" :pagination="false">
      <template #columns>
        <a-table-column title="层级" data-index="level" width="100" />
        <a-table-column title="管道" data-index="name" />
        <a-table-column title="编码" data-index="code" width="180" />
        <a-table-column title="状态" data-index="status" width="100" />
      </template>
    </a-table>
  </a-card>
</template>

<script setup>
import { ref, reactive, onMounted, nextTick, watch, computed, h } from 'vue'
import { useRoute } from 'vue-router'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { Network } from 'vis-network/standalone/esm/vis-network'
import { pipelineApi } from '@/api'

const route = useRoute()
const loading = ref(false)
const viewMode = ref('graph')
const graphEl = ref(null)
const allPipes = ref([])
const startPipelineId = ref(+route.query.startPipelineId || 0)
const dag = ref({ nodes: {}, topoSort: [], levels: {} })
const criticalPath = ref(null)
const affected = ref([])
let network = null

const levelTable = computed(() => {
  const rows = []
  Object.entries(dag.value.levels || {}).forEach(([lvl, ids]) => {
    ids.forEach(id => {
      const n = dag.value.nodes[id]
      if (n) rows.push({ id, level: `第${+lvl+1}层`, name: n.name, code: n.code, status: n.status })
    })
  })
  return rows
})

async function reload() {
  loading.value = true
  try {
    const [r, cp] = await Promise.all([
      pipelineApi.dagGraph({ startPipelineId: startPipelineId.value, includeUpstream: true, includeDownstream: true, maxDepth: 0 }),
      pipelineApi.criticalPath().catch(() => ({ data: null }))
    ])
    dag.value = r.data || { nodes: {}, topoSort: [], levels: {} }
    criticalPath.value = cp.data
    if (startPipelineId.value) {
      const af = await pipelineApi.affected(startPipelineId.value)
      affected.value = af.data || []
    } else {
      affected.value = []
    }
    nextTick(() => renderGraph())
  } finally { loading.value = false }
}

async function loadAllPipes() {
  const r = await pipelineApi.list({ pageSize: 500 })
  allPipes.value = r.data || []
}

function renderGraph() {
  if (!graphEl.value) return
  const nodesArr = []
  const edgesArr = []
  const healthColor = { green:'#52c41a', yellow:'#faad14', red:'#ff4d4f', blue:'#1890ff', gray:'#bfbfbf' }
  Object.values(dag.value.nodes || {}).forEach(n => {
    nodesArr.push({
      id: n.id, label: `${n.name}\n${n.code}`, level: n.level || 0,
      color: { background: '#fff', border: healthColor[n.health] || healthColor.gray },
      shape: 'box', font: { size: 13, align: 'center' }, borderWidth: 2,
      size: 20
    })
    n.children.forEach(cid => {
      if (dag.value.nodes[cid]) edgesArr.push({ from: n.id, to: cid, arrows: 'to', color: { color: '#aaa' } })
    })
  })
  const data = { nodes: nodesArr, edges: edgesArr }
  const options = {
    layout: { hierarchical: { enabled: true, direction: 'UD', sortMethod: 'directed', levelSeparation: 150, nodeSpacing: 180 } },
    interaction: { hover: true, zoomView: true, dragView: true, dragNodes: true, tooltipDelay: 100 },
    physics: false,
    edges: { smooth: { type: 'cubicBezier', forceDirection: 'vertical', roundness: 0.4 } }
  }
  if (network) network.destroy()
  network = new Network(graphEl.value, data, options)
  network.on('doubleClick', p => {
    if (p.nodes?.length) window.location = `#/pipelines/${p.nodes[0]}`
  })
}

watch(startPipelineId, () => reload())
onMounted(async () => { await loadAllPipes(); reload() })
</script>
