<template>
  <fieldset class="space-y-3 border-t pt-4">
    <legend class="sr-only">{{ t('admin.groups.modelSkills.title') }}</legend>
    <div>
      <p class="input-label">{{ t('admin.groups.modelSkills.title') }}</p>
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.groups.modelSkills.hint') }}</p>
    </div>
    <label class="block">
      <span class="input-label">{{ t('admin.groups.modelSkills.scope') }}</span>
      <select :value="form.scope" class="input" @change="changeScope">
        <option value="all">{{ t('admin.groups.modelSkills.allModels') }}</option>
        <option value="specified">{{ t('admin.groups.modelSkills.specifiedModels') }}</option>
      </select>
    </label>
    <p v-if="isSpecified" class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.groups.modelSkills.specifiedHint') }}</p>
    <div
      v-for="(rule, index) in visibleRules"
      :key="isSpecified ? index : 'all'"
      :class="isSpecified
        ? 'rounded-lg border border-gray-200 bg-gray-50/70 p-3 dark:border-gray-700 dark:bg-gray-800/40'
        : 'space-y-2'"
    >
      <div
        :class="isSpecified
          ? 'grid grid-cols-1 items-start gap-3 lg:grid-cols-[minmax(0,1.8fr)_minmax(260px,1fr)_auto]'
          : 'space-y-2'"
      >
        <label v-if="isSpecified" class="block min-w-0">
          <span class="input-label">{{ t('admin.groups.modelSkills.modelName') }}</span>
          <input
            :value="rule.model"
            type="text"
            class="input w-full"
            :placeholder="t('admin.groups.modelSkills.modelPlaceholder')"
            @input="updateName(index, $event)"
          />
          <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.groups.modelSkills.modelExactMatch') }}
          </span>
        </label>

        <div class="min-w-0">
          <span v-if="isSpecified" class="input-label">{{ t('admin.groups.modelSkills.skillsForModel') }}</span>
          <div v-if="rule.skillIds.length" class="mb-2 flex min-h-8 flex-wrap items-center gap-1.5">
            <span
              v-for="skillId in rule.skillIds"
              :key="skillId"
              class="inline-flex max-w-full items-center gap-1 rounded-full bg-primary-100 px-2.5 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300"
            >
              <span class="truncate">{{ skillName(skillId) }}</span>
              <button
                type="button"
                class="shrink-0 text-primary-500 hover:text-primary-700 dark:hover:text-primary-200"
                :aria-label="t('admin.groups.modelSkills.removeSkill', { name: skillName(skillId) })"
                @click="setSkills(index, rule.skillIds.filter((id) => id !== skillId))"
              >
                <Icon name="x" size="xs" />
              </button>
            </span>
          </div>
          <div v-else-if="isSpecified" class="mb-2 min-h-8 text-xs leading-8 text-gray-400 dark:text-gray-500">
            {{ t('admin.groups.modelSkills.noSkillsSelected') }}
          </div>
          <select
            class="input w-full"
            :aria-label="t('admin.groups.modelSkills.selectPlaceholder')"
            :disabled="loading || !skills.length"
            @change="addSkill(index, rule.skillIds, $event)"
          >
            <option value="">{{ t('admin.groups.modelSkills.selectPlaceholder') }}</option>
            <option v-for="skill in skills" :key="skill.id" :value="skill.id" :disabled="rule.skillIds.includes(skill.id)">
              {{ skill.name }}{{ skill.filename ? ` · ${skill.filename}` : '' }}
            </option>
          </select>
        </div>

        <button
          v-if="isSpecified"
          type="button"
          class="mt-6 inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-md border border-red-200 text-red-500 transition hover:bg-red-50 hover:text-red-600 dark:border-red-900/60 dark:hover:bg-red-950/30"
          :aria-label="t('admin.groups.modelSkills.removeModel')"
          :title="t('admin.groups.modelSkills.removeModel')"
          @click="removeModel(index)"
        >
          <Icon name="trash" size="sm" />
        </button>
      </div>
    </div>
    <p v-if="loading || !skills.length" class="text-xs text-gray-500 dark:text-gray-400">
      {{ t(loading ? 'admin.groups.modelSkills.loading' : 'admin.groups.modelSkills.empty') }}
    </p>
    <button v-if="isSpecified" type="button" class="btn btn-secondary btn-sm" @click="addModel">
      <Icon name="plus" size="sm" class="mr-1" />
      {{ t('admin.groups.modelSkills.addModel') }}
    </button>
  </fieldset>
</template>

<script lang="ts">
export interface GroupSkillsForm {
  scope: 'all' | 'specified'
  defaultSkillIds: string[]
  rules: { model: string; skillIds: string[] }[]
}
</script>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ModelSkill } from '@/api/admin/settings'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ skills: ModelSkill[]; loading: boolean }>()
const form = defineModel<GroupSkillsForm>({ required: true })
const { t } = useI18n()
const isSpecified = computed(() => form.value.scope === 'specified')
const visibleRules = computed(() => isSpecified.value ? form.value.rules : [{ model: '', skillIds: form.value.defaultSkillIds }])

function skillName(id: string) {
  return props.skills.find((skill) => skill.id === id)?.name ?? id
}

function changeScope(event: Event) {
  const scope = (event.target as HTMLSelectElement).value === 'specified' ? 'specified' : 'all'
  const rules = scope === 'specified' && form.value.rules.length === 0 ? [{ model: '', skillIds: [] }] : form.value.rules
  form.value = { ...form.value, scope, rules }
}

function addModel() {
  form.value = { ...form.value, rules: [...form.value.rules, { model: '', skillIds: [] }] }
}

function removeModel(index: number) {
  form.value = { ...form.value, rules: form.value.rules.filter((_, i) => i !== index) }
}

function updateRule(index: number, changes: Partial<GroupSkillsForm['rules'][number]>) {
  const rules = form.value.rules.map((rule, i) => i === index ? { ...rule, ...changes } : rule)
  form.value = { ...form.value, rules }
}

function updateName(index: number, event: Event) {
  updateRule(index, { model: (event.target as HTMLInputElement).value })
}

function setSkills(index: number, skillIds: string[]) {
  if (isSpecified.value) {
    updateRule(index, { skillIds })
  } else {
    form.value = { ...form.value, defaultSkillIds: skillIds }
  }
}

function addSkill(index: number, skillIds: string[], event: Event) {
  const select = event.target as HTMLSelectElement
  if (select.value && !skillIds.includes(select.value)) {
    setSkills(index, [...skillIds, select.value])
  }
  select.value = ''
}
</script>
