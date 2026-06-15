import axios from 'axios'
import { message } from 'ant-design-vue'
import router from '@/router'

const instance = axios.create({
  baseURL: '/api',
  timeout: 30000
})

instance.interceptors.request.use(config => {
  const token = localStorage.getItem('token') || sessionStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
}, error => Promise.reject(error))

instance.interceptors.response.use(
  res => res.data,
  error => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      sessionStorage.removeItem('token')
      message.error('登录已过期，请重新登录')
      router.push('/login')
    } else if (error.response?.data?.error) {
      message.error(error.response.data.error)
    } else {
      message.error(error.message || '请求失败')
    }
    return Promise.reject(error)
  }
)

export default instance
