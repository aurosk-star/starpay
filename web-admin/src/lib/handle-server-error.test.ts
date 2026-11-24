import { beforeEach, describe, expect, it, vi } from 'vitest'
import { APIError } from './api'
import { handleServerError } from './handle-server-error'

const toastError = vi.hoisted(() => vi.fn())

vi.mock('sonner', () => ({
  toast: {
    error: toastError,
  },
}))

vi.mock('@/i18n', () => ({
  default: {
    t: (key: string) => key,
  },
}))

beforeEach(() => {
  vi.mocked(toastError).mockClear()
})

describe('handleServerError', () => {
  it('shows a generic message when the error is not recognised', () => {
    handleServerError(undefined)

    expect(toastError).toHaveBeenCalledWith('errors.api.unknown')
  })

  it('prefers the API error message', () => {
    handleServerError(new APIError(422, 'validation_failed', '用户名已存在'))

    expect(toastError).toHaveBeenCalledWith('用户名已存在')
  })

  it('maps network failures to the network message', () => {
    handleServerError(new TypeError('Failed to fetch'))

    expect(toastError).toHaveBeenCalledWith('errors.api.network')
  })

  it('falls back to the error message for plain errors', () => {
    handleServerError(new Error('boom'))

    expect(toastError).toHaveBeenCalledWith('boom')
  })

  it('logs the error to the console in development', () => {
    const log = vi.spyOn(console, 'log').mockImplementation(() => {})
    const err = new Error('logged')

    handleServerError(err)

    expect(log).toHaveBeenCalledTimes(1)
    expect(log).toHaveBeenCalledWith(err)

    log.mockRestore()
  })

  it('does not log the error to the console in production', () => {
    vi.stubEnv('DEV', false)

    const log = vi.spyOn(console, 'log').mockImplementation(() => {})
    const err = new Error('not logged')

    handleServerError(err)

    expect(log).not.toHaveBeenCalled()

    log.mockRestore()
  })
})
