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
            <div style="display:flex;align-items:center;gap:8px;">
              <span class="legend-dot" style="background:#e6f7ff;border:2px dashed #1890ff;"></span>
              <span>团队分组(聚合)</span>
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

        <a-card size="small" title="快照管理" style="margin-top:12px;">
          <template #extra v-if="auth.isAdmin">
            <a-button type="primary" size="small" @click="openCreateSnapshot">
              <camera-outlined />打快照
            </a-button>
          </template>
          <div v-if="snapshots.length === 0" style="color:#999;text-align:center;padding:20px 0;">
            暂无快照
          </div>
          <a-checkbox-group v-model:value="selectedSnapshotIds" v-else>
            <div v-for="s in snapshots" :key="s.id" style="padding:6px 0;border-bottom:1px solid #f0f0f0;">
              <a-checkbox :value="s.id">
                <span>{{ s.name }}</span>
              </a-checkbox>
              <div style="font-size:12px;color:#999;margin-left:24px;">
                {{ formatDateTime(s.createdAt) }} | {{ s.user?.fullName || s.user?.username || '-' }}
                <a-popconfirm v-if="auth.isAdmin" title="确认删除?" @confirm="()=>deleteSnapshot(s.id)" style="margin-left:8px;">
                  <a-button type="link" size="small" danger>删除</a-button>
                </a-popconfirm>
              </div>
            </div>
          </a-checkbox-group>
          <a-button
            v-if="snapshots.length > 0"
            type="primary"
            size="small"
            block
            style="margin-top:12px;"
            :disabled="!selectedSnapshotIds || selectedSnapshotIds.length !== 2"
            @click="compareSnapshots"
          >
            对比选中的2个快照
          </a-button>
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

    <a-modal v-model:open="createSnapshotVisible" title="创建血缘快照" @ok="doCreateSnapshot" :confirm-loading="creatingSnapshot" ok-text="创建">
      <a-form layout="vertical">
        <a-form-item label="快照名称" required>
          <a-input v-model:value="snapshotForm.name" placeholder="例如：2024Q1版本血缘快照" />
        </a-form-item>
        <a-form-item label="描述">
          <a-textarea v-model:value="snapshotForm.description" :rows="3" placeholder="描述快照的用途或版本信息" />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="snapshotDiffVisible" title="快照差异对比" width="800px" :footer="null">
      <a-alert type="info" show-icon style="margin-bottom:16px;">
        <template #message>
          新增 <strong>{{ snapshotDiff.added?.length || 0 }}</strong> 条边，删除 <strong>{{ snapshotDiff.removed?.length || 0 }}</strong> 条边
        </template>
      </a-alert>
      <a-table
        :data-source="allDiffItems"
        :pagination="{ pageSize: 10 }"
        row-key="diffKey"
        size="small"
      >
        <template #columns>
          <a-table-column title="变更类型" data-index="changeType" width="100">
            <template #default="{ record }">
              <a-tag :color="record.changeType === 'added' ? 'green' : 'red'">
                {{ record.changeType === 'added' ? '新增' : '删除' }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column title="上游" width="200">
            <template #default="{ record }">
              {{ record.upstreamName || '-' }}
              <a-tag v-if="record.upstreamCode" color="default" style="margin-left:4px;">{{ record.upstreamCode }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="下游" width="200">
            <template #default="{ record }">
              {{ record.downstreamName || '-' }}
              <a-tag v-if="record.downstreamCode" color="default" style="margin-left:4px;">{{ record.downstreamCode }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="依赖类型" data-index="dependencyType" width="100">
            <template #default="{ record }">
              <a-tag :color="record.dependencyType === 'hard' ? 'red' : 'orange'">
                {{ record.dependencyType === 'hard' ? '强依赖' : '弱依赖' }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column title="描述" data-index="description">
            <template #default="{ record }">
              {{ record.description || '-' }}
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-modal>
  </a-card>
</template>

<script setup>
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ReloadOutlined, BulbOutlined, WarningOutlined, CameraOutlined } from '@ant-design/icons-vue'
import { Network } from 'vis-network/standalone/esm/vis-network'
import { message } from 'ant-design-vue'
import { pipelineApi, lineageApi } from '@/api'
import dayjs from 'dayjs'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
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
const collapsedTeams = ref(new Set())
const healthScores = ref({})
let network = null

const snapshots = ref([])
const selectedSnapshotIds = ref([])
const createSnapshotVisible = ref(false)
const creatingSnapshot = ref(false)
const snapshotForm = ref({ name: '', description: '' })
const snapshotDiffVisible = ref(false)
const snapshotDiff = ref({ added: [], removed: [] })

const upstreamCount = computed(() => {
  return graphData.value.nodes?.filter(n => n.direction === 'upstream').length || 0
})

const downstreamCount = computed(() => {
  return graphData.value.nodes?.filter(n => n.direction === 'downstream').length || 0
})

const allDiffItems = computed(() => {
  const added = (snapshotDiff.value.added || []).map((item, idx) => ({ ...item, diffKey: `a_${idx}` }))
  const removed = (snapshotDiff.value.removed || []).map((item, idx) => ({ ...item, diffKey: `r_${idx}` }))
  return [...added, ...removed]
})

async function loadAllPipes() {
  const r = await pipelineApi.list({ pageSize: 500 })
  allPipes.value = r.data || []
}

async function loadSnapshots() {
  try {
    const r = await lineageApi.listSnapshots()
    snapshots.value = r.data || []
  } catch (e) {
    // ignore
  }
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
    healthScores.value = {}

    collapsedTeams.value = new Set()
    const teamCounts = {}
    graphData.value.nodes.forEach(n => {
      if (n.team && n.type === 'pipeline') {
        teamCounts[n.team] = (teamCounts[n.team] || 0) + 1
      }
    })
    Object.entries(teamCounts).forEach(([team, count]) => {
      if (count > 3) {
        collapsedTeams.value.add(team)
      }
    })

    const pipelineIds = graphData.value.nodes.filter(n => n.type === 'pipeline' && n.pipelineId).map(n => n.pipelineId)
    const healthPromises = pipelineIds.map(async (pid) => {
      try {
        const hs = await lineageApi.getHealthScore(pid)
        healthScores.value[String(pid)] = hs.data
      } catch (e) {
        // ignore
      }
    })
    await Promise.all(healthPromises)

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

  const nodesMap = new Map()
  graphData.value.nodes.forEach(n => nodesMap.set(n.id, n))

  const teamGroups = {}
  graphData.value.nodes.forEach(n => {
    if (n.team && n.type === 'pipeline') {
      if (!teamGroups[n.team]) teamGroups[n.team] = []
      teamGroups[n.team].push(n)
    }
  })

  const nodesArr = []
  const edgesArr = []
  const collapsedNodeMap = new Map()

  graphData.value.nodes.forEach(n => {
    const isCollapsed = n.team && collapsedTeams.value.has(n.team) && teamGroups[n.team]?.length > 3
    if (isCollapsed && n.team) {
      if (!collapsedNodeMap.has(n.team)) {
        const teamNodes = teamGroups[n.team]
        collapsedNodeMap.set(n.team, true)
        nodesArr.push({
          id: `team_${n.team}`,
          label: `${n.team}\n(${teamNodes.length}个节点)`,
          shape: 'box',
          color: {
            background: '#e6f7ff',
            border: '#1890ff'
          },
          borderWidth: 2,
          borderWidthSelected: 4,
          font: { size: 14, align: 'center', color: '#0050b3' },
          size: 30,
          title: getTeamTooltip(n.team, teamNodes),
          team: n.team,
          isTeamGroup: true,
          direction: teamNodes[0]?.direction || 'center'
        })
      }
      return
    }

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
    if (!sourceNode || !targetNode) return

    let sourceId = e.source
    let targetId = e.target

    if (sourceNode.team && collapsedTeams.value.has(sourceNode.team) && teamGroups[sourceNode.team]?.length > 3) {
      sourceId = `team_${sourceNode.team}`
    }
    if (targetNode.team && collapsedTeams.value.has(targetNode.team) && teamGroups[targetNode.team]?.length > 3) {
      targetId = `team_${targetNode.team}`
    }

    if (sourceId === targetId) return

    const isHighlighted = highlightedNodes.value.has(e.source) && highlightedNodes.value.has(e.target)

    const existingEdge = edgesArr.find(ed => ed.from === sourceId && ed.to === targetId)
    if (existingEdge) return

    edgesArr.push({
      id: e.id,
      from: sourceId,
      to: targetId,
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
      if (nodeId.startsWith('team_')) {
        selectedNode.value = { id: nodeId, name: nodeId.replace('team_', ''), type: 'team' }
      } else {
        selectedNode.value = nodesMap.get(nodeId) || null
      }
    } else {
      selectedNode.value = null
    }
  })

  network.on('doubleClick', (params) => {
    if (params.nodes.length > 0) {
      const nodeId = params.nodes[0]
      if (nodeId.startsWith('team_')) {
        const team = nodeId.replace('team_', '')
        if (collapsedTeams.value.has(team)) {
          collapsedTeams.value.delete(team)
        } else {
          collapsedTeams.value.add(team)
        }
        renderGraph()
        return
      }
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
  const health = node.pipelineId ? healthScores.value[String(node.pipelineId)] : null

  let healthHtml = ''
  if (health) {
    const healthColors = { excellent: '#52c41a', good: '#1890ff', fair: '#faad14', poor: '#ff4d4f' }
    const healthLabels = { excellent: '优秀', good: '良好', fair: '一般', poor: '较差' }
    healthHtml = `<div style="border-top:1px solid #eee;margin-top:8px;padding-top:8px;">
      <strong style="color:${healthColors[health.level] || '#333'};">健康度: ${health.score}/100 (${healthLabels[health.level] || health.level})</strong>
      ${health.details?.length ? '<br/><span style="color:#888;font-size:12px;">' + health.details.join('<br/>') + '</span>' : ''}
    </div>`
  }

  return `<div style="padding:8px;min-width:220px;max-width:320px;">
    <strong>${node.name}</strong><br/>
    <span style="color:#888;">编码: ${node.code || '-'}</span><br/>
    <span style="color:#888;">团队: ${node.team || '-'}</span><br/>
    <span style="color:#888;">最近运行: ${runTime}</span><br/>
    <span style="color:#888;">SLA状态: ${slaLabel}</span>
    ${node.hasAlert ? '<br/><span style="color:#faad14;">⚠ 存在未处理告警</span>' : ''}
    ${healthHtml}
  </div>`
}

function getTeamTooltip(team, nodes) {
  return `<div style="padding:8px;min-width:200px;">
    <strong style="font-size:14px;">${team}</strong><br/>
    <span style="color:#888;">共 ${nodes.length} 个管道节点</span><br/>
    <div style="margin-top:8px;font-size:12px;color:#666;">
      ${nodes.map(n => n.name).join('<br/>')}
    </div>
    <div style="margin-top:8px;color:#1890ff;font-size:12px;">双击可${collapsedTeams.value.has(team) ? '展开' : '折叠'}</div>
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

function formatDateTime(t) {
  return t ? dayjs(t).format('YYYY-MM-DD HH:mm') : '-'
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

function openCreateSnapshot() {
  snapshotForm.value = {
    name: dayjs().format('YYYY-MM-DD HH:mm') + ' 血缘快照',
    description: ''
  }
  createSnapshotVisible.value = true
}

async function doCreateSnapshot() {
  if (!snapshotForm.value.name.trim()) {
    message.warning('请填写快照名称')
    return
  }
  creatingSnapshot.value = true
  try {
    await lineageApi.createSnapshot(snapshotForm.value)
    message.success('快照创建成功')
    createSnapshotVisible.value = false
    loadSnapshots()
  } catch (e) {
    message.error(e.message || '创建失败')
  } finally {
    creatingSnapshot.value = false
  }
}

async function deleteSnapshot(id) {
  try {
    await lineageApi.deleteSnapshot(id)
    message.success('已删除')
    selectedSnapshotIds.value = selectedSnapshotIds.value.filter(sid => sid !== id)
    loadSnapshots()
  } catch (e) {
    message.error(e.message || '删除失败')
  }
}

async function compareSnapshots() {
  const ids = selectedSnapshotIds.value
  if (!ids || ids.length !== 2) {
    message.warning('请选择2个快照进行对比')
    return
  }
  const id1 = Number(ids[0])
  const id2 = Number(ids[1])
  if (!id1 || !id2) {
    message.warning('快照ID无效，请重新选择')
    return
  }
  try {
    const r = await lineageApi.compareSnapshots({
      snapshotId1: id1,
      snapshotId2: id2
    })
    snapshotDiff.value = r.data || { added: [], removed: [] }
    snapshotDiffVisible.value = true
  } catch (e) {
    console.error('快照对比失败:', e)
    message.error(e?.response?.data?.error || e.message || '对比失败')
  }
}

watch(selectedPipelineId, () => {
  if (selectedPipelineId.value) {
    reloadGraph()
  }
})

onMounted(async () => {
  await loadAllPipes()
  await loadSnapshots()
  if (!selectedPipelineId.value && allPipes.value.length > 0) {
    selectedPipelineId.value = allPipes.value[0].id
  }
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
