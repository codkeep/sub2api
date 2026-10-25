<template>
  <div v-if="subscriptionFeatureEnabled && hasActiveSubscriptions" class="relative" ref="containerRef">
    <!-- Mini Progress Display -->
    <button
      @click="toggleTooltip"
      class="group flex cursor-pointer items-center gap-2 rounded-xl border border-slate-200 bg-white/90 px-2.5 py-1.5 shadow-sm shadow-slate-900/5 backdrop-blur transition-all hover:-translate-y-0.5 hover:border-teal-300 hover:shadow-md dark:border-slate-700 dark:bg-slate-900/90 dark:shadow-black/20 dark:hover:border-teal-500/60"
      :title="t('subscriptionProgress.viewDetails')"
    >
      <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-slate-900 text-teal-300 shadow-sm shadow-slate-900/20 dark:bg-teal-400/15 dark:text-teal-300">
        <Icon name="sparkles" size="sm" />
      </span>
      <span class="hidden min-w-0 flex-col items-start leading-tight sm:flex">
        <span class="text-[10px] font-semibold uppercase tracking-[0.14em] text-slate-400 dark:text-slate-500">
          {{ t('subscriptionProgress.title') }}
        </span>
        <span class="text-xs font-semibold text-slate-700 dark:text-slate-200">
          {{ t('subscriptionProgress.activeCount', { count: activeSubscriptions.length }) }}
        </span>
      </span>
      <span class="flex items-center gap-1 rounded-full border border-slate-200 bg-slate-50 px-2 py-1 dark:border-slate-700 dark:bg-slate-800">
        <span
          v-for="(sub, index) in displaySubscriptions.slice(0, 3)"
          :key="index"
          class="h-1.5 w-1.5 rounded-full ring-2 ring-slate-50 dark:ring-slate-800"
          :class="getProgressDotClass(sub)"
        ></span>
      </span>
      <Icon name="chevronDown" size="xs" class="text-slate-400 transition-transform group-hover:text-teal-500" :class="{ 'rotate-180': tooltipOpen }" />
    </button>

    <!-- Hover/Click Tooltip -->
    <transition name="dropdown">
      <div
        v-if="tooltipOpen"
        class="absolute right-0 z-50 mt-3 w-[min(440px,calc(100vw-1.5rem))] overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-[0_24px_70px_-20px_rgba(15,23,42,0.35)] dark:border-slate-700 dark:bg-slate-950 dark:shadow-[0_24px_70px_-20px_rgba(0,0,0,0.7)]"
      >
        <div class="relative overflow-hidden border-b border-slate-200 bg-gradient-to-br from-slate-50 via-white to-teal-50/70 px-5 py-4 dark:border-slate-800 dark:from-slate-900 dark:via-slate-950 dark:to-teal-950/50">
          <div class="pointer-events-none absolute -right-12 -top-16 h-36 w-36 rounded-full bg-teal-300/20 blur-3xl dark:bg-teal-400/10"></div>
          <div class="relative flex items-center gap-3">
            <span class="flex h-10 w-10 items-center justify-center rounded-2xl bg-slate-900 text-teal-300 shadow-lg shadow-slate-900/15 dark:bg-teal-400/15 dark:text-teal-200">
              <Icon name="sparkles" size="md" />
            </span>
            <div class="min-w-0">
              <h3 class="text-base font-semibold tracking-tight text-slate-900 dark:text-white">
                {{ t('subscriptionProgress.title') }}
              </h3>
              <p class="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
                {{ t('subscriptionProgress.activeCount', { count: activeSubscriptions.length }) }}
              </p>
            </div>
          </div>
        </div>

        <div class="max-h-[min(500px,calc(100vh-13rem))] space-y-3 overflow-y-auto bg-slate-50/80 p-3 dark:bg-slate-950/80">
          <div
            v-for="subscription in displaySubscriptions"
            :key="subscription.id"
            class="group relative overflow-hidden rounded-2xl border border-slate-200 bg-white p-4 shadow-sm transition-all hover:-translate-y-0.5 hover:border-teal-200 hover:shadow-lg hover:shadow-slate-900/5 dark:border-slate-800 dark:bg-slate-900 dark:hover:border-teal-500/30 dark:hover:shadow-black/20"
          >
            <div class="mb-4 flex items-start justify-between gap-3">
              <div class="flex min-w-0 items-start gap-3">
                <span class="mt-1.5 h-2.5 w-2.5 flex-shrink-0 rounded-full ring-4 ring-slate-100 dark:ring-slate-800" :class="getSubscriptionAccentClass(subscription)"></span>
                <div class="min-w-0">
                  <span class="block truncate text-sm font-semibold text-slate-900 dark:text-white">
                    {{ subscription.group?.name || `Group #${subscription.group_id}` }}
                  </span>
                  <span class="mt-1 inline-flex items-center rounded-md bg-slate-100 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                    {{ subscription.group?.platform || 'API' }}
                  </span>
                </div>
              </div>
              <span
                v-if="subscription.expires_at"
                class="shrink-0 rounded-lg border px-2.5 py-1 text-[11px] font-semibold"
                :class="getDaysRemainingClass(subscription.expires_at)"
              >
                {{ formatDaysRemaining(subscription.expires_at) }}
              </span>
            </div>

            <!-- Progress bars or Unlimited badge -->
            <div class="rounded-xl border border-slate-200/80 bg-slate-50/80 p-3 dark:border-slate-800 dark:bg-slate-950/50">
              <!-- Unlimited subscription badge -->
              <div
                v-if="isUnlimited(subscription)"
                class="flex items-center gap-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2.5 dark:border-emerald-400/20 dark:bg-emerald-400/10"
              >
                <span class="text-xl font-light text-emerald-600 dark:text-emerald-300">∞</span>
                <span class="text-xs font-semibold text-emerald-700 dark:text-emerald-300">
                  {{ t('subscriptionProgress.unlimited') }}
                </span>
              </div>

              <!-- Progress bars for limited subscriptions -->
              <template v-else>
                <div v-if="subscription.group?.daily_limit_usd" class="space-y-1.5">
                  <div class="flex items-center justify-between gap-3">
                    <span class="text-[10px] font-semibold uppercase tracking-[0.12em] text-slate-400 dark:text-slate-500">{{ t('subscriptionProgress.daily') }}</span>
                    <span class="text-[11px] font-semibold tabular-nums text-slate-600 dark:text-slate-300">
                      {{ formatUsage(subscription.daily_usage_usd, subscription.group?.daily_limit_usd, subscription.group) }}
                    </span>
                  </div>
                  <div class="h-1.5 min-w-0 overflow-hidden rounded-full bg-slate-200 dark:bg-slate-800">
                    <div
                      class="h-full rounded-full transition-all"
                      :class="
                        getProgressBarClass(
                          subscription.daily_usage_usd,
                          subscription.group?.daily_limit_usd
                        )
                      "
                      :style="{
                        width: getProgressWidth(
                          subscription.daily_usage_usd,
                          subscription.group?.daily_limit_usd
                        )
                      }"
                    ></div>
                  </div>
                </div>

                <div v-if="subscription.group?.weekly_limit_usd" class="mt-3 space-y-1.5">
                  <div class="flex items-center justify-between gap-3">
                    <span class="text-[10px] font-semibold uppercase tracking-[0.12em] text-slate-400 dark:text-slate-500">{{ t('subscriptionProgress.weekly') }}</span>
                    <span class="text-[11px] font-semibold tabular-nums text-slate-600 dark:text-slate-300">
                      {{ formatUsage(subscription.weekly_usage_usd, subscription.group?.weekly_limit_usd, subscription.group) }}
                    </span>
                  </div>
                  <div class="h-1.5 min-w-0 overflow-hidden rounded-full bg-slate-200 dark:bg-slate-800">
                    <div
                      class="h-full rounded-full transition-all"
                      :class="
                        getProgressBarClass(
                          subscription.weekly_usage_usd,
                          subscription.group?.weekly_limit_usd
                        )
                      "
                      :style="{
                        width: getProgressWidth(
                          subscription.weekly_usage_usd,
                          subscription.group?.weekly_limit_usd
                        )
                      }"
                    ></div>
                  </div>
                </div>

                <div v-if="subscription.group?.monthly_limit_usd" class="mt-3 space-y-1.5">
                  <div class="flex items-center justify-between gap-3">
                    <span class="text-[10px] font-semibold uppercase tracking-[0.12em] text-slate-400 dark:text-slate-500">{{ t('subscriptionProgress.monthly') }}</span>
                    <span class="text-[11px] font-semibold tabular-nums text-slate-600 dark:text-slate-300">
                      {{ formatUsage(subscription.monthly_usage_usd, subscription.group?.monthly_limit_usd, subscription.group) }}
                    </span>
                  </div>
                  <div class="h-1.5 min-w-0 overflow-hidden rounded-full bg-slate-200 dark:bg-slate-800">
                    <div
                      class="h-full rounded-full transition-all"
                      :class="
                        getProgressBarClass(
                          subscription.monthly_usage_usd,
                          subscription.group?.monthly_limit_usd
                        )
                      "
                      :style="{
                        width: getProgressWidth(
                          subscription.monthly_usage_usd,
                          subscription.group?.monthly_limit_usd
                        )
                      }"
                    ></div>
                  </div>
                </div>
              </template>
            </div>
          </div>
        </div>

        <div class="border-t border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-950">
          <router-link
            to="/subscriptions"
            @click="closeTooltip"
            class="block w-full rounded-xl bg-slate-900 py-2.5 text-center text-xs font-semibold text-white shadow-sm shadow-slate-900/15 transition-all hover:bg-teal-700 dark:bg-teal-400/15 dark:text-teal-200 dark:hover:bg-teal-400/25"
          >
            {{ t('subscriptionProgress.viewAll') }}
          </router-link>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { useQuotaUsageDisplay, type QuotaGroup } from '@/composables/useSubscriptionUnit'
const formatQuotaUsage = useQuotaUsageDisplay()
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useSubscriptionStore } from '@/stores'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import type { UserSubscription } from '@/types'
import { getExpirationDateRelation } from '@/utils/subscriptionQuota'

const { t } = useI18n()

const subscriptionStore = useSubscriptionStore()

const containerRef = ref<HTMLElement | null>(null)
const tooltipOpen = ref(false)

// Use store data instead of local state
const activeSubscriptions = computed(() => subscriptionStore.activeSubscriptions)
const hasActiveSubscriptions = computed(() => subscriptionStore.hasActiveSubscriptions)
// 订阅功能关闭后，即使用户仍持有后台分配的订阅，顶栏也不再露出订阅进度与「查看全部订阅」入口。
const subscriptionFeatureEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.subscription))

const displaySubscriptions = computed(() => {
  // Sort by most usage (highest percentage first)
  return [...activeSubscriptions.value].sort((a, b) => {
    const aMax = getMaxUsagePercentage(a)
    const bMax = getMaxUsagePercentage(b)
    return bMax - aMax
  })
})

function getMaxUsagePercentage(sub: UserSubscription): number {
  const percentages: number[] = []
  if (sub.group?.daily_limit_usd) {
    percentages.push(((sub.daily_usage_usd || 0) / sub.group.daily_limit_usd) * 100)
  }
  if (sub.group?.weekly_limit_usd) {
    percentages.push(((sub.weekly_usage_usd || 0) / sub.group.weekly_limit_usd) * 100)
  }
  if (sub.group?.monthly_limit_usd) {
    percentages.push(((sub.monthly_usage_usd || 0) / sub.group.monthly_limit_usd) * 100)
  }
  return percentages.length > 0 ? Math.max(...percentages) : 0
}

function isUnlimited(sub: UserSubscription): boolean {
  return (
    !sub.group?.daily_limit_usd &&
    !sub.group?.weekly_limit_usd &&
    !sub.group?.monthly_limit_usd
  )
}

function getProgressDotClass(sub: UserSubscription): string {
  // Unlimited subscriptions get a special color
  if (isUnlimited(sub)) {
    return 'bg-emerald-500'
  }
  const maxPercentage = getMaxUsagePercentage(sub)
  if (maxPercentage >= 90) return 'bg-rose-500'
  if (maxPercentage >= 70) return 'bg-cyan-500'
  return 'bg-emerald-500'
}

function getProgressBarClass(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return 'bg-gray-400'
  const percentage = ((used || 0) / limit) * 100
  if (percentage >= 90) return 'bg-rose-500'
  if (percentage >= 70) return 'bg-cyan-500'
  return 'bg-emerald-500'
}

function getProgressWidth(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return '0%'
  const percentage = Math.min(((used || 0) / limit) * 100, 100)
  return `${percentage}%`
}

function getSubscriptionAccentClass(sub: UserSubscription): string {
  if (isUnlimited(sub)) return 'bg-emerald-500'
  const percentage = getMaxUsagePercentage(sub)
  if (percentage >= 90) return 'bg-rose-500'
  if (percentage >= 70) return 'bg-cyan-500'
  return 'bg-emerald-500'
}

function formatUsage(used: number | undefined, limit: number | null | undefined, group?: QuotaGroup): string {
  return formatQuotaUsage(used, limit, group)
}

function formatDaysRemaining(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const relation = getExpirationDateRelation(expires, now)
  if (relation === 'expired') return t('subscriptionProgress.expired')
  if (relation === 'today') return t('subscriptionProgress.expiresToday')
  if (relation === 'tomorrow') return t('subscriptionProgress.expiresTomorrow')
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  return t('subscriptionProgress.daysRemaining', { days })
}

function getDaysRemainingClass(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  if (days <= 3) return 'border-rose-200 bg-rose-50 text-rose-600 dark:border-rose-400/30 dark:bg-rose-400/10 dark:text-rose-300'
  if (days <= 7) return 'border-cyan-200 bg-cyan-50 text-cyan-700 dark:border-cyan-400/30 dark:bg-cyan-400/10 dark:text-cyan-300'
  return 'border-slate-200 bg-slate-50 text-slate-500 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-400'
}

function toggleTooltip() {
  tooltipOpen.value = !tooltipOpen.value
}

function closeTooltip() {
  tooltipOpen.value = false
}

function handleClickOutside(event: MouseEvent) {
  if (containerRef.value && !containerRef.value.contains(event.target as Node)) {
    closeTooltip()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  // Trigger initial fetch if not already loaded
  // The actual data loading is handled by App.vue globally
  if (!subscriptionFeatureEnabled.value) return
  subscriptionStore.fetchActiveSubscriptions().catch((error) => {
    console.error('Failed to load subscriptions in SubscriptionProgressMini:', error)
  })
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scale(0.95) translateY(-4px);
}
</style>
