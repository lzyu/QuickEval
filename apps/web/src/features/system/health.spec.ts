import { describe, expect, it, vi } from 'vitest'

import { apiClient } from '@/api/client'
import { fetchReadiness } from './health'

describe('fetchReadiness', () => {
  it('returns the API envelope', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValueOnce({
      data: {
        data: {
          status: 'ok',
          dependencies: {
            mysql: 'ok',
            redis: 'ok',
          },
        },
        meta: {
          request_id: 'req-test',
        },
      },
    })

    await expect(fetchReadiness()).resolves.toMatchObject({
      data: {
        status: 'ok',
      },
      meta: {
        request_id: 'req-test',
      },
    })
  })
})
