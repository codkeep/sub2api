import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'

export type WithdrawalStatus = 'pending' | 'processing' | 'paid' | 'rejected'
export interface WithdrawalPolicy { minimum_amount: number }
export interface WithdrawalInput {
  amount: number
  payout_method: string
  payout_account: string
  payout_name: string
  user_note: string
}
export interface WithdrawalRecord extends WithdrawalInput {
  id: number
  user_id: number
  user_email?: string
  username?: string
  minimum_amount: number
  status: WithdrawalStatus
  admin_note: string
  payment_reference: string
  processed_by?: number
  processed_at?: string
  created_at: string
  updated_at: string
}
export interface WithdrawalReview {
  status: Exclude<WithdrawalStatus, 'pending'>
  admin_note: string
  payment_reference: string
}
export interface WithdrawalListParams {
  page?: number
  page_size?: number
  status?: WithdrawalStatus | ''
  search?: string
}

const base = (admin: boolean) => admin ? '/admin/affiliates' : '/user/aff'
export const affiliateWithdrawalsAPI = {
  async getPolicy(admin = false): Promise<WithdrawalPolicy> {
    return (await apiClient.get<WithdrawalPolicy>(`${base(admin)}/withdrawal-policy`)).data
  },
  async setPolicy(policy: WithdrawalPolicy): Promise<WithdrawalPolicy> {
    return (await apiClient.put<WithdrawalPolicy>(`${base(true)}/withdrawal-policy`, policy)).data
  },
  async list(params: WithdrawalListParams = {}, admin = false): Promise<PaginatedResponse<WithdrawalRecord>> {
    return (await apiClient.get<PaginatedResponse<WithdrawalRecord>>(`${base(admin)}/withdrawals`, { params })).data
  },
  async apply(input: WithdrawalInput, key: string): Promise<WithdrawalRecord> {
    return (await apiClient.post<WithdrawalRecord>(`${base(false)}/withdrawals`, input, {
      headers: { 'Idempotency-Key': key },
    })).data
  },
  async review(id: number, input: WithdrawalReview): Promise<WithdrawalRecord> {
    return (await apiClient.post<WithdrawalRecord>(`${base(true)}/withdrawals/${id}/review`, input)).data
  },
}
