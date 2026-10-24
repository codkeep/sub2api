<template>
  <div v-if="proxy?.id" class="space-y-1 text-xs" data-testid="usage-proxy">
    <div class="font-medium text-gray-900 dark:text-white">{{ proxy.name || `#${proxy.id}` }}</div>
    <div class="text-gray-500 dark:text-gray-400" :title="observationHint">
      {{ t('admin.usage.proxyExitIP') }}:
      <button v-if="proxy.exit_ip" type="button"
        class="font-mono text-primary-600 underline decoration-dotted underline-offset-2 hover:text-primary-500 disabled:opacity-50 dark:text-primary-400"
        :title="t('admin.usage.proxyIPLookup')" :disabled="loading" @click.stop="lookupLocation">
        {{ proxy.exit_ip }}
      </button>
      <span v-else>{{ t('admin.usage.proxyIPUnavailable') }}</span>
    </div>
    <div v-if="loading" role="status" class="text-gray-500">{{ t('admin.usage.proxyIPLoading') }}</div>
    <div v-else-if="location" class="max-w-64 whitespace-normal text-gray-500 dark:text-gray-400" role="status">
      <div>{{ [location.country, location.region, location.city].filter((part, index, all) => part && all.indexOf(part) === index).join(' · ') || t('admin.usage.proxyIPLocationUnknown') }}</div>
      <div v-if="location.isp">{{ location.isp }}</div>
    </div>
    <button v-else-if="failed" type="button" class="text-red-500" @click.stop="lookupLocation">{{ t('admin.usage.proxyIPLookupFailed') }}</button>
  </div>
  <span v-else class="text-xs text-gray-500 dark:text-gray-400">
    {{ t(proxy ? 'admin.usage.proxyDirect' : 'admin.usage.proxyNotRecorded') }}
  </span>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminUsageLog } from '@/types'
import { getUsageIPLocation, type UsageIPLocation } from '@/api/admin/usage'

const props = defineProps<{ proxy: AdminUsageLog['proxy_snapshot'] }>()
const { t } = useI18n()
const loading = ref(false)
const failed = ref(false)
const location = ref<UsageIPLocation | null>(null)
let lookupVersion = 0
watch(() => props.proxy?.exit_ip, () => {
  lookupVersion++
  location.value = null
  failed.value = false
  loading.value = false
})
async function lookupLocation() {
  const ip = props.proxy?.exit_ip
  if (!ip || loading.value) return
  const version = ++lookupVersion
  loading.value = true
  failed.value = false
  try {
    const result = await getUsageIPLocation(ip)
    if (version === lookupVersion) location.value = result
  } catch {
    if (version === lookupVersion) failed.value = true
  } finally {
    if (version === lookupVersion) loading.value = false
  }
}
const observationHint = computed(() => t('admin.usage.proxyIPHint', {
  time: props.proxy?.exit_ip_checked_at ? new Date(props.proxy.exit_ip_checked_at).toLocaleString() : '-',
}))
</script>
