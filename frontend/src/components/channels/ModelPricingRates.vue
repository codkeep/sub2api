<template>
  <dl v-if="rates.length" class="grid gap-x-6 gap-y-2 text-xs sm:grid-cols-2">
    <div v-for="rate in rates" :key="rate.label" class="flex items-baseline justify-between gap-4">
      <dt class="text-gray-500 dark:text-gray-400">{{ rate.label }}</dt>
      <dd class="whitespace-nowrap font-mono font-medium text-gray-900 dark:text-gray-100">
        {{ formatScaled(rate.value, scale) }}
        <span class="font-sans font-normal text-gray-400">{{ unit }}</span>
      </dd>
    </div>
  </dl>
  <span v-else class="text-xs text-gray-400">{{ t('availableChannels.noPricing') }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserSupportedModelPricing } from '@/api/channels'
import { formatScaled } from '@/utils/pricing'

const props = defineProps<{ pricing: UserSupportedModelPricing }>()
const { t } = useI18n()
const scale = computed(() => props.pricing.billing_mode === 'token' ? 1_000_000 : 1)
const unit = computed(() => t(`modelPricing.units.${props.pricing.billing_mode}`))
const rates = computed(() => {
  const p = props.pricing
  if (p.billing_mode !== 'token') {
    return p.per_request_price == null ? [] : [{ label: t('modelPricing.defaultPrice'), value: p.per_request_price }]
  }
  const fields = [
    ['inputPrice', p.input_price],
    ['outputPrice', p.output_price],
    ['cacheWrite5mPrice', p.cache_write_price],
    ['cacheWrite1hPrice', p.cache_write_1h_price],
    ['cacheReadPrice', p.cache_read_price],
    ['imageInputPrice', p.image_input_price],
    ['imageOutputPrice', p.image_output_price]
  ] as const
  return fields.flatMap(([key, value]) => value == null ? [] : [{ label: t(`availableChannels.pricing.${key}`), value }])
})
</script>
