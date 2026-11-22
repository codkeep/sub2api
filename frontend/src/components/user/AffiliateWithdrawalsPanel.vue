<template>
  <section class="card space-y-5 p-6">
    <div>
      <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.withdrawals.title') }}</h3>
      <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.withdrawals.description') }}</p>
      <p v-if="policy" class="mt-2 text-sm font-medium text-primary-600 dark:text-primary-400">
        {{ t('affiliate.withdrawals.minimumHint', { amount: formatCurrency(policy.minimum_amount) }) }}
      </p>
      <button v-else type="button" class="btn btn-secondary btn-sm mt-2" @click="loadPolicy">{{ t('affiliate.withdrawals.retry') }}</button>
    </div>
    <form class="space-y-4" novalidate @submit.prevent="apply">
      <fieldset :disabled="submitting || uncertain" class="grid gap-4 sm:grid-cols-2">
        <label class="space-y-1 text-sm">
          <span>{{ t('affiliate.withdrawals.amount') }}</span>
          <input v-model="form.amount" data-test="amount" type="number" step="0.00000001" min="0" class="input" />
        </label>
        <label class="space-y-1 text-sm">
          <span>{{ t('affiliate.withdrawals.payoutMethod') }}</span>
          <input v-model="form.payout_method" data-test="method" class="input" maxlength="40" :placeholder="t('affiliate.withdrawals.methodPlaceholder')" />
        </label>
        <label class="space-y-1 text-sm">
          <span>{{ t('affiliate.withdrawals.payoutAccount') }}</span>
          <input v-model="form.payout_account" data-test="account" class="input" maxlength="200" autocomplete="off" />
        </label>
        <label class="space-y-1 text-sm">
          <span>{{ t('affiliate.withdrawals.payoutName') }}</span>
          <input v-model="form.payout_name" data-test="name" class="input" maxlength="100" autocomplete="off" />
        </label>
        <label class="space-y-1 text-sm sm:col-span-2">
          <span>{{ t('affiliate.withdrawals.userNote') }}</span>
          <textarea v-model="form.user_note" data-test="note" class="input" maxlength="500" rows="2" />
        </label>
      </fieldset>
      <p v-if="uncertain" role="status" class="text-sm text-amber-600 dark:text-amber-400">{{ t('affiliate.withdrawals.uncertain') }}</p>
      <p v-if="errorMessage" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ errorMessage }}</p>
      <button type="submit" data-test="apply" class="btn btn-primary" :disabled="submitting || disabled || (!uncertain && (!policy || availableQuota <= 0 || availableQuota < policy.minimum_amount))">
        {{ submitting ? t('affiliate.withdrawals.submitting') : uncertain ? t('affiliate.withdrawals.retry') : t('affiliate.withdrawals.apply') }}
      </button>
    </form>
    <div class="border-t border-gray-200 pt-5 dark:border-dark-700">
      <div class="flex items-center justify-between gap-3">
        <h4 class="font-medium">{{ t('affiliate.withdrawals.records') }}</h4>
        <button class="btn btn-secondary btn-sm" :disabled="loading" @click="refresh">{{ t('common.refresh') }}</button>
      </div>
      <p v-if="recordsError" role="alert" class="mt-3 text-sm text-red-600 dark:text-red-400">{{ recordsError }}</p>
      <p v-if="loading" class="py-6 text-center text-sm text-gray-500">{{ t('common.loading') }}</p>
      <p v-else-if="!records.length && !recordsError" class="py-6 text-center text-sm text-gray-500">{{ t('affiliate.withdrawals.empty') }}</p>
      <div v-if="records.length" class="mt-3 overflow-x-auto">
        <table class="w-full min-w-[640px] text-left text-sm">
          <thead><tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700">
            <th class="p-3">{{ t('affiliate.withdrawals.amount') }}</th>
            <th class="p-3">{{ t('affiliate.withdrawals.payoutDetails') }}</th>
            <th class="p-3">{{ t('affiliate.withdrawals.status') }}</th>
            <th class="p-3">{{ t('affiliate.withdrawals.result') }}</th>
            <th class="p-3">{{ t('affiliate.withdrawals.createdAt') }}</th>
          </tr></thead>
          <tbody><tr v-for="record in records" :key="record.id" class="border-b border-gray-100 dark:border-dark-800">
            <td class="p-3 font-medium">{{ formatCurrency(record.amount) }}<p class="text-xs text-gray-500">#{{ record.id }}</p></td>
            <td class="max-w-60 break-words p-3">{{ record.payout_method }} · {{ record.payout_name }}<p>{{ record.payout_account }}</p><p v-if="record.user_note" class="text-gray-500">{{ record.user_note }}</p></td>
            <td class="whitespace-nowrap p-3">{{ t(`affiliate.withdrawals.statuses.${record.status}`) }}</td>
            <td class="max-w-60 break-words p-3"><p>{{ record.admin_note || '—' }}</p><p v-if="record.payment_reference">{{ t('affiliate.withdrawals.reference') }}: {{ record.payment_reference }}</p><p v-if="record.processed_at" class="mt-1 text-xs text-gray-500">{{ t('affiliate.withdrawals.processedAt') }}: {{ formatDateTime(record.processed_at) }}</p></td>
            <td class="whitespace-nowrap p-3">{{ formatDateTime(record.created_at) }}</td>
          </tr></tbody>
        </table>
      </div>
      <Pagination v-if="total > pageSize" :page="page" :page-size="pageSize" :total="total" :show-page-size-selector="false" @update:page="changePage" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Pagination from '@/components/common/Pagination.vue'
import { affiliateWithdrawalsAPI as api, type WithdrawalInput, type WithdrawalPolicy, type WithdrawalRecord } from '@/api/affiliateWithdrawals'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { formatLedgerCurrency as formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorCode, extractI18nErrorMessage } from '@/utils/apiError'
import { prepareWithdrawalOperation, completeWithdrawalOperation } from '@/utils/affiliateWithdrawalOperation'

const props = defineProps<{ availableQuota: number; disabled?: boolean }>()
const emit = defineEmits<{ updated: []; busy: [boolean] }>()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const policy = ref<WithdrawalPolicy | null>(null)
const form = reactive({ amount: '', payout_method: '', payout_account: '', payout_name: '', user_note: '' })
const submitting = ref(false)
const uncertain = ref(false)
const errorMessage = ref('')
const recordsError = ref('')
const records = ref<WithdrawalRecord[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = 10
const total = ref(0)
let loadSequence = 0
let pending: { input: WithdrawalInput; operation: Awaited<ReturnType<typeof prepareWithdrawalOperation>>; userID: number } | null = null
const message = (error: unknown, fallback: string) => extractI18nErrorMessage(error, t, 'affiliate.withdrawals.errors', t(fallback))

watch([submitting, uncertain], () => emit('busy', submitting.value || uncertain.value))

async function loadPolicy() {
  try { policy.value = await api.getPolicy() }
  catch (error) { appStore.showError(message(error, 'affiliate.withdrawals.loadFailed')) }
}
async function loadRecords() {
  const sequence = ++loadSequence
  loading.value = true
  recordsError.value = ''
  try {
    const result = await api.list({ page: page.value, page_size: pageSize })
    if (sequence !== loadSequence) return
    records.value = result.items
    total.value = result.total
  } catch (error) {
    if (sequence === loadSequence) recordsError.value = message(error, 'affiliate.withdrawals.loadFailed')
  } finally { if (sequence === loadSequence) loading.value = false }
}
function changePage(value: number) { page.value = value; void loadRecords() }
function refresh() { void loadPolicy(); void loadRecords(); emit('updated') }

async function apply() {
  if (submitting.value || props.disabled) return
  errorMessage.value = ''
  if (!pending) {
    const amount = Math.round(Number(form.amount) * 1e8) / 1e8
    if (!policy.value) return
    if (!Number.isFinite(amount) || amount <= 0 || amount >= 1e12) { errorMessage.value = t('affiliate.withdrawals.invalidAmount'); return }
    if (amount < policy.value.minimum_amount) { errorMessage.value = t('affiliate.withdrawals.minimumHint', { amount: formatCurrency(policy.value.minimum_amount) }); return }
    if (amount > props.availableQuota) { errorMessage.value = t('affiliate.withdrawals.insufficient'); return }
    if (!form.payout_method.trim() || !form.payout_account.trim() || !form.payout_name.trim()) { errorMessage.value = t('affiliate.withdrawals.detailsRequired'); return }
  }
  submitting.value = true
  try {
    if (!pending) {
      const input: WithdrawalInput = {
        amount: Math.round(Number(form.amount) * 1e8) / 1e8,
        payout_method: form.payout_method.trim(), payout_account: form.payout_account.trim(),
        payout_name: form.payout_name.trim(), user_note: form.user_note.trim(),
      }
      const userID = authStore.user?.id ?? 0
      pending = { input, userID, operation: await prepareWithdrawalOperation(userID, input) }
    }
    await api.apply(pending.input, pending.operation.key)
    completeWithdrawalOperation(pending.userID, pending.input, pending.operation)
    pending = null
    uncertain.value = false
    Object.assign(form, { amount: '', payout_method: '', payout_account: '', payout_name: '', user_note: '' })
    appStore.showSuccess(t('affiliate.withdrawals.success'))
    emit('updated')
    page.value = 1
    await loadRecords()
  } catch (error) {
    const status = (error as { status?: number; response?: { status?: number } })?.status ?? (error as { response?: { status?: number } })?.response?.status
    uncertain.value = !!pending && (!status || status >= 500 || status === 408)
    if (!uncertain.value && pending) {
      completeWithdrawalOperation(pending.userID, pending.input, pending.operation)
      pending = null
    }
    errorMessage.value = message(error, 'affiliate.withdrawals.applyFailed')
    if (extractApiErrorCode(error) === 'AFFILIATE_WITHDRAWAL_BELOW_MINIMUM') void loadPolicy()
    if (extractApiErrorCode(error) === 'AFFILIATE_QUOTA_INSUFFICIENT') emit('updated')
  } finally { submitting.value = false }
}
onMounted(() => { void loadPolicy(); void loadRecords() })
</script>
