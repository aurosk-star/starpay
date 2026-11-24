import { toast } from 'sonner'

import i18n from '@/i18n'
import { APIError } from './api'

export function handleServerError(error: unknown) {
  if (import.meta.env.DEV) {
    // eslint-disable-next-line no-console
    console.log(error)
  }

  let errMsg = i18n.t('errors.api.unknown')

  if (error instanceof APIError) {
    errMsg = error.message
  } else if (error instanceof TypeError) {
    // fetch 在网络异常时抛出 TypeError
    errMsg = i18n.t('errors.api.network')
  } else if (error instanceof Error && error.message) {
    errMsg = error.message
  }

  toast.error(errMsg)
}
