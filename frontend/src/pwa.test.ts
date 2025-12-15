import { describe, expect, it, vi } from 'vitest'

vi.mock('virtual:pwa-register', () => {
  return { registerSW: vi.fn() }
})

describe('registerPWA', () => {
  it('registers the service worker immediately', async () => {
    const { registerSW } = await import('virtual:pwa-register')
    const { registerPWA } = await import('./pwa')

    registerPWA()

    expect(registerSW).toHaveBeenCalledWith({ immediate: true })
  })
})
