<template>
  <fieldset class="space-y-3 border-t pt-4">
    <legend class="sr-only">{{ t('admin.groups.modelSkills.title') }}</legend>
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0 flex-1">
        <p class="input-label">{{ t('admin.groups.modelSkills.title') }}</p>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.groups.modelSkills.hint') }}</p>
      </div>
      <button v-if="!editorOpen" type="button" class="btn btn-secondary shrink-0 whitespace-nowrap" @click="addRule">
        <Icon name="plus" size="sm" class="mr-1" />
        {{ t('admin.groups.modelSkills.addModelSkill') }}
      </button>
    </div>

    <div v-if="editorOpen" class="space-y-2">
      <div
        v-for="(rule, index) in form.rules"
        :key="index"
        class="rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-600 dark:bg-dark-800"
      >
        <div class="grid grid-cols-1 items-start gap-3 lg:grid-cols-[minmax(0,1.8fr)_minmax(260px,1fr)_auto]">
          <div class="min-w-0">
            <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.groups.modelSkills.modelName') }}
            </label>
            <ModelTagInput
              :models="rule.models"
              :placeholder="t('admin.groups.modelSkills.modelPlaceholder')"
              class="mt-1"
              @update:models="updateModels(index, $event)"
            />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.groups.modelSkills.modelExactMatch') }}
            </p>
          </div>

          <div class="min-w-0">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.groups.modelSkills.skillsForModel') }}
            </span>
            <div v-if="rule.skillIds.length" class="mb-2 mt-1 flex min-h-8 flex-wrap items-center gap-1.5">
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
            <div v-else class="mb-2 mt-1 min-h-8 text-xs leading-8 text-gray-400 dark:text-gray-500">
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
            type="button"
            class="mt-6 inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-md border border-red-200 text-red-500 transition hover:bg-red-50 hover:text-red-600 dark:border-red-900/60 dark:hover:bg-red-950/30"
            :aria-label="t('admin.groups.modelSkills.removeModel')"
            :title="t('admin.groups.modelSkills.removeModel')"
            @click="removeRule(index)"
          >
            <Icon name="trash" size="sm" />
          </button>
        </div>
      </div>

      <button type="button" class="btn btn-secondary btn-sm" @click="addRule">
        <Icon name="plus" size="sm" class="mr-1" />
        {{ t('admin.groups.modelSkills.addModelSkill') }}
      </button>
    </div>

    <p v-if="loading || !skills.length" class="text-xs text-gray-500 dark:text-gray-400">
      {{ t(loading ? 'admin.groups.modelSkills.loading' : 'admin.groups.modelSkills.empty') }}
    </p>
  </fieldset>
</template>

<script lang="ts">
export interface GroupSkillsForm {
  rules: { models: string[]; skillIds: string[] }[]
}
</script>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ModelSkill } from '@/api/admin/settings'
import Icon from '@/components/icons/Icon.vue'
import ModelTagInput from '@/components/admin/channel/ModelTagInput.vue'

const props = defineProps<{ skills: ModelSkill[]; loading: boolean }>()
const form = defineModel<GroupSkillsForm>({ required: true })
const { t } = useI18n()
const editorOpen = ref(form.value.rules.length > 0)

watch(
  () => form.value.rules.length,
  (length) => {
    editorOpen.value = length > 0
  },
)

function skillName(id: string) {
  return props.skills.find((skill) => skill.id === id)?.name ?? id
}

function addRule() {
  editorOpen.value = true
  form.value = {
    rules: [...form.value.rules, { models: [], skillIds: [] }],
  }
}

function removeRule(index: number) {
  form.value = {
    rules: form.value.rules.filter((_, i) => i !== index),
  }
}

function updateRule(index: number, changes: Partial<GroupSkillsForm['rules'][number]>) {
  const rules = form.value.rules.map((rule, i) => (i === index ? { ...rule, ...changes } : rule))
  form.value = { rules }
}

function updateModels(index: number, models: string[]) {
  updateRule(index, { models })
}

function setSkills(index: number, skillIds: string[]) {
  updateRule(index, { skillIds })
}

function addSkill(index: number, skillIds: string[], event: Event) {
  const select = event.target as HTMLSelectElement
  if (select.value && !skillIds.includes(select.value)) {
    setSkills(index, [...skillIds, select.value])
  }
  select.value = ''
}
</script>
