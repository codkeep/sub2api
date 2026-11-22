<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="card p-6">
        <h3 class="font-semibold">{{ t('admin.affiliates.withdrawals.policyTitle') }}</h3>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('admin.affiliates.withdrawals.policyHint') }}</p>
        <form class="mt-4 flex flex-wrap items-end gap-3" @submit.prevent="savePolicy">
          <label class="space-y-1 text-sm">
            <span>{{ t('admin.affiliates.withdrawals.minimumAmount') }}</span>
            <input v-model="minimumAmount" data-test="minimum" type="number" min="0" max="999999999999" step="0.00000001" class="input w-64" :disabled="!policyLoaded || savingPolicy" required />
          </label>
          <button v-if="policyLoaded" data-test="save-policy" class="btn btn-primary" :disabled="savingPolicy">{{ t('common.save') }}</button>
          <button v-else type="button" class="btn btn-secondary" @click="loadPolicy">{{ t('affiliate.withdrawals.retry') }}</button>
        </form>
      </section>
      <section class="card overflow-hidden">
        <form class="flex flex-wrap items-center gap-3 p-5" @submit.prevent="reload">
          <input v-model="search" data-test="search" class="input w-full sm:w-72" :placeholder="t('admin.affiliates.withdrawals.searchPlaceholder')" :aria-label="t('admin.affiliates.records.search')" />
          <select v-model="status" data-test="status-filter" class="input w-full sm:w-40" :aria-label="t('affiliate.withdrawals.status')" @change="reload">
            <option value="">{{ t('admin.affiliates.withdrawals.allStatuses') }}</option>
            <option v-for="value in statuses" :key="value" :value="value">{{ t(`affiliate.withdrawals.statuses.${value}`) }}</option>
          </select>
          <button class="btn btn-secondary" :disabled="loading">{{ t('admin.affiliates.records.search') }}</button>
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadRecords">{{ t('common.refresh') }}</button>
        </form>
        <p v-if="recordsError" role="alert" class="px-5 pb-4 text-sm text-red-600 dark:text-red-400">{{ recordsError }}</p>
        <p v-if="loading" class="py-8 text-center text-sm text-gray-500">{{ t('common.loading') }}</p>
        <p v-else-if="!records.length && !recordsError" class="py-8 text-center text-sm text-gray-500">{{ t('affiliate.withdrawals.empty') }}</p>
        <div v-if="records.length" class="overflow-x-auto">
          <table class="w-full min-w-[1050px] text-left text-sm">
            <thead class="border-y border-gray-200 bg-gray-50 text-gray-500 dark:border-dark-700 dark:bg-dark-800"><tr>
              <th class="p-4">{{ t('admin.affiliates.records.user') }}</th>
              <th class="p-4">{{ t('affiliate.withdrawals.amount') }}</th>
              <th class="p-4">{{ t('affiliate.withdrawals.payoutDetails') }}</th>
              <th class="p-4">{{ t('affiliate.withdrawals.status') }}</th>
              <th class="p-4">{{ t('affiliate.withdrawals.result') }}</th>
              <th class="p-4">{{ t('affiliate.withdrawals.createdAt') }}</th>
              <th class="p-4">{{ t('admin.affiliates.withdrawals.actions') }}</th>
            </tr></thead>
            <tbody><tr v-for="record in records" :key="record.id" :data-test="`withdrawal-${record.id}`" class="border-b border-gray-100 dark:border-dark-800">
              <td class="max-w-60 break-words p-4"><p>{{ record.user_email }}</p><p class="text-gray-500">{{ record.username }} · #{{ record.user_id }}</p></td>
              <td class="p-4 font-medium">{{ formatCurrency(record.amount) }}<p class="text-xs text-gray-500">#{{ record.id }}</p></td>
              <td class="max-w-64 break-words p-4"><p>{{ record.payout_method }} · {{ record.payout_name }}</p><p class="select-all">{{ record.payout_account }}</p><p v-if="record.user_note" class="mt-1 text-gray-500">{{ record.user_note }}</p></td>
              <td class="whitespace-nowrap p-4"><span class="badge" :class="statusClass(record.status)">{{ t(`affiliate.withdrawals.statuses.${record.status}`) }}</span><p v-if="record.processed_by" class="mt-1 text-xs text-gray-500">{{ t('admin.affiliates.withdrawals.handler') }} #{{ record.processed_by }}</p></td>
              <td class="max-w-64 break-words p-4"><p>{{ record.admin_note || '—' }}</p><p v-if="record.payment_reference">{{ t('affiliate.withdrawals.reference') }}: {{ record.payment_reference }}</p><p v-if="record.processed_at" class="mt-1 text-xs text-gray-500">{{ t('affiliate.withdrawals.processedAt') }}: {{ formatDateTime(record.processed_at) }}</p></td>
              <td class="whitespace-nowrap p-4">{{ formatDateTime(record.created_at) }}</td>
              <td class="p-4"><div class="flex flex-col items-start gap-2">
                <button v-if="record.status === 'pending'" data-test="claim" class="btn btn-primary btn-sm" @click="openReview(record, 'processing')">{{ t('admin.affiliates.withdrawals.claim') }}</button>
                <button v-if="record.status === 'processing' && record.processed_by === authStore.user?.id" data-test="pay" class="btn btn-primary btn-sm" @click="openReview(record, 'paid')">{{ t('admin.affiliates.withdrawals.markPaid') }}</button>
                <button v-if="record.status === 'pending' || (record.status === 'processing' && record.processed_by === authStore.user?.id)" data-test="reject" class="btn btn-secondary btn-sm text-red-600" @click="openReview(record, 'rejected')">{{ t('admin.affiliates.withdrawals.reject') }}</button>
              </div></td>
            </tr></tbody>
          </table>
        </div>
        <Pagination v-if="total > 0" :page="page" :page-size="pageSize" :total="total" @update:page="changePage" @update:page-size="changePageSize" />
      </section>
    </div>
    <BaseDialog :show="!!selected" :title="reviewTitle" :close-on-escape="!reviewing && !uncertain" :show-close-button="!reviewing && !uncertain" @close="closeReview">
      <form id="withdrawal-review" class="space-y-4" novalidate @submit.prevent="submitReview">
        <div v-if="selected" class="rounded-xl bg-gray-50 p-4 text-sm dark:bg-dark-800">
          <p class="font-medium">#{{ selected.id }} · {{ selected.user_email }} · {{ formatCurrency(selected.amount) }}</p>
          <p class="mt-2 break-words">{{ selected.payout_method }} · {{ selected.payout_name }} · {{ selected.payout_account }}</p>
        </div>
        <p class="text-sm text-gray-600 dark:text-dark-300">{{ t(`admin.affiliates.withdrawals.hints.${review.status}`) }}</p>
        <fieldset :disabled="reviewing || uncertain" class="space-y-4">
          <label v-if="review.status === 'paid'" class="block space-y-1 text-sm">
            <span>{{ t('affiliate.withdrawals.reference') }}</span>
            <input v-model="review.payment_reference" data-test="reference" class="input" maxlength="200" />
          </label>
          <label v-if="review.status !== 'processing'" class="block space-y-1 text-sm">
            <span>{{ review.status === 'rejected' ? t('admin.affiliates.withdrawals.rejectionReason') : t('admin.affiliates.withdrawals.adminNote') }}</span>
            <textarea v-model="review.admin_note" data-test="admin-note" class="input" maxlength="500" rows="3" />
          </label>
        </fieldset>
        <p v-if="uncertain" role="status" class="text-sm text-amber-600 dark:text-amber-400">{{ t('admin.affiliates.withdrawals.uncertain') }}</p>
        <p v-if="reviewError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ reviewError }}</p>
      </form>
      <template #footer>
        <button class="btn btn-secondary" :disabled="reviewing || uncertain" @click="closeReview">{{ t('common.cancel') }}</button>
        <button data-test="submit-review" form="withdrawal-review" type="submit" class="btn btn-primary" :disabled="reviewing">{{ reviewing ? t('affiliate.withdrawals.submitting') : uncertain ? t('affiliate.withdrawals.retry') : t('common.confirm') }}</button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import { affiliateWithdrawalsAPI as api, type WithdrawalRecord, type WithdrawalReview, type WithdrawalStatus } from '@/api/affiliateWithdrawals'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorCode, extractI18nErrorMessage } from '@/utils/apiError'
import { formatLedgerCurrency as formatCurrency, formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const minimumAmount = ref<number | string>('')
const policyLoaded = ref(false)
const savingPolicy = ref(false)
const search = ref('')
const status = ref<WithdrawalStatus | ''>('')
const statuses: WithdrawalStatus[] = ['pending', 'processing', 'paid', 'rejected']
const records = ref<WithdrawalRecord[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const recordsError = ref('')
let loadSequence = 0
const selected = ref<WithdrawalRecord | null>(null)
const review = reactive<WithdrawalReview>({ status: 'processing', admin_note: '', payment_reference: '' })
const reviewing = ref(false)
const uncertain = ref(false)
const reviewError = ref('')
const reviewTitle = computed(() => t(`admin.affiliates.withdrawals.titles.${review.status}`))
const message = (error: unknown, fallback: string) => extractI18nErrorMessage(error, t, 'affiliate.withdrawals.errors', t(fallback))
const statusClass = (value: WithdrawalStatus) => ({ pending: 'badge-warning', processing: 'badge-primary', paid: 'badge-success', rejected: 'badge-danger' })[value]

async function loadPolicy() {
  try { minimumAmount.value = (await api.getPolicy(true)).minimum_amount; policyLoaded.value = true }
  catch (error) { appStore.showError(message(error, 'affiliate.withdrawals.loadFailed')) }
}
async function savePolicy() {
  if (!policyLoaded.value || savingPolicy.value) return
  const amount = Number(minimumAmount.value)
  if (minimumAmount.value === '' || !Number.isFinite(amount) || amount < 0 || amount >= 1e12) { appStore.showError(t('admin.affiliates.withdrawals.invalidMinimum')); return }
  savingPolicy.value = true
  try {
    minimumAmount.value = (await api.setPolicy({ minimum_amount: amount })).minimum_amount
    appStore.showSuccess(t('admin.affiliates.withdrawals.policySaved'))
  } catch (error) { appStore.showError(message(error, 'admin.affiliates.withdrawals.saveFailed')) }
  finally { savingPolicy.value = false }
}
async function loadRecords() {
  const sequence = ++loadSequence
  loading.value = true
  recordsError.value = ''
  try {
    const result = await api.list({ page: page.value, page_size: pageSize.value, status: status.value, search: search.value.trim() }, true)
    if (sequence !== loadSequence) return
    records.value = result.items; total.value = result.total
  } catch (error) { if (sequence === loadSequence) recordsError.value = message(error, 'affiliate.withdrawals.loadFailed') }
  finally { if (sequence === loadSequence) loading.value = false }
}
function reload() { page.value = 1; void loadRecords() }
function changePage(value: number) { page.value = value; void loadRecords() }
function changePageSize(value: number) { pageSize.value = value; reload() }
function openReview(record: WithdrawalRecord, value: WithdrawalReview['status']) {
  if (reviewing.value || uncertain.value) return
  selected.value = record
  Object.assign(review, { status: value, admin_note: '', payment_reference: '' })
  reviewError.value = ''
}
function closeReview() { if (!reviewing.value && !uncertain.value) selected.value = null }
async function submitReview() {
  if (!selected.value || reviewing.value) return
  reviewError.value = ''
  if (review.status === 'paid' && !review.payment_reference.trim()) { reviewError.value = t('affiliate.withdrawals.errors.AFFILIATE_WITHDRAWAL_REFERENCE_REQUIRED'); return }
  if (review.status === 'rejected' && !review.admin_note.trim()) { reviewError.value = t('affiliate.withdrawals.errors.AFFILIATE_WITHDRAWAL_REJECTION_REQUIRED'); return }
  reviewing.value = true
  try {
    await api.review(selected.value.id, { status: review.status, admin_note: review.admin_note.trim(), payment_reference: review.payment_reference.trim() })
    uncertain.value = false; selected.value = null
    appStore.showSuccess(t('admin.affiliates.withdrawals.reviewSuccess'))
    await loadRecords()
  } catch (error) {
    const statusCode = (error as { status?: number; response?: { status?: number } })?.status ?? (error as { response?: { status?: number } })?.response?.status
    uncertain.value = !statusCode || statusCode >= 500 || statusCode === 408
    reviewError.value = message(error, 'admin.affiliates.withdrawals.reviewFailed')
    if (extractApiErrorCode(error) === 'AFFILIATE_WITHDRAWAL_STATE_CONFLICT') {
      appStore.showError(reviewError.value)
      uncertain.value = false; selected.value = null
      await loadRecords()
    }
  } finally { reviewing.value = false }
}
onMounted(() => { void loadPolicy(); void loadRecords() })
</script>
