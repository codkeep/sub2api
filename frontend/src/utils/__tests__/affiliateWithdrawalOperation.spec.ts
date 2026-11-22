import { webcrypto } from 'node:crypto'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { WithdrawalInput } from '@/api/affiliateWithdrawals'

const input: WithdrawalInput = {
  amount: 150, payout_method: 'bank', payout_account: 'private-account', payout_name: 'private-name', user_note: '',
}
describe('withdrawal idempotency keys', () => {
  beforeEach(() => {
    sessionStorage.clear()
    vi.resetModules()
    vi.stubGlobal('crypto', webcrypto)
  })
  afterEach(() => vi.unstubAllGlobals())
  it('restores the same key after a page reload without storing payout details', async () => {
    const first = await import('../affiliateWithdrawalOperation')
    const operation = await first.prepareWithdrawalOperation(7, input)
    expect(sessionStorage.length).toBe(1)
    const storageKey = sessionStorage.key(0)!
    expect(storageKey).not.toContain(input.payout_account)
    expect(sessionStorage.getItem(storageKey)).toBe(operation.key)
    expect(sessionStorage.getItem(storageKey)).not.toContain(input.payout_name)
    vi.resetModules()
    const second = await import('../affiliateWithdrawalOperation')
    expect((await second.prepareWithdrawalOperation(7, input)).key).toBe(operation.key)
  })
  it('uses different keys for different users and payout details', async () => {
    const { prepareWithdrawalOperation } = await import('../affiliateWithdrawalOperation')
    const original = await prepareWithdrawalOperation(7, input)
    const otherUser = await prepareWithdrawalOperation(8, input)
    const otherAccount = await prepareWithdrawalOperation(7, { ...input, payout_account: 'another' })
    expect(new Set([original.key, otherUser.key, otherAccount.key]).size).toBe(3)
  })
  it('releases a completed operation so a later legitimate request gets a new key', async () => {
    const { prepareWithdrawalOperation, completeWithdrawalOperation } = await import('../affiliateWithdrawalOperation')
    const original = await prepareWithdrawalOperation(7, input)
    completeWithdrawalOperation(7, input, original)
    expect(sessionStorage.length).toBe(0)
    expect((await prepareWithdrawalOperation(7, input)).key).not.toBe(original.key)
  })
  it('still protects retries when secure browser storage is unavailable', async () => {
    vi.stubGlobal('crypto', undefined)
    const { prepareWithdrawalOperation } = await import('../affiliateWithdrawalOperation')
    const original = await prepareWithdrawalOperation(7, input)
    expect((await prepareWithdrawalOperation(7, input)).key).toBe(original.key)
  })
})
