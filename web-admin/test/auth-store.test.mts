import { beforeEach, describe, expect, it } from 'bun:test'

import { useAuthStore } from '../src/features/auth/store'

const user = {
  id: 1,
  username: 'admin',
  email: 'admin@example.com',
  display_name: 'Administrator',
  status: 'enabled',
  roles: ['super_admin'],
}

describe('admin auth state', () => {
  const storage = new Map<string, string>()

  beforeEach(() => {
    storage.clear()
    ;(globalThis as typeof globalThis & { window?: Window }).window = {
      localStorage: {
        getItem: (key: string) => storage.get(key) ?? null,
        setItem: (key: string, value: string) => storage.set(key, value),
        removeItem: (key: string) => storage.delete(key),
        clear: () => storage.clear(),
      } as Storage,
    } as Window
    useAuthStore.getState().clearSession()
    useAuthStore.getState().hydrate()
  })

  it('hydrates the persisted Starpay access token', () => {
    storage.set('payment_gateway_access_token', 'saved-token')
    useAuthStore.getState().hydrate()

    expect(useAuthStore.getState().accessToken).toBe('saved-token')
    expect(useAuthStore.getState().hydrated).toBe(true)
  })

  it('keeps the user and token together when starting a session', () => {
    useAuthStore.getState().setSession('session-token', user)

    expect(useAuthStore.getState()).toMatchObject({
      accessToken: 'session-token',
      user,
    })
    expect(storage.get('payment_gateway_access_token')).toBe('session-token')
  })

  it('clears both memory and persistence when the session ends', () => {
    useAuthStore.getState().setSession('session-token', user)
    useAuthStore.getState().clearSession()

    expect(useAuthStore.getState().accessToken).toBeNull()
    expect(useAuthStore.getState().user).toBeNull()
    expect(storage.has('payment_gateway_access_token')).toBe(false)
  })
})
