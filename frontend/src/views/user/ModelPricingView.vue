<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="space-y-4">
          <div class="flex items-start gap-3 rounded-xl border border-primary-100 bg-primary-50/60 px-4 py-3 text-sm text-primary-800 dark:border-primary-800/40 dark:bg-primary-900/10 dark:text-primary-200">
            <Icon name="infoCircle" size="md" class="mt-0.5 shrink-0" />
            <p>{{ t('modelPricing.hint') }}</p>
          </div>
          <div class="flex flex-wrap items-center gap-3">
            <div class="relative min-w-48 flex-1 sm:max-w-sm">
              <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model="search" class="input pl-10" :aria-label="t('modelPricing.search')" :placeholder="t('modelPricing.search')" />
            </div>
            <div class="w-40"><Select v-model="platform" :options="platformOptions" :aria-label="t('availableChannels.columns.platform')" /></div>
            <div class="w-40"><Select v-model="mode" :options="modeOptions" :aria-label="t('availableChannels.pricing.billingMode')" /></div>
            <button type="button" class="btn btn-secondary ml-auto" :disabled="loading" :aria-label="t('common.refresh')" @click="loadPricing">
              <Icon name="refresh" size="md" :class="{ 'animate-spin': loading }" />
            </button>
          </div>
        </div>
      </template>
      <template #table>
        <div v-if="loading" class="p-12 text-center text-sm text-gray-500" role="status">{{ t('common.loading') }}</div>
        <div v-else-if="error" class="space-y-3 p-12 text-center text-sm text-gray-500" role="alert">
          <p>{{ error }}</p>
          <button type="button" class="btn btn-secondary" @click="loadPricing">{{ t('common.refresh') }}</button>
        </div>
        <div v-else-if="!filteredRows.length" class="p-12 text-center text-sm text-gray-500">
          {{ t(rows.length ? 'modelPricing.noResults' : 'modelPricing.empty') }}
        </div>
        <div v-else class="table-wrapper">
          <table>
            <thead class="sticky top-0 z-10">
              <tr>
                <th>{{ t('modelPricing.model') }}</th>
                <th>{{ t('availableChannels.columns.name') }}</th>
                <th>{{ t('availableChannels.pricing.billingMode') }}</th>
                <th>{{ t('modelPricing.basePrice') }}</th>
                <th><span class="sr-only">{{ t('modelPricing.details') }}</span></th>
              </tr>
            </thead>
            <tbody>
              <template v-for="row in pagedRows" :key="row.key">
                <tr class="transition-colors hover:bg-gray-50/60 dark:hover:bg-dark-700/30">
                  <td class="align-top">
                    <div class="max-w-72 break-all font-semibold text-gray-900 dark:text-gray-100">{{ row.model }}</div>
                    <div class="mt-2 flex items-center gap-2">
                      <span class="rounded-md border px-2 py-0.5 text-[10px] font-medium uppercase" :class="platformBadgeClass(row.pricing.platform)">{{ row.pricing.platform }}</span>
                      <span v-if="row.model.includes('*')" class="text-xs text-gray-400">{{ t('modelPricing.pattern') }}</span>
                    </div>
                  </td>
                  <td class="max-w-48 break-words align-top">{{ row.channel }}</td>
                  <td class="whitespace-nowrap align-top"><span class="rounded-md bg-gray-100 px-2 py-1 text-xs dark:bg-dark-700">{{ modeLabel(row.pricing.billing_mode) }}</span></td>
                  <td class="min-w-80 align-top"><ModelPricingRates :pricing="row.pricing" /></td>
                  <td class="align-top">
                    <button v-if="hasDetails(row.pricing)" type="button" class="flex items-center gap-1 whitespace-nowrap text-xs font-medium text-primary-600 hover:text-primary-500 dark:text-primary-400" :aria-expanded="expanded.has(row.key)" :aria-controls="`pricing-${row.key}`" @click="toggleDetails(row.key)">
                      {{ t('modelPricing.details') }}
                      <Icon name="chevronDown" size="sm" :class="{ 'rotate-180': expanded.has(row.key) }" />
                    </button>
                  </td>
                </tr>
                <tr v-if="expanded.has(row.key)" :id="`pricing-${row.key}`">
                  <td colspan="5" class="!bg-gray-50/70 dark:!bg-dark-900/40">
                    <div class="space-y-5">
                      <section v-if="row.pricing.intervals.length" class="space-y-3">
                        <h3 class="text-xs font-semibold">{{ t('availableChannels.pricing.intervals') }}</h3>
                        <p class="text-xs text-gray-500">{{ t('modelPricing.tierHint') }}</p>
                        <div class="grid gap-3 lg:grid-cols-2">
                          <div v-for="(iv, index) in row.pricing.intervals" :key="index" class="space-y-3 rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
                            <div class="text-xs font-medium">{{ intervalLabel(iv, row.pricing) }}</div>
                            <ModelPricingRates :pricing="intervalPricing(iv, row.pricing)" />
                            <div v-if="intervalMultipliers(iv).length" class="flex flex-wrap gap-2 text-xs text-gray-500">
                              <span v-for="item in intervalMultipliers(iv)" :key="item.label">{{ item.label }} ×{{ item.value }}</span>
                            </div>
                          </div>
                        </div>
                      </section>
                      <section v-if="multipliers(row.pricing).length" class="space-y-2">
                        <h3 class="text-xs font-semibold">{{ t('modelPricing.multipliers') }}</h3>
                        <div class="flex flex-wrap gap-2">
                          <span v-for="item in multipliers(row.pricing)" :key="item.label" class="rounded-md border border-gray-200 px-3 py-1.5 text-xs dark:border-dark-600">{{ item.label }} <strong class="ml-2 font-mono">×{{ item.value }}</strong></span>
                        </div>
                      </section>
                      <section v-if="row.pricing.time_pricing" class="space-y-2 text-xs">
                        <h3 class="font-semibold">{{ t('modelPricing.timePricing') }} <span class="ml-2 font-normal text-gray-500">{{ row.pricing.time_pricing.timezone }} · {{ t(row.pricing.time_pricing.weekdays_only ? 'modelPricing.weekdays' : 'modelPricing.everyday') }}</span></h3>
                        <div class="flex flex-wrap gap-2">
                          <span v-for="(period, index) in row.pricing.time_pricing.periods" :key="index" class="rounded-md border border-gray-200 px-3 py-1.5 dark:border-dark-600">{{ period.start_time }}–{{ period.end_time }} <strong class="ml-2 font-mono">×{{ period.multiplier }}</strong></span>
                        </div>
                      </section>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </template>
      <template v-if="!loading && !error && filteredRows.length" #pagination>
        <Pagination v-model:page="page" v-model:page-size="pageSize" :total="filteredRows.length" />
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import Pagination from '@/components/common/Pagination.vue'
import ModelPricingRates from '@/components/channels/ModelPricingRates.vue'
import { getModelPricing, type UserChannelPricing, type UserModelPricing, type UserPricingInterval } from '@/api/channels'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { resolveIntervalPrices } from '@/utils/pricing'
import { extractApiErrorMessage } from '@/utils/apiError'
import { platformBadgeClass } from '@/utils/platformColors'

const { t } = useI18n()
const channels = ref<UserChannelPricing[]>([])
const loading = ref(true)
const error = ref('')
const search = ref('')
const platform = ref('')
const mode = ref('')
const page = ref(1)
const pageSize = ref(getPersistedPageSize())
const expanded = ref(new Set<string>())
const modes = ['token', 'per_request', 'image', 'video']
const modeKeys: Record<string, string> = { token: 'billingModeToken', per_request: 'billingModePerRequest', image: 'billingModeImage', video: 'billingModeVideo' }
const modeLabel = (value: string) => t(`availableChannels.pricing.${modeKeys[value]}`)
const modeOptions = computed(() => [{ value: '', label: t('modelPricing.allModes') }, ...modes.map(value => ({ value, label: modeLabel(value) }))])
const platformOptions = computed(() => [
  { value: '', label: t('modelPricing.allPlatforms') },
  ...[...new Set(channels.value.flatMap(ch => ch.model_pricing.map(p => p.platform)))].sort().map(value => ({ value, label: value }))
])
const rows = computed(() => channels.value.flatMap((ch, ci) => ch.model_pricing.flatMap((pricing, pi) => pricing.models.map((model, mi) => ({ key: `${ci}-${pi}-${mi}`, channel: ch.name, model, pricing })))))
const filteredRows = computed(() => {
  const q = search.value.trim().toLowerCase()
  return rows.value.filter(row => (!platform.value || row.pricing.platform === platform.value) && (!mode.value || row.pricing.billing_mode === mode.value) && (!q || `${row.model} ${row.channel} ${row.pricing.platform}`.toLowerCase().includes(q)))
})
const pagedRows = computed(() => filteredRows.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
watch([search, platform, mode, pageSize], () => { page.value = 1 })

function toggleDetails(key: string) {
  if (expanded.value.has(key)) expanded.value.delete(key)
  else expanded.value.add(key)
}
function multipliers(p: UserModelPricing) {
  const values: { label: string; value: number }[] = []
  if (p.fast_multiplier != null) values.push({ label: 'Fast', value: p.fast_multiplier })
  if (p.flex_multiplier != null) values.push({ label: 'Flex', value: p.flex_multiplier })
  for (const [effort, value] of Object.entries(p.reasoning_effort_multipliers ?? {})) values.push({ label: `${t('modelPricing.reasoning')} · ${effort}`, value })
  return values
}
function intervalMultipliers(iv: UserPricingInterval) {
  const values = [
    ['inputPrice', iv.input_multiplier], ['outputPrice', iv.output_multiplier],
    ['cacheWritePrice', iv.cache_write_multiplier], ['cacheReadPrice', iv.cache_read_multiplier]
  ] as const
  return values.flatMap(([key, value]) => value == null ? [] : [{ label: t(`availableChannels.pricing.${key}`), value }])
}
function hasDetails(p: UserModelPricing) {
  return p.intervals.length > 0 || multipliers(p).length > 0 || p.time_pricing != null
}
function intervalLabel(iv: UserPricingInterval, p: UserModelPricing) {
  const range = `(${iv.min_tokens.toLocaleString()}, ${iv.max_tokens == null ? '∞' : iv.max_tokens.toLocaleString()}] tokens`
  return p.billing_mode === 'token' ? [iv.tier_label, range].filter(Boolean).join(' · ') : iv.tier_label || range
}
function intervalPricing(iv: UserPricingInterval, base: UserModelPricing) {
  return { ...base, ...resolveIntervalPrices(iv, base), per_request_price: iv.per_request_price ?? base.per_request_price }
}
async function loadPricing() {
  loading.value = true
  error.value = ''
  try {
    channels.value = await getModelPricing()
    page.value = 1
    expanded.value.clear()
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('common.error'))
  } finally {
    loading.value = false
  }
}
onMounted(loadPricing)
</script>
