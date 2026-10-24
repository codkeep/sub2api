import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import UsageProxyCell from './UsageProxyCell.vue'
import { getUsageIPLocation } from '@/api/admin/usage'

vi.mock('@/api/admin/usage', () => ({ getUsageIPLocation: vi.fn() }))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string, values?: { time?: string }) => {
    const messages: Record<string, string> = {
      'admin.usage.proxyExitIP': '出口 IP（探测）',
      'admin.usage.proxyIPUnavailable': '未获取',
      'admin.usage.proxyNotRecorded': '未记录',
      'admin.usage.proxyDirect': '直连',
    }
    return key === 'admin.usage.proxyIPHint' ? `探测时间：${values?.time}` : messages[key] || key
  } }),
}))

describe('UsageProxyCell', () => {
  it('looks up the recorded IP only on click and allows retry after failure', async () => {
    const lookup = vi.mocked(getUsageIPLocation)
    lookup.mockReset()
    lookup.mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce({ ip: '8.8.8.8', country: 'United States', region: 'California', city: 'Mountain View', isp: 'Google' })
    const wrapper = mount(UsageProxyCell, { props: { proxy: { id: 1, exit_ip: '8.8.8.8' } } })
    expect(lookup).not.toHaveBeenCalled()
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('admin.usage.proxyIPLookupFailed')
    await wrapper.findAll('button')[1]!.trigger('click')
    await flushPromises()
    expect(lookup).toHaveBeenLastCalledWith('8.8.8.8')
    expect(wrapper.text()).toContain('United States · California · Mountain View')
    expect(wrapper.text()).toContain('Google')
    await wrapper.setProps({ proxy: { id: 2, exit_ip: '1.1.1.1' } })
    expect(wrapper.text()).not.toContain('Google')
  })
  it('shows the recorded proxy name and exit observation with its timestamp', () => {
    const wrapper = mount(UsageProxyCell, { props: { proxy: { id: 42, name: '代理 A', exit_ip: '203.0.113.7', exit_ip_checked_at: '2026-10-24T00:00:00Z' } } })
    expect(wrapper.text()).toContain('代理 A')
    expect(wrapper.text()).toContain('203.0.113.7')
    expect(wrapper.text()).toContain('出口 IP（探测）')
    expect(wrapper.get('[title]').attributes('title')).toContain('2026')
  })

  it('distinguishes historical records, direct requests, and failed probes', () => {
    for (const [proxy, text] of [[null, '未记录'], [{}, '直连'], [{ id: 1, name: '代理 B' }, '未获取']] as const) {
      const wrapper = mount(UsageProxyCell, { props: { proxy } })
      expect(wrapper.text()).toContain(text)
    }
  })
})
