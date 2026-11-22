import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import AffiliateWithdrawalsPanel from '../AffiliateWithdrawalsPanel.vue'

const { getPolicy, list, apply, showError, showSuccess } = vi.hoisted(() => ({
  getPolicy: vi.fn(), list: vi.fn(), apply: vi.fn(), showError: vi.fn(), showSuccess: vi.fn(),
}))
vi.mock('@/api/affiliateWithdrawals', () => ({ affiliateWithdrawalsAPI: { getPolicy, list, apply } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 7 } }) }))

async function mountPanel(availableQuota = 200) {
  const wrapper = mount(AffiliateWithdrawalsPanel, {
    props: { availableQuota },
    global: {
      plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })],
      stubs: { Pagination: true },
    },
  })
  await flushPromises()
  return wrapper
}
async function fill(wrapper: VueWrapper, amount = '150') {
  await wrapper.get('[data-test="amount"]').setValue(amount)
  await wrapper.get('[data-test="method"]').setValue(' bank ')
  await wrapper.get('[data-test="account"]').setValue(' 1234 ')
  await wrapper.get('[data-test="name"]').setValue(' 张三 ')
  await wrapper.get('[data-test="note"]').setValue(' note ')
}
describe('affiliate withdrawal requests', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    getPolicy.mockResolvedValue({ minimum_amount: 100 })
    list.mockResolvedValue({ items: [], total: 0 })
    apply.mockResolvedValue({ id: 11, status: 'pending' })
  })
  it('shows the minimum and disables requests when available rebates are below it', async () => {
    const wrapper = await mountPanel(99)
    expect(wrapper.text()).toContain('最低提现金额')
    expect(wrapper.get('[data-test="apply"]').attributes('disabled')).toBeDefined()
    await fill(wrapper, '99')
    await wrapper.get('form').trigger('submit')
    expect(apply).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('最低提现金额')
    wrapper.unmount()
  })
  it('checks amount and required payout details before sending a request', async () => {
    const wrapper = await mountPanel()
    await fill(wrapper, '201')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.text()).toContain('提现金额不能超过')
    await wrapper.get('[data-test="amount"]').setValue('150')
    await wrapper.get('[data-test="account"]').setValue(' ')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.text()).toContain('请填写收款方式')
    expect(apply).not.toHaveBeenCalled()
    wrapper.unmount()
  })
  it('submits trimmed details and refreshes the available quota and records after success', async () => {
    const wrapper = await mountPanel()
    await fill(wrapper)
    await wrapper.get('form').trigger('submit')
    await vi.waitFor(() => expect(apply).toHaveBeenCalledTimes(1))
    await flushPromises()
    expect(apply).toHaveBeenCalledWith({
      amount: 150, payout_method: 'bank', payout_account: '1234', payout_name: '张三', user_note: 'note',
    }, expect.any(String))
    expect(wrapper.emitted('updated')).toHaveLength(1)
    expect(list).toHaveBeenCalledTimes(2)
    expect(showSuccess).toHaveBeenCalledOnce()
    expect((wrapper.get('[data-test="amount"]').element as HTMLInputElement).value).toBe('')
    wrapper.unmount()
  })
  it('locks an uncertain request and retries the same payload and key even after the quota decreases', async () => {
    apply.mockRejectedValueOnce({ status: 503, message: 'temporarily unavailable' })
    const wrapper = await mountPanel()
    await fill(wrapper)
    await wrapper.get('form').trigger('submit')
    await vi.waitFor(() => expect(wrapper.text()).toContain('上次提交结果未知'))
    const firstCall = apply.mock.calls[0]
    expect(wrapper.get('fieldset').attributes('disabled')).toBeDefined()
    await wrapper.setProps({ availableQuota: 20 })
    expect(wrapper.get('[data-test="apply"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(apply).toHaveBeenCalledTimes(2)
    expect(apply.mock.calls[1]).toEqual(firstCall)
    expect(wrapper.emitted('updated')).toHaveLength(1)
    expect(wrapper.text()).not.toContain('上次提交结果未知')
    wrapper.unmount()
  })
  it('shows a specific minimum error and reloads the policy when it changes', async () => {
    apply.mockRejectedValue({ status: 400, reason: 'AFFILIATE_WITHDRAWAL_BELOW_MINIMUM', metadata: { minimum: '200' } })
    const wrapper = await mountPanel()
    await fill(wrapper)
    await wrapper.get('form').trigger('submit')
    await vi.waitFor(() => expect(wrapper.text()).toContain('未达到最低提现金额：USD 200'))
    expect(getPolicy).toHaveBeenCalledTimes(2)
    expect(wrapper.get('fieldset').attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })
  it('displays the rejection reason and payout reference in the user records', async () => {
    list.mockResolvedValue({ total: 1, items: [{
      id: 11, amount: 150, status: 'rejected', payout_method: 'bank', payout_account: '1234', payout_name: '张三',
      admin_note: '收款账号有误', payment_reference: 'ref-11', created_at: '2026-11-23T08:00:00Z',
    }] })
    const wrapper = await mountPanel()
    expect(wrapper.text()).toContain('已拒绝（额度已退回）')
    expect(wrapper.text()).toContain('收款账号有误')
    expect(wrapper.text()).toContain('ref-11')
    expect(list).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    wrapper.unmount()
  })
})
