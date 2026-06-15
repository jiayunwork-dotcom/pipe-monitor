import { defineStore } from 'pinia'

export const useWebSocketStore = defineStore('ws', {
  state: () => ({
    connected: false,
    ws: null,
    listeners: {},
    reconnectTimer: null
  }),
  actions: {
    connect(token) {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) return
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const url = `${protocol}//${window.location.host}/ws?token=${token}`
      this.ws = new WebSocket(url)
      this.ws.onopen = () => {
        this.connected = true
        if (this.reconnectTimer) { clearInterval(this.reconnectTimer); this.reconnectTimer = null }
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
      this.ws.onerror = () => { this.connected = false }
      this.ws.onclose = () => {
        this.connected = false
        if (!this.reconnectTimer) {
          this.reconnectTimer = setInterval(() => this.connect(token), 5000)
        }
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
    disconnect() {
      if (this.ws) this.ws.close()
      this.connected = false
      this.listeners = {}
    }
  }
})
