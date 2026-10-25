import { computed } from 'vue'
import { useAppStore } from '@/stores/app'
import { formatChannelPrice } from '@/utils/pricing'

export interface QuotaGroup { pricing_unit?: string; subscription_type?: string }
export function useSubscriptionUnits() {
  const appStore = useAppStore()
  return computed(() => Array.isArray(appStore.cachedPublicSettings?.subscription_units)
    ? appStore.cachedPublicSettings.subscription_units
    : [{ id: 'points', name: appStore.cachedPublicSettings?.subscription_unit_name?.trim() || '积分', units_per_usd: 1 }])
}
export function resolveQuotaUnit(group?: QuotaGroup | null): string {
  return group?.pricing_unit ?? (group?.subscription_type === 'subscription' ? 'points' : 'usd')
}
export function useSubscriptionUnit() {
  const units = useSubscriptionUnits()
  return computed(() => units.value.find(u => u.id === 'points')?.name || units.value[0]?.name || '积分')
}
export function useQuotaDisplay() {
  const units = useSubscriptionUnits()
  return (value: number | null | undefined, group?: QuotaGroup | null) => {
    const id = resolveQuotaUnit(group)
    if (id === 'usd') return formatChannelPrice(value ?? 0, 1, 'usd', '')
    const unit = units.value.find(u => u.id === id)
    return formatChannelPrice(value ?? 0, 1, 'points', unit?.name || id, unit?.units_per_usd || 1)
  }
}

export function useUnitPrice() {
  const units = useSubscriptionUnits()
  return (value: number | null, scale: number, ids: string[]) => {
    if (value == null) return '-'
    return [...new Set(ids.length ? ids : ['usd'])].map(id => {
      if (id === 'usd') return formatChannelPrice(value, scale, 'usd', '')
      const unit = units.value.find(u => u.id === id)
      return formatChannelPrice(value, scale, 'points', unit?.name || id, unit?.units_per_usd || 1)
    }).join(' · ')
  }
}

export function formatQuotaPercentage(used: number | null | undefined, limit: number | null | undefined): string {
  if (limit == null || limit <= 0 || !Number.isFinite(limit)) return '—'
  return `${Math.max(0, (used || 0) / limit * 100).toFixed(1).replace(/\.0$/, '')}%`
}
export function useQuotaUsageDisplay() {
  const appStore = useAppStore()
  const format = useQuotaDisplay()
  return (used: number | null | undefined, limit: number | null | undefined, group?: QuotaGroup | null) =>
    appStore.cachedPublicSettings?.subscription_usage_display === 'percentage'
      ? formatQuotaPercentage(used, limit)
      : `${format(used, group)} / ${limit == null ? '∞' : format(limit, group)}`
}
