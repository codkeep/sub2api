<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="space-y-4">
          <div class="flex items-start gap-3 rounded-xl border border-primary-100 bg-primary-50/60 px-4 py-3 text-sm text-primary-800 dark:border-primary-800/40 dark:bg-primary-900/10 dark:text-primary-200">
            <Icon name="infoCircle" size="md" class="mt-0.5 shrink-0" />
            <p>{{ t(selectedGroupId === null ? 'modelPricing.hint' : 'modelPricing.detailHint') }}</p>
          </div>
          <div v-if="selectedGroupId === null" class="flex flex-wrap items-center gap-3">
            <div class="relative min-w-48 flex-1 sm:max-w-sm">
              <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model="groupSearch" class="input pl-10" :aria-label="t('modelPricing.groupSearch')" :placeholder="t('modelPricing.groupSearch')" />
            </div>
            <button type="button" class="btn btn-secondary ml-auto" :disabled="loading" :aria-label="t('common.refresh')" @click="loadPricing">
              <Icon name="refresh" size="md" :class="{ 'animate-spin': loading }" />
            </button>
          </div>
          <div v-else class="flex flex-wrap items-center gap-3">
            <button type="button" class="btn btn-secondary" @click="backToGroups">
              <Icon name="arrowLeft" size="sm" class="mr-1" />{{ t('modelPricing.backToGroups') }}
            </button>
            <div class="flex min-w-0 items-center gap-2 text-sm">
              <span class="truncate font-semibold text-gray-900 dark:text-white">{{ selectedGroup?.name }}</span>
              <span v-if="selectedGroup?.pricingUnit" class="shrink-0 rounded-full bg-primary-50 px-2 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">{{ selectedGroup.pricingUnit }}</span>
            </div>
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
        <div v-else-if="selectedGroupId === null && !filteredGroups.length" class="p-12 text-center text-sm text-gray-500">
          {{ t(groupSummaries.length ? 'modelPricing.noGroupResults' : 'modelPricing.empty') }}
        </div>
        <div v-else-if="selectedGroupId === null" class="grid gap-4 p-5 sm:grid-cols-2 xl:grid-cols-3">
          <button
            v-for="group in filteredGroups"
            :key="group.id"
            type="button"
            class="group rounded-2xl border border-gray-200 bg-white p-5 text-left shadow-sm transition hover:-translate-y-0.5 hover:border-primary-300 hover:shadow-md dark:border-dark-700 dark:bg-dark-800 dark:hover:border-primary-700"
            @click="openGroup(group.id)"
          >
            <div class="flex items-start gap-3">
              <span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300">
                <Icon name="sparkles" size="md" />
              </span>
              <div class="min-w-0 flex-1">
                <div class="flex items-start justify-between gap-2">
                  <h2 class="truncate text-base font-semibold text-gray-900 dark:text-white">{{ group.name }}</h2>
                  <Icon name="chevronRight" size="sm" class="mt-0.5 shrink-0 text-gray-400 transition group-hover:translate-x-0.5 group-hover:text-primary-500" />
                </div>
                <p v-if="group.pricingUnit" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('modelPricing.pricingUnit') }}：{{ group.pricingUnit }}</p>
              </div>
            </div>
            <div class="mt-5 grid grid-cols-2 gap-3">
              <div class="rounded-xl bg-gray-50 px-3 py-2.5 dark:bg-dark-700/60">
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('modelPricing.channelCount') }}</div>
                <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ group.channelCount }}</div>
              </div>
              <div class="rounded-xl bg-gray-50 px-3 py-2.5 dark:bg-dark-700/60">
                <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('modelPricing.modelCount') }}</div>
                <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ group.modelCount }}</div>
              </div>
            </div>
            <div class="mt-4 flex flex-wrap gap-1.5">
              <span v-for="channel in group.channels" :key="channel" class="max-w-full truncate rounded-full border border-gray-200 px-2 py-0.5 text-[11px] text-gray-500 dark:border-dark-600 dark:text-gray-400">{{ channel }}</span>
            </div>
          </button>
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
const groupSearch = ref('')
const selectedGroupId = ref<number | null>(null)
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

type PricingGroupSummary = {
  id: number
  name: string
  pricingUnit?: string
  channelCount: number
  modelCount: number
  channels: string[]
}

const groupSummaries = computed<PricingGroupSummary[]>(() => {
  const groups = new Map<number, { name: string; pricingUnit?: string; channels: Set<string>; models: Set<string> }>()
  for (const channel of channels.value) {
    const current = groups.get(channel.group_id) ?? {
      name: channel.group_name,
      pricingUnit: channel.pricing_unit,
      channels: new Set<string>(),
      models: new Set<string>()
    }
    current.name = channel.group_name || current.name
    current.pricingUnit = channel.pricing_unit || current.pricingUnit
    current.channels.add(channel.name)
    for (const pricing of channel.model_pricing) {
      for (const model of pricing.models) current.models.add(model)
    }
    groups.set(channel.group_id, current)
  }
  return [...groups.entries()]
    .map(([id, group]) => ({
      id,
      name: group.name,
      pricingUnit: group.pricingUnit,
      channelCount: group.channels.size,
      modelCount: group.models.size,
      channels: [...group.channels].sort((a, b) => a.localeCompare(b))
    }))
    .sort((a, b) => a.name.localeCompare(b.name))
})
const filteredGroups = computed(() => {
  const q = groupSearch.value.trim().toLowerCase()
  if (!q) return groupSummaries.value
  return groupSummaries.value.filter(group => `${group.name} ${group.pricingUnit ?? ''} ${group.channels.join(' ')}`.toLowerCase().includes(q))
})
const selectedGroup = computed(() => groupSummaries.value.find(group => group.id === selectedGroupId.value))
const rows = computed(() => channels.value.flatMap((ch, ci) => ch.model_pricing.flatMap((pricing, pi) => pricing.models.map((model, mi) => ({
  key: `${ch.group_id}-${ci}-${pi}-${mi}`,
  groupId: ch.group_id,
  channel: ch.name,
  group: ch.group_name,
  pricingUnit: ch.pricing_unit,
  model,
  pricing,
})))))
const groupRows = computed(() => selectedGroupId.value === null ? [] : rows.value.filter(row => row.groupId === selectedGroupId.value))
const platformOptions = computed(() => [
  { value: '', label: t('modelPricing.allPlatforms') },
  ...[...new Set(groupRows.value.map(row => row.pricing.platform))].sort().map(value => ({ value, label: value }))
])
const filteredRows = computed(() => {
  const q = search.value.trim().toLowerCase()
  return groupRows.value.filter(row => (!platform.value || row.pricing.platform === platform.value) && (!mode.value || row.pricing.billing_mode === mode.value) && (!q || `${row.model} ${row.channel} ${row.group} ${row.pricing.platform}`.toLowerCase().includes(q)))
})
const pagedRows = computed(() => filteredRows.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
watch([groupSearch, search, platform, mode, pageSize], () => { page.value = 1 })

function openGroup(groupId: number) {
  selectedGroupId.value = groupId
  groupSearch.value = ''
  search.value = ''
  platform.value = ''
  mode.value = ''
  page.value = 1
  expanded.value.clear()
}

function backToGroups() {
  selectedGroupId.value = null
  search.value = ''
  platform.value = ''
  mode.value = ''
  page.value = 1
  expanded.value.clear()
}

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
    if (selectedGroupId.value !== null && !groupSummaries.value.some(group => group.id === selectedGroupId.value)) {
      selectedGroupId.value = null
    }
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
