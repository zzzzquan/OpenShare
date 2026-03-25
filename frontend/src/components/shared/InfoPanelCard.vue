<script setup lang="ts">
import { computed } from "vue";

export interface InfoPanelCardItem {
  id: string;
  label: string;
  badge?: string;
}

const props = withDefaults(
  defineProps<{
    title: string;
    items?: Array<string | InfoPanelCardItem>;
    emptyText?: string;
    clickable?: boolean;
    actionLabel?: string;
  }>(),
  {
    items: () => [],
    emptyText: "暂无内容",
    clickable: false,
    actionLabel: "",
  },
);

const hasItems = computed(() => props.items.length > 0);

const normalizedItems = computed<InfoPanelCardItem[]>(() => {
  return props.items.map((item, index) => (
    typeof item === "string"
      ? { id: `${props.title}-${index}-${item}`, label: item }
      : item
  ));
});

const emit = defineEmits<{
  select: [item: InfoPanelCardItem];
  action: [];
}>();
</script>

<template>
  <section class="panel p-4">
    <header class="flex flex-wrap items-center justify-between gap-2 sm:gap-3">
      <h2 class="text-sm font-medium tracking-tight text-slate-900 dark:text-slate-100">
        {{ title }}
      </h2>
      <div class="flex items-center gap-3">
        <button
          v-if="actionLabel"
          type="button"
          class="shrink-0 text-xs font-medium text-slate-500 transition hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100"
          @click="emit('action')"
        >
          {{ actionLabel }}
        </button>
      </div>
    </header>

    <div class="mt-4">
      <div v-if="hasItems" class="space-y-2">
        <component
          v-for="item in normalizedItems"
          :key="item.id"
          :is="clickable ? 'button' : 'div'"
          :type="clickable ? 'button' : undefined"
          class="block w-full rounded-lg px-2 py-2 text-left text-sm leading-6 text-slate-600 dark:text-slate-300"
          :class="clickable ? 'transition hover:bg-slate-50 hover:text-slate-900 dark:hover:bg-slate-800/70 dark:hover:text-slate-100' : ''"
          @click="clickable ? emit('select', item) : undefined"
        >
          <span class="flex items-start gap-2">
            <span
              v-if="item.badge"
              class="mt-0.5 inline-flex shrink-0 rounded-md bg-[#dcecff] px-2 py-0.5 text-xs font-semibold text-[#4f8ff7]"
            >
              {{ item.badge }}
            </span>
            <span class="line-clamp-2">{{ item.label }}</span>
          </span>
        </component>
      </div>

      <div v-else class="rounded-lg bg-[#fafafa] px-3 py-3 text-sm text-slate-500 dark:bg-slate-800/70 dark:text-slate-400">
        {{ props.emptyText }}
      </div>
    </div>
  </section>
</template>
