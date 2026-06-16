import { defineStore } from 'pinia'

const RECONNECT_BASE_DELAY = 2000
const RECONNECT_MAX_DELAY = 30000

export const useWebSocketStore = defineStore('ws', {
  state: () => ({
    connected: false,
    ws: null,
    listeners: {},
    reconnectTimer: null,
    reconnectAttempts: 0,
    token: null,
    manualClose: false,
    visibilityHandler: null,
    onlineHandler: null
  }),
  actions: {
    connect(token) {
      if (token) {
        this.token = token
      }
      if (!this.token) {
        return
      }
      if (this.ws && this.ws.readyState === WebSocket.OPEN) return
      if (this.ws && this.ws.readyState === WebSocket.CONNECTING) return

      this.manualClose = false
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const url = `${protocol}//${window.location.host}/ws?token=${this.token}`
      this.ws = new WebSocket(url)

      this.ws.onopen = () => {
        this.connected = true
        this.reconnectAttempts = 0
        this.clearReconnectTimer()
        this.emit('connected', null)
      }

      this.ws.onmessage = e => {
        try {
          const msg = JSON.parse(e.data)
          const handlers = this.listeners[msg.type] || []
          handlers.forEach(fn => {
            try { fn(msg.payload) } catch (err) { console.error(err) }
          })
        } catch (err) {
          console.error('WS parse error:', err)
        }
      }

      this.ws.onerror = () => {
        this.connected = false
      }

      this.ws.onclose = () => {
        this.connected = false
        if (!this.manualClose) {
          this.scheduleReconnect()
        }
      }

      this.setupGlobalListeners()
    },
    setupGlobalListeners() {
      if (this.visibilityHandler) return

      this.visibilityHandler = () => {
        if (document.visibilityState === 'visible') {
          if (!this.connected || !this.ws || this.ws.readyState !== WebSocket.OPEN) {
            this.clearReconnectTimer()
            this.reconnectAttempts = 0
            this.connect()
          }
        }
      }
      document.addEventListener('visibilitychange', this.visibilityHandler)

      this.onlineHandler = () => {
        if (!this.connected || !this.ws || this.ws.readyState !== WebSocket.OPEN) {
          this.clearReconnectTimer()
          this.reconnectAttempts = 0
          this.connect()
        }
      }
      window.addEventListener('online', this.onlineHandler)
    },
    removeGlobalListeners() {
      if (this.visibilityHandler) {
        document.removeEventListener('visibilitychange', this.visibilityHandler)
        this.visibilityHandler = null
      }
      if (this.onlineHandler) {
        window.removeEventListener('online', this.onlineHandler)
        this.onlineHandler = null
      }
    },
    scheduleReconnect() {
      if (this.reconnectTimer) return
      this.reconnectAttempts++
      const delay = Math.min(
        RECONNECT_BASE_DELAY * Math.pow(2, this.reconnectAttempts - 1),
        RECONNECT_MAX_DELAY
      )
      this.reconnectTimer = setTimeout(() => {
        this.reconnectTimer = null
        this.connect()
      }, delay)
    },
    clearReconnectTimer() {
      if (this.reconnectTimer) {
        clearTimeout(this.reconnectTimer)
        this.reconnectTimer = null
      }
    },
    on(type, handler) {
      if (!this.listeners[type]) this.listeners[type] = []
      this.listeners[type].push(handler)
    },
    off(type, handler) {
      if (!this.listeners[type]) return
      const idx = this.listeners[type].indexOf(handler)
      if (idx >= 0) this.listeners[type].splice(idx, 1)
    },
    emit(type, payload) {
      const handlers = this.listeners[type] || []
      handlers.forEach(fn => {
        try { fn(payload) } catch (err) { console.error(err) }
      })
    },
    disconnect() {
      this.manualClose = true
      this.removeGlobalListeners()
      this.clearReconnectTimer()
      if (this.ws) this.ws.close()
      this.connected = false
    }
  }
})
