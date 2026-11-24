import { beforeEach, describe, expect, it, mock } from 'bun:test'

import { APIError, apiRequest } from '../src/lib/api'
import { login, me } from '../src/features/auth/api'

describe('Starpay API client', () => {
  const fetchMock = mock<typeof fetch>()

  beforeEach(() => {
    fetchMock.mockReset()
    globalThis.fetch = fetchMock
  })

  it('unwraps a successful API envelope and sends bearer credentials', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 'ok',
          message: 'ok',
          data: { id: 7 },
          error: null,
        }),
        { status: 200, headers: { 'content-type': 'application/json' } },
      ),
    )

    await expect(
      apiRequest<{ id: number }>('/v1/admin/apps', {
        method: 'POST',
        accessToken: 'access-token',
        body: JSON.stringify({ name: 'demo' }),
      }),
    ).resolves.toEqual({ id: 7 })

    const [input, init] = fetchMock.mock.calls[0] ?? []
    expect(input).toBe('/v1/admin/apps')
    expect(init?.credentials).toBe('include')
    expect(new Headers(init?.headers).get('authorization')).toBe(
      'Bearer access-token',
    )
    expect(new Headers(init?.headers).get('content-type')).toBe(
      'application/json',
    )
  })

  it('turns the gateway error envelope into a typed APIError', async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          code: 'invalid_request',
          message: 'invalid request',
          data: null,
          error: {
            code: 'invalid_request',
            message: 'merchant_order_no is required',
            details: { field: 'merchant_order_no' },
          },
        }),
        { status: 400, headers: { 'content-type': 'application/json' } },
      ),
    )

    let error: unknown
    try {
      await apiRequest('/v1/admin/orders', { method: 'POST' })
    } catch (cause) {
      error = cause
    }

    expect(error).toBeInstanceOf(APIError)
    expect(error).toMatchObject({
      status: 400,
      code: 'invalid_request',
      message: 'merchant_order_no is required',
      details: { field: 'merchant_order_no' },
    })
  })

  it('preserves cookie auth for login and adds bearer auth for me', async () => {
    fetchMock
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            code: 'ok',
            message: 'ok',
            data: {
              access_token: 'new-token',
              expires_at: '2030-01-01T00:00:00Z',
              user: { id: 1 },
              roles: ['super_admin'],
            },
            error: null,
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            code: 'ok',
            message: 'ok',
            data: { user: { id: 1 } },
            error: null,
          }),
          { status: 200 },
        ),
      )

    await login({ username: 'admin', password: 'password' })
    await me('access-token')

    const [, loginInit] = fetchMock.mock.calls[0] ?? []
    const [meInput, meInit] = fetchMock.mock.calls[1] ?? []
    expect(loginInit?.credentials).toBe('include')
    expect(new Headers(loginInit?.headers).get('authorization')).toBeNull()
    expect(meInput).toBe('/v1/admin/auth/me')
    expect(new Headers(meInit?.headers).get('authorization')).toBe(
      'Bearer access-token',
    )
  })
})
