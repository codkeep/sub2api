import type { WithdrawalInput } from '@/api/affiliateWithdrawals'

interface WithdrawalOperation {
  key: string
  storageKey?: string
}
const pendingKeys = new Map<string, string>()

// Store only a request digest and its key; bank details never go into browser storage.
export async function prepareWithdrawalOperation(userID: number, input: WithdrawalInput): Promise<WithdrawalOperation> {
  const identity = `${userID}:${JSON.stringify(input)}`
  let storageKey: string | undefined
  if (globalThis.crypto?.subtle) {
    const bytes = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(identity))
    const digest = Array.from(new Uint8Array(bytes), b => b.toString(16).padStart(2, '0')).join('')
    storageKey = `sub2api:affiliate-withdrawal:${userID}:${digest}`
  }
  let key = pendingKeys.get(identity)
  try { key ??= storageKey ? sessionStorage.getItem(storageKey) ?? undefined : undefined } catch { /* memory fallback */ }
  key ??= `withdrawal-${globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`}`
  pendingKeys.set(identity, key)
  try { if (storageKey) sessionStorage.setItem(storageKey, key) } catch { /* memory fallback */ }
  return { key, storageKey }
}

export function completeWithdrawalOperation(userID: number, input: WithdrawalInput, operation: WithdrawalOperation) {
  pendingKeys.delete(`${userID}:${JSON.stringify(input)}`)
  try { if (operation.storageKey) sessionStorage.removeItem(operation.storageKey) } catch { /* storage unavailable */ }
}
