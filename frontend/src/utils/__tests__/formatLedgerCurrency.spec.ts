import { describe, expect, it, vi } from 'vitest'
import { formatLedgerCurrency } from '../format'

vi.mock('@/i18n', () => ({ getLocale: () => 'en', i18n: { global: { t: (key: string) => key } } }))
describe('withdrawal amounts', () => {
  it('keeps every ledger decimal visible for payment reconciliation', () => {
    expect(formatLedgerCurrency(123.12345678)).toBe('$123.12345678')
    expect(formatLedgerCurrency(0.00000001)).toBe('$0.00000001')
    expect(formatLedgerCurrency(100)).toBe('$100.00')
  })
})
