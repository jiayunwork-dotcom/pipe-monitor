import http from './http'

export const authApi = {
  login: data => http.post('/auth/login', data),
  logout: () => http.post('/auth/logout')
}

export const dashboardApi = {
  overview: () => http.get('/v1/dashboard/overview')
}

export const pipelineApi = {
  list: params => http.get('/v1/pipelines', { params }),
  get: id => http.get(`/v1/pipelines/${id}`),
  create: data => http.post('/v1/pipelines', data),
  update: (id, data) => http.put(`/v1/pipelines/${id}`, data),
  runHistory: (id, params) => http.get(`/v1/pipelines/${id}/runs/history`, { params }),
  dagGraph: params => http.get('/v1/pipelines/dag/graph', { params }),
  affected: id => http.get(`/v1/pipelines/${id}/affected`),
  criticalPath: () => http.get('/v1/pipelines/dag/critical-path'),
  checkCycle: id => http.post(`/v1/pipelines/${id}/check-cycle`)
}

export const runApi = {
  list: params => http.get('/v1/runs', { params }),
  report: data => http.post('/v1/runs/report', data)
}

export const slaApi = {
  rules: params => http.get('/v1/sla/rules', { params }),
  createRule: data => http.post('/v1/sla/rules', data),
  updateRule: (id, data) => http.put(`/v1/sla/rules/${id}`, data),
  deleteRule: id => http.delete(`/v1/sla/rules/${id}`),
  evaluations: params => http.get('/v1/sla/evaluations', { params }),
  stats: params => http.get('/v1/sla/stats', { params }),
  monthlyReports: params => http.get('/v1/sla/monthly-reports', { params })
}

export const alertApi = {
  list: params => http.get('/v1/alerts', { params }),
  acknowledge: (id, data) => http.post(`/v1/alerts/${id}/acknowledge`, data),
  resolve: (id, data) => http.post(`/v1/alerts/${id}/resolve`, data),
  rules: params => http.get('/v1/alerts/rules', { params }),
  createRule: data => http.post('/v1/alerts/rules', data),
  updateRule: (id, data) => http.put(`/v1/alerts/rules/${id}`, data),
  deleteRule: id => http.delete(`/v1/alerts/rules/${id}`),
  notifications: id => http.get(`/v1/alerts/${id}/notifications`)
}

export const oncallApi = {
  groups: () => http.get('/v1/oncall/groups'),
  createGroup: data => http.post('/v1/oncall/groups', data),
  assignments: (id, params) => http.get(`/v1/oncall/groups/${id}/assignments`, { params }),
  current: (id, params) => http.get(`/v1/oncall/groups/${id}/current`, { params }),
  handover: (id, data) => http.post(`/v1/oncall/groups/${id}/handover`, data),
  handovers: (id, params) => http.get(`/v1/oncall/groups/${id}/handovers`, { params }),
  me: () => http.get('/v1/oncall/me')
}

export const userApi = {
  me: () => http.get('/v1/users/me'),
  list: params => http.get('/v1/users', { params })
}
