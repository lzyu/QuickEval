import { defineStore } from 'pinia'

import { apiClient, setCsrfToken } from '@/api/client'
import type { ResponseEnvelope, SessionPayload, User } from '@/api/types'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as User | null,
    permissions: {} as Record<string, boolean>,
    initialized: false,
  }),
  getters: {
    isAuthenticated: (state) => state.user !== null,
    isAdmin: (state) => state.user?.role === 'admin',
  },
  actions: {
    apply(payload: SessionPayload) {
      this.user = payload.user
      this.permissions = payload.permissions
      setCsrfToken(payload.csrf_token)
    },
    clear() {
      this.user = null
      this.permissions = {}
      setCsrfToken('')
    },
    async restore() {
      if (this.initialized) return
      try {
        const response =
          await apiClient.get<ResponseEnvelope<SessionPayload>>('/api/v1/auth/session')
        this.apply(response.data.data)
      } catch {
        this.clear()
      } finally {
        this.initialized = true
      }
    },
    async login(username: string, password: string) {
      const response = await apiClient.post<ResponseEnvelope<SessionPayload>>(
        '/api/v1/auth/login',
        { username, password },
      )
      this.apply(response.data.data)
      this.initialized = true
    },
    async logout() {
      try {
        await apiClient.delete('/api/v1/auth/session')
      } finally {
        this.clear()
      }
    },
  },
})
