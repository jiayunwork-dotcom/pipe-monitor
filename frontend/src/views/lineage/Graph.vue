<template>
  <a-card>
    <template #title>
      <div style="display:flex;justify-content:space-between;align-items:center;">
        <span>数据血缘追踪</span>
        <a-space>
          <a-select v-model:value="selectedPipelineId" placeholder="选择中心管道" style="width:280px;" @change="reloadGraph">
            <a-select-option v-for="p in allPipes" :key="p.id" :value="p.id">{{ p.name }} ({{ p.code }})</a-select-option>
          </a-select>
          <a-select v-model:value="maxDepth" style="width:120px;" @change="reloadGraph">
            <a-select-option :value="1">1层</a-select-option>
            <a-select-option :value="2">2层</a-select-option>
            <a-select-option :value="3">3层</a-select-option>
            <a-select-option :value="4">4层</a-select-option>
            <a-select-option :value="5">5层</a-select-option>
          </a-select>
          <a-button type="primary" @click="reloadGraph" :loading="loading">
            <reload-outlined />刷新
          </a-button>
        </a-space>
      </div>
    </template>

    <a-row :gutter="16" style="margin-bottom:16px;">
      <a-col :span="18">
        <div ref="graphContainer" class="lineage-graph-container"></div>
      </a-col>
      <a-col :span="6">
        <a-card size="small" title="图例" style="margin-bottom:12px;">
          <div style="display:flex;flex-direction:column;gap:8px;">
            <div style="display:flex;align-items:center;gap:8px;">
              <span class="legend-dot" style="background:#52c41a;"></span>
              <span>运行成功</span>
            </div>
            <div style="display:flex;align-items:center;gap:8px;">
              <span class="legend-dot" style="background:#ff4d4f;"></span>
              <span>运行失败</span>
            </div>
            <div style="display:flex;align-items:center;gap:8px;">
              <span class="legend-dot" style="background:#1890ff;"></span>
              <span>运行中</span>
            </div>
            <div style="display:flex;align-items:center;gap:8px;">
              <span class="legend-dot" style="background:#bfbfbf;"></span>
              <span>未调度</span>
            </div>
            <div style="display:flex;align-items:center;gap:8px;">
              <span class="legend-dot" style="background:#722ed1;"></span>
              <span>外部数据源</span>
            </div>
            <div style="display:flex;align-items:center;gap:8px;">
              <span class="legend-dot" style="border:3px solid #fa8c16;"></span>
              <span>受影响节点</span>
            </div>
          </div>
        </a-card>

        <a-card size="small" title="操作" v-if="selectedNode">
          <a-space direction="vertical" style="width:100%;">
            <p style="margin:0;color:#666;">已选中: <strong>{{ selectedNode.name }}</strong></p>
            <a-button type="primary" block @click="doImpactAnalysis" :disabled="selectedNode.type !== 'pipeline'">
              <bulb-outlined />影响分析
            </a-button>
            <a-button block @click="clearSelection">
              清除选择
            </a-button>
          </a-space>
        </a-card>

        <a-card size="small" title="统计信息" style="margin-top:12px;">
          <a-descriptions size="small" :column="1" bordered>
            <a-descriptions-item label="节点总数">{{ graphData.nodes?.length || 0 }}</a-descriptions-item>
            <a-descriptions-item label="边总数">{{ graphData.edges?.length || 0 }}</a-descriptions-item>
            <a-descriptions-item label="上游节点">{{ upstreamCount }}</a-descriptions-item>
            <a-descriptions-item label="下游节点">{{ downstreamCount }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
    </a-row>

    <a-drawer
      v-model:open="impactDrawerVisible"
      title="影响分析结果"
      width="480"
      :mask-closable="false"
    >
      <div v-if="impactResult">
        <a-alert type="info" show-icon style="margin-bottom:16px;">
          <template #message>
            共影响 <strong>{{ impactResult.totalAffected }}</strong> 个下游管道，预计累计影响时间 <strong>{{ formatDuration(impactResult.totalImpactSec) }}</strong>
          </template>
        </a-alert>

        <a-table
          :data-source="impactResult.affectedPipelines"
          :pagination="{ pageSize: 10 }"
          row-key="pipelineId"
          size="small"
        >
          <template #columns>
            <a-table-column title="管道名称" data-index="name">
              <template #default="{ record }">
                <span>
                  <warning-outlined v-if="record.hasAlert" style="color:#faad14;margin-right:4px;" />
                  {{ record.name }}
                </span>
              </template>
            </a-table-column>
            <a-table-column title="编码" data-index="code" width="120" />
            <a-table-column title="负责团队" data-index="team" width="100" />
            <a-table-column title="影响深度" data-index="depth" width="80" />
            <a-table-column title="SLA状态" data-index="slaStatus" width="100">
              <template #default="{ record }">
                <a-tag :color="getSLAColor(record.slaStatus)">
                  {{ getSLALabel(record.slaStatus) }}
                </a-tag>
              </template>
            </a-table-column>
            <a-table-column title="预计影响" data-index="expectedImpactSec" width="100">
              <template #default="{ record }">
                {{ formatDuration(record.expectedImpactSec) }}
              </template>
            </a-table-column>
          </template>
        </a-table>
      </div>
    </a-drawer>
  </a-card>
</template>

<script setup>
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ReloadOutlined, BulbOutlined, WarningOutlined } from '@ant-design/icons-vue'
import { Network } from 'vis-network/standalone/esm/vis-network'
import { message } from 'ant-design-vue'
import { pipelineApi, lineageApi } from '@/api'
import dayjs from 'dayjs'

const route = useRoute()
const loading = ref(false)
const graphContainer = ref(null)
const allPipes = ref([])
const selectedPipelineId = ref(+route.query.pipelineId || 0)
const maxDepth = ref(5)
const graphData = ref({ nodes: [], edges: [] })
const selectedNode = ref(null)
const impactDrawerVisible = ref(false)
const impactResult = ref(null)
const highlightedNodes = ref(new Set())
let network = null

const upstreamCount = computed(() => {
  return graphData.value.nodes?.filter(n => n.direction === 'upstream').length || 0
})

const downstreamCount = computed(() => {
  return graphData.value.nodes?.filter(n => n.direction === 'downstream').length || 0
})

async function loadAllPipes() {
  const r = await pipelineApi.list({ pageSize: 500 })
  allPipes.value = r.data?.data || []
}

async function reloadGraph() {
  if (!selectedPipelineId.value) {
    message.info('请先选择中心管道')
    return
  }
  loading.value = true
  try {
    const r = await lineageApi.getGraph({ pipelineId: selectedPipelineId.value, maxDepth: maxDepth.value })
    graphData.value = r.data || { nodes: [], edges: [] }
    selectedNode.value = null
    highlightedNodes.value = new Set()
    await nextTick()
    renderGraph()
  } finally {
    loading.value = false
  }
}

function getNodeColor(status) {
  const colors = {
    success: '#52c41a',
    failed: '#ff4d4f',
    running: '#1890ff',
    gray: '#bfbfbf'
  }
  return colors[status] || colors.gray
}

function renderGraph() {
  if (!graphContainer.value) return

  const nodesArr = []
  const edgesArr = []
  const nodesMap = new Map()

  graphData.value.nodes.forEach(n => {
    nodesMap.set(n.id, n)
    const isHighlighted = highlightedNodes.value.has(n.id)
    const isCenter = n.direction === 'center'

    let bgColor = '#fff'
    let borderColor = '#bfbfbf'
    let shape = 'box'

    if (n.type === 'external') {
      bgColor = '#f9f0ff'
      borderColor = '#722ed1'
      shape = 'ellipse'
    } else {
      borderColor = getNodeColor(n.lastRunStatus)
      if (isCenter) {
        bgColor = '#e6f7ff'
      }
    }

    const node = {
      id: n.id,
      label: n.type === 'pipeline' ? `${n.name}\n${n.code || ''}` : n.name,
      shape: shape,
      color: {
        background: bgColor,
        border: isHighlighted ? '#fa8c16' : borderColor
      },
      borderWidth: isHighlighted ? 4 : 2,
      font: { size: 13, align: 'center' },
      size: n.type === 'external' ? 18 : 22,
      level: n.level,
      title: getNodeTooltip(n),
      shadow: isHighlighted
    }

    if (n.hasAlert && n.type === 'pipeline') {
      node.label = `⚠ ${n.name}\n${n.code || ''}`
    }

    nodesArr.push(node)
  })

  graphData.value.edges.forEach(e => {
    const sourceNode = nodesMap.get(e.source)
    const targetNode = nodesMap.get(e.target)
    const isHighlighted = highlightedNodes.value.has(e.source) && highlightedNodes.value.has(e.target)

    edgesArr.push({
      id: e.id,
      from: e.source,
      to: e.target,
      arrows: 'to',
      color: {
        color: isHighlighted ? '#fa8c16' : '#aaa',
        highlight: '#fa8c16'
      },
      width: isHighlighted ? 3 : 1,
      smooth: {
        type: 'cubicBezier',
        forceDirection: 'horizontal',
        roundness: 0.4
      },
      dashes: e.dependencyType === 'soft' ? [5, 5] : false
    })
  })

  const data = { nodes: nodesArr, edges: edgesArr }
  const options = {
    layout: {
      hierarchical: {
        enabled: true,
        direction: 'LR',
        sortMethod: 'directed',
        levelSeparation: 200,
        nodeSpacing: 150,
        treeSpacing: 200,
        shakeTowards: 'leaves'
      }
    },
    interaction: {
      hover: true,
      zoomView: true,
      dragView: true,
      dragNodes: true,
      tooltipDelay: 100,
      multiselect: false,
      selectConnectedEdges: false
    },
    physics: false,
    edges: {
      smooth: {
        type: 'cubicBezier',
        forceDirection: 'horizontal',
        roundness: 0.4
      },
      arrows: {
        to: { enabled: true, scaleFactor: 0.8 }
      }
    },
    nodes: {
      shadow: {
        enabled: false,
        color: 'rgba(0,0,0,0.2)',
        size: 10,
        x: 3,
        y: 3
      }
    }
  }

  if (network) {
    network.destroy()
  }

  network = new Network(graphContainer.value, data, options)

  network.on('click', (params) => {
    if (params.nodes.length > 0) {
      const nodeId = params.nodes[0]
      selectedNode.value = nodesMap.get(nodeId) || null
    } else {
      selectedNode.value = null
    }
  })

  network.on('doubleClick', (params) => {
    if (params.nodes.length > 0) {
      const nodeId = params.nodes[0]
      const node = nodesMap.get(nodeId)
      if (node && node.type === 'pipeline' && node.pipelineId) {
        window.location = `#/pipelines/${node.pipelineId}`
      }
    }
  })
}

function getNodeTooltip(node) {
  if (node.type === 'external') {
    return `<div style="padding:8px;">
      <strong>${node.name}</strong><br/>
      <span style="color:#888;">外部数据源</span>
    </div>`
  }

  const slaLabel = getSLALabel(node.slaStatus)
  const runTime = node.lastRunTime ? dayjs(node.lastRunTime).format('YYYY-MM-DD HH:mm:ss') : '从未运行'

  return `<div style="padding:8px;min-width:200px;">
    <strong>${node.name}</strong><br/>
    <span style="color:#888;">编码: ${node.code || '-'}</span><br/>
    <span style="color:#888;">团队: ${node.team || '-'}</span><br/>
    <span style="color:#888;">最近运行: ${runTime}</span><br/>
    <span style="color:#888;">SLA状态: ${slaLabel}</span>
    ${node.hasAlert ? '<br/><span style="color:#faad14;">⚠ 存在未处理告警</span>' : ''}
  </div>`
}

function getSLAColor(status) {
  const colors = {
    achieved: 'green',
    predicted_breach: 'orange',
    breached: 'red',
    running: 'blue',
    unknown: 'default'
  }
  return colors[status] || 'default'
}

function getSLALabel(status) {
  const labels = {
    achieved: '已达成',
    predicted_breach: '预计违约',
    breached: '已违约',
    running: '运行中',
    unknown: '未知'
  }
  return labels[status] || '未知'
}

function formatDuration(seconds) {
  if (!seconds || seconds <= 0) return '-'
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = seconds % 60
  if (hours > 0) {
    return `${hours}小时${minutes}分`
  } else if (minutes > 0) {
    return `${minutes}分${secs}秒`
  }
  return `${secs}秒`
}

async function doImpactAnalysis() {
  if (!selectedNode.value || selectedNode.value.type !== 'pipeline') {
    message.warning('请先选择一个管道节点')
    return
  }

  try {
    const r = await lineageApi.impactAnalysis({ nodeId: selectedNode.value.id })
    impactResult.value = r.data
    impactDrawerVisible.value = true

    const affected = new Set()
    affected.add(selectedNode.value.id)
    impactResult.value.affectedPipelines?.forEach(p => {
      affected.add(`pipe_${p.pipelineId}`)
    })
    highlightedNodes.value = affected

    renderGraph()
  } catch (e) {
    message.error(e.message || '影响分析失败')
  }
}

function clearSelection() {
  selectedNode.value = null
  highlightedNodes.value = new Set()
  impactDrawerVisible.value = false
  impactResult.value = null
  renderGraph()
}

watch(selectedPipelineId, () => {
  if (selectedPipelineId.value) {
    reloadGraph()
  }
})

onMounted(async () => {
  await loadAllPipes()
  if (selectedPipelineId.value) {
    await reloadGraph()
  }
})
</script>

<style scoped>
.lineage-graph-container {
  width: 100%;
  height: 600px;
  border: 1px solid #e8e8e8;
  border-radius: 4px;
  background: #fafafa;
}

.legend-dot {
  display: inline-block;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #ccc;
}
</style>
