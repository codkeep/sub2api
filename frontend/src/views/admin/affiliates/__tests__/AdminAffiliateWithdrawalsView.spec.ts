import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import AdminAffiliateWithdrawalsView from '../AdminAffiliateWithdrawalsView.vue'

const { getPolicy, setPolicy, list, review, showError, showSuccess } = vi.hoisted(() => ({
  getPolicy: vi.fn(), setPolicy: vi.fn(), list: vi.fn(), review: vi.fn(), showError: vi.fn(), showSuccess: vi.fn(),
}))
vi.mock('@/api/affiliateWithdrawals', () => ({ affiliateWithdrawalsAPI: { getPolicy, setPolicy, list, review } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 9 } }) }))
const record = (status = 'pending', adminID?: number) => ({
  id: 11, user_id: 7, user_email: 'u@example.com', username: 'user', amount: 150, minimum_amount: 100,
  payout_method: 'bank', payout_account: '1234', payout_name: '张三', user_note: 'note', status,
  admin_note: '', payment_reference: '', processed_by: adminID, created_at: '2026-11-23T08:00:00Z',
})
async function mountView() {
  const wrapper = mount(AdminAffiliateWithdrawalsView, {
    global: {
      plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })],
      stubs: {
        AppLayout: { template: '<main><slot /></main>' }, Pagination: true,
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
      },
    },
  })
  await flushPromises()
  return wrapper
}
async function submitReview(wrapper: VueWrapper) {
  await wrapper.get('#withdrawal-review').trigger('submit')
  await flushPromises()
}
describe('admin affiliate withdrawals', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getPolicy.mockResolvedValue({ minimum_amount: 100 })
    setPolicy.mockImplementation(async value => value)
    list.mockResolvedValue({ items: [record()], total: 1 })
    review.mockResolvedValue(record('processing', 9))
  })
  it('loads admin records and permits setting the minimum to zero', async () => {
    const wrapper = await mountView()
    expect(getPolicy).toHaveBeenCalledWith(true)
    expect(list).toHaveBeenCalledWith({ page: 1, page_size: 20, status: '', search: '' }, true)
    await wrapper.get('[data-test="minimum"]').setValue('0')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(setPolicy).toHaveBeenCalledWith({ minimum_amount: 0 })
    expect(showSuccess).toHaveBeenCalledWith('提现设置已保存')
    wrapper.unmount()
  })
  it('claims a pending request before allowing payment', async () => {
    const wrapper = await mountView()
    expect(wrapper.find('[data-test="pay"]').exists()).toBe(false)
    await wrapper.get('[data-test="claim"]').trigger('click')
    await submitReview(wrapper)
    expect(review).toHaveBeenCalledWith(11, { status: 'processing', admin_note: '', payment_reference: '' })
    expect(list).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
  it('requires a payment reference before marking an assigned request as paid', async () => {
    list.mockResolvedValue({ items: [record('processing', 9)], total: 1 })
    const wrapper = await mountView()
    await wrapper.get('[data-test="pay"]').trigger('click')
    await submitReview(wrapper)
    expect(review).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('请填写打款凭证或流水号')
    await wrapper.get('[data-test="reference"]').setValue(' ref-11 ')
    await submitReview(wrapper)
    expect(review).toHaveBeenCalledWith(11, { status: 'paid', admin_note: '', payment_reference: 'ref-11' })
    wrapper.unmount()
  })
  it('requires a rejection reason and sends it to the server for refunding', async () => {
    const wrapper = await mountView()
    await wrapper.get('[data-test="reject"]').trigger('click')
    await submitReview(wrapper)
    expect(review).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('请填写拒绝原因')
    await wrapper.get('[data-test="admin-note"]').setValue(' 收款信息有误 ')
    await submitReview(wrapper)
    expect(review).toHaveBeenCalledWith(11, { status: 'rejected', admin_note: '收款信息有误', payment_reference: '' })
    wrapper.unmount()
  })
  it('leaves another administrator’s processing request read-only', async () => {
    list.mockResolvedValue({ items: [record('processing', 8)], total: 1 })
    const wrapper = await mountView()
    expect(wrapper.find('[data-test="pay"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="reject"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="claim"]').exists()).toBe(false)
    wrapper.unmount()
  })
  it('refreshes after a conflicting review and displays the specific reason', async () => {
    review.mockRejectedValue({ status: 409, reason: 'AFFILIATE_WITHDRAWAL_STATE_CONFLICT' })
    const wrapper = await mountView()
    await wrapper.get('[data-test="claim"]').trigger('click')
    await submitReview(wrapper)
    expect(showError).toHaveBeenCalledWith('申请状态或处理人已变更，请刷新记录后重试')
    expect(list).toHaveBeenCalledTimes(2)
    expect(wrapper.find('#withdrawal-review').exists()).toBe(false)
    wrapper.unmount()
  })
  it('locks an uncertain review and retries identical details', async () => {
    review.mockRejectedValueOnce({ status: 503, message: 'unavailable' })
    const wrapper = await mountView()
    await wrapper.get('[data-test="reject"]').trigger('click')
    await wrapper.get('[data-test="admin-note"]').setValue('reason')
    await submitReview(wrapper)
    expect(wrapper.text()).toContain('上次处理结果未知')
    expect(wrapper.get('fieldset').attributes('disabled')).toBeDefined()
    await submitReview(wrapper)
    expect(review.mock.calls[1]).toEqual(review.mock.calls[0])
    expect(wrapper.find('#withdrawal-review').exists()).toBe(false)
    wrapper.unmount()
  })
  it('passes status and search filters to the admin endpoint', async () => {
    const wrapper = await mountView()
    await wrapper.get('[data-test="search"]').setValue(' u@example.com ')
    await wrapper.get('[data-test="status-filter"]').setValue('paid')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith({ page: 1, page_size: 20, status: 'paid', search: 'u@example.com' }, true)
    wrapper.unmount()
  })
})
