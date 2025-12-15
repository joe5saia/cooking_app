import { registerSW } from 'virtual:pwa-register'

export function registerPWA(): void {
  registerSW({ immediate: true })
}
