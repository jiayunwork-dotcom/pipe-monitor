import { defineStore } from 'pinia'
import { authApi, userApi } from '@/api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: '',
    user: null
  }),
  getters: {
    isAdmin: state => state.user?.role === 'admin' || state.user?.role === 'super_admin',
    isSuper: state => state.user?.role === 'super_admin',
    tenantId: state => state.user?.tenantId
  },
  actions: {
    async login(username, password) {
      const res = await authApi.login({ username, password })
      this.token = res.token
      this.user = res.user
      localStorage.setItem('token', res.token)
      return res
    },
    async loadUser() {
      try {
        const res = await userApi.me()
        this.user = res.data
      } catch (e) {
        this.logout()
      }
    },
    logout() {
      this.token = ''
      this.user = null
      localStorage.removeItem('token')
      sessionStorage.removeItem('token')
    }
  },
  persist: {
    key: 'pipe-monitor-auth',
    paths: ['token', 'user']
  }
})
