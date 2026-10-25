<template>
  <div class="subscription-plan" :class="{ 'subscription-plan--active': isRenewal }">
    <span v-if="isRenewal" class="subscription-plan__ribbon">{{ t('payment.design.currentPlan') }}</span>

    <div class="relative flex flex-1 flex-col p-5 sm:p-7">
      <!-- Header: name + badge + price -->
      <div class="mb-6 flex flex-wrap items-start justify-between gap-5">
        <div class="min-w-0 flex-1 basis-40">
          <div class="mb-2 flex items-center gap-2">
            <span class="h-2 w-2 rounded-full bg-teal-400 shadow-[0_0_10px_#2dd4bf80]" />
            <span class="text-xs font-semibold uppercase tracking-[0.18em] text-gray-500 dark:text-slate-400">{{ pLabel }}</span>
          </div>
          <h3
            :title="plan.name"
            class="min-w-0 break-words text-2xl font-semibold leading-8 tracking-tight text-gray-900 dark:text-white [overflow-wrap:anywhere]"
          >
            {{ plan.name }}
          </h3>
          <p v-if="plan.description" class="mt-4 break-words text-sm leading-relaxed text-gray-500 dark:text-slate-400">
            {{ plan.description }}
          </p>
        </div>
        <div class="min-w-0 text-right">
          <div class="flex flex-wrap items-baseline justify-end gap-1.5">
            <span class="text-sm text-gray-400 dark:text-dark-500">{{ planCurrencySymbol }}</span>
            <span class="subscription-plan__price text-4xl font-semibold tabular-nums tracking-tight">{{ plan.price }}</span>
            <span v-if="plan.currency" class="text-xs font-medium text-gray-400 dark:text-dark-500">{{ plan.currency }}</span>
          </div>
          <div class="mt-2 flex flex-wrap items-center justify-end gap-2">
            <span :class="['inline-flex shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium', badgeLightClass]">
              {{ pLabel }}
            </span>
            <span class="text-[11px] text-gray-400 dark:text-dark-500">/ {{ validitySuffix }}</span>
          </div>
          <div v-if="plan.original_price" class="mt-2 flex flex-wrap items-center justify-end gap-1.5">
            <span class="text-xs text-gray-400 line-through dark:text-dark-500">{{ planCurrencySymbol }}{{ plan.original_price }}<template v-if="plan.currency"> {{ plan.currency }}</template></span>
            <span v-if="discountText" class="rounded-full bg-teal-500/10 px-2 py-0.5 text-xs font-semibold text-teal-700 dark:text-teal-300">{{ discountText }}</span>
          </div>
        </div>
      </div>

      <!-- Group quota info (compact) -->
      <div class="subscription-plan__quota mb-5 grid grid-cols-1 gap-x-6 gap-y-3 p-4 text-sm sm:grid-cols-2">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.rate') }}</span>
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ rateDisplay }}</span>
        </div>
        <div v-if="hasPeakRate" class="flex flex-wrap items-center justify-between gap-2 sm:col-span-2">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.peakRate') }}</span>
          <span class="text-right font-medium text-amber-700 dark:text-amber-300">{{ peakRateDisplay }}</span>
        </div>
        <div v-if="plan.daily_limit_usd != null" class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.dailyLimit') }}</span>
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ formatQuota(plan.daily_limit_usd, plan) }}</span>
        </div>
        <div v-if="plan.weekly_limit_usd != null" class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.weeklyLimit') }}</span>
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ formatQuota(plan.weekly_limit_usd, plan) }}</span>
        </div>
        <div v-if="plan.monthly_limit_usd != null" class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.monthlyLimit') }}</span>
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ formatQuota(plan.monthly_limit_usd, plan) }}</span>
        </div>
        <div v-if="plan.daily_limit_usd == null && plan.weekly_limit_usd == null && plan.monthly_limit_usd == null" class="flex flex-wrap items-center justify-between gap-2">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.quota') }}</span>
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ t('payment.planCard.unlimited') }}</span>
        </div>
        <div v-if="modelScopeLabels.length > 0" class="flex flex-wrap items-center justify-between gap-2 sm:col-span-2">
          <span class="text-gray-400 dark:text-dark-500">{{ t('payment.planCard.models') }}</span>
          <div class="flex flex-wrap justify-end gap-1">
            <span v-for="scope in modelScopeLabels" :key="scope"
              class="rounded bg-gray-200/80 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-dark-600 dark:text-gray-300">
              {{ scope }}
            </span>
          </div>
        </div>
      </div>

      <!-- Features list (compact) -->
      <div v-if="plan.features.length > 0" class="mb-3 space-y-1">
        <div v-for="feature in plan.features" :key="feature" class="flex items-start gap-1.5">
          <svg :class="['mt-0.5 h-3.5 w-3.5 flex-shrink-0', iconClass]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
          </svg>
          <span class="text-xs text-gray-600 dark:text-gray-300">{{ feature }}</span>
        </div>
      </div>

      <div class="flex-1" />

      <!-- Subscribe Button -->
      <button
        type="button"
        class="subscription-plan__button w-full rounded-xl py-3.5 text-sm font-semibold transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500"
        @click="emit('select', plan)"
      >
        {{ isRenewal ? t('payment.renewNow') : t('payment.subscribeNow') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useQuotaDisplay } from '@/composables/useSubscriptionUnit'
const formatQuota = useQuotaDisplay()
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionPlan } from '@/types/payment'
import type { UserSubscription } from '@/types'
import { useAppStore } from '@/stores/app'
import { hasPeakRate as groupHasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { planValiditySuffix } from './validity'
import { currencySymbol } from '@/components/payment/currency'
import {
  platformBadgeLightClass,
  platformIconClass,
  platformLabel,
} from '@/utils/platformColors'

const props = defineProps<{ plan: SubscriptionPlan; activeSubscriptions?: UserSubscription[] }>()
const emit = defineEmits<{ select: [plan: SubscriptionPlan] }>()
const { t } = useI18n()

const platform = computed(() => props.plan.group_platform || '')
const isRenewal = computed(() =>
  props.activeSubscriptions?.some(s => s.group_id === props.plan.group_id && s.status === 'active') ?? false
)

// Derived color classes from central config
const badgeLightClass = computed(() => platformBadgeLightClass(platform.value))
const iconClass = computed(() => platformIconClass(platform.value))
const pLabel = computed(() => platformLabel(platform.value))

const discountText = computed(() => {
  if (!props.plan.original_price || props.plan.original_price <= 0) return ''
  const pct = Math.round((1 - props.plan.price / props.plan.original_price) * 100)
  return pct > 0 ? `-${pct}%` : ''
})

const rateDisplay = computed(() => {
  const rate = props.plan.rate_multiplier ?? 1
  return `×${Number(rate.toPrecision(10))}`
})

const appStore = useAppStore()
const planCurrencySymbol = computed(() => currencySymbol(props.plan.currency || 'USD'))

const hasPeakRate = computed(() => groupHasPeakRate(props.plan))

const peakRateDisplay = computed(() => {
  return formatPeakRateWindow(props.plan, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
})

const MODEL_SCOPE_LABELS: Record<string, string> = {
  claude: 'Claude',
  gemini_text: 'Gemini',
  gemini_image: 'Imagen',
}

const modelScopeLabels = computed(() => {
  if (platform.value !== 'antigravity') return []
  const scopes = props.plan.supported_model_scopes
  if (!scopes || scopes.length === 0) return []
  return scopes.map(s => MODEL_SCOPE_LABELS[s] || s)
})

const validitySuffix = computed(() => planValiditySuffix(props.plan, t))
</script>

<style>
.subscription-plan {
  position: relative;
  isolation: isolate;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid #d5e4e1;
  border-top: 3px solid #14cda9;
  border-radius: 18px;
  background: linear-gradient(130deg, #fff, #f4faf8);
  box-shadow: 0 8px 28px #062d2308, inset 0 1px 0 #fff;
  transition: border-color .2s, box-shadow .2s;
}
.subscription-plan::before {
  content: '';
  position: absolute;
  z-index: -1;
  width: 55%;
  height: 180px;
  top: 0;
  right: 0;
  background: linear-gradient(135deg, #10cba609, transparent);
  clip-path: polygon(30% 0, 100% 0, 55% 100%, 0 100%);
}
.subscription-plan:hover { border-color: #14cda980; box-shadow: 0 8px 28px #0ab58e14; }
.subscription-plan--active { border-color: #10cda670; padding-top: 14px; }
.subscription-plan__ribbon { position: absolute; top: 0; right: 0; z-index: 1; padding: 5px 18px; border-radius: 0 0 0 12px; background: #24e6c0; color: #043e34; font-size: 11px; font-weight: 700; }
.subscription-plan__price { color: #0b997f; overflow-wrap: anywhere; }
.subscription-plan__quota { border: 1px solid #dce6e5; border-radius: 13px; background: linear-gradient(115deg, #f0f5f6, #f8fbfb); }
.subscription-plan__button { border: 1px solid #16cda4; background: linear-gradient(110deg, #10bf99, #09a77e); color: #fff; box-shadow: inset 0 1px 0 #ffffff25; }
.subscription-plan__button:hover { background: linear-gradient(110deg, #11cba4, #0cb98b); }
.dark .subscription-plan { background: radial-gradient(ellipse at 35% 0, #1e4c4933, transparent 60%), linear-gradient(120deg, #19232f, #0d1821 65%, #0a2023); border-color: #4f7b8038; border-top-color: #24e6c0; box-shadow: 0 8px 28px #00000015, inset 0 1px 0 #ffffff0a; }
.dark .subscription-plan--active { border-color: #20d4b25c; border-top-color: #24e6c0; }
.dark .subscription-plan__price { color: #35f0cf; }
.dark .subscription-plan__quota { border-color: #8ca9b529; background: linear-gradient(115deg, #293543b3, #15212bb3); }
</style>
