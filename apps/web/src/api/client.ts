import axios from 'axios'

let csrfToken = ''
let unauthorizedHandler: (() => void) | undefined

export function setCsrfToken(value: string) {
  csrfToken = value
}

export function setUnauthorizedHandler(handler: () => void) {
  unauthorizedHandler = handler
}

export function apiErrorMessage(error: unknown) {
  if (!axios.isAxiosError(error)) return '操作失败，请稍后重试'
  const body = error.response?.data as
    | { error?: { message?: string; field_errors?: Array<{ message: string }> } }
    | undefined
  const fieldMessage = body?.error?.field_errors?.[0]?.message
  return fieldMessage || body?.error?.message || '操作失败，请稍后重试'
}

export const apiClient = axios.create({
  baseURL: '/',
  timeout: 15_000,
  withCredentials: true,
  headers: {
    Accept: 'application/json',
  },
})

apiClient.interceptors.request.use((config) => {
  if (csrfToken && config.method && !['get', 'head', 'options'].includes(config.method)) {
    config.headers.set('X-CSRF-Token', csrfToken)
  }
  return config
})

apiClient.interceptors.response.use(
  (response) => response,
  (error: unknown) => {
    if (axios.isAxiosError(error) && error.response?.status === 401) {
      const url = error.config?.url || ''
      if (!url.endsWith('/auth/login') && !url.endsWith('/auth/session')) {
        unauthorizedHandler?.()
      }
    }
    return Promise.reject(error)
  },
)
