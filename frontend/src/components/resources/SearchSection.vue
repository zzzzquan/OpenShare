<script setup lang="ts">
import { computed } from "vue";
import { Search, X } from "lucide-vue-next";

const props = withDefaults(
  defineProps<{
    embedded?: boolean;
    loading?: boolean;
    modelValue?: string;
  }>(),
  {
    embedded: false,
    loading: false,
    modelValue: "",
  },
);
const emit = defineEmits<{
  search: [keyword: string];
  clear: [];
  "update:modelValue": [value: string];
}>();

const keyword = computed({
  get: () => props.modelValue,
  set: (value: string) => emit("update:modelValue", value),
});
const canSearch = computed(() => keyword.value.trim().length > 0);

async function submitSearch() {
  if (!canSearch.value) {
    return;
  }
  emit("search", keyword.value.trim());
}

function clearSearch() {
  keyword.value = "";
  emit("clear");
}
</script>

<template>
  <section :class="props.embedded ? 'px-5 py-4 sm:px-6' : 'panel px-6 py-6'">
    <div class="space-y-4">
      <div class="space-y-3">
        <form class="flex flex-col gap-3 xl:flex-row xl:items-center" @submit.prevent="submitSearch">
          <label class="relative block min-w-0 flex-1">
            <Search class="pointer-events-none absolute left-5 top-1/2 h-5 w-5 -translate-y-1/2 text-slate-400" />
            <input
              v-model="keyword"
              type="text"
              placeholder="在该目录下搜索文件/文件夹"
              class="h-14 w-full rounded-lg border border-slate-300 bg-white pl-14 pr-14 text-[15px] text-slate-900 outline-none transition placeholder:text-slate-400 focus:border-slate-400 focus:ring-4 focus:ring-slate-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100 dark:placeholder:text-slate-500 dark:focus:border-slate-500 dark:focus:ring-slate-800"
            />
            <button
              v-if="keyword"
              type="button"
              class="absolute right-4 top-1/2 inline-flex h-8 w-8 -translate-y-1/2 items-center justify-center rounded-full text-slate-400 transition hover:bg-slate-100 hover:text-slate-700"
              aria-label="清除搜索"
              @click="clearSearch"
            >
              <X class="h-4 w-4" />
            </button>
          </label>

          <button
            type="submit"
            class="h-11 rounded-lg px-6 text-sm font-medium transition xl:shrink-0"
            :class="
              canSearch
                ? 'bg-slate-900 text-white hover:bg-slate-800 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-white'
                : 'cursor-not-allowed bg-slate-200 text-slate-500 dark:bg-slate-800 dark:text-slate-500'
            "
            :disabled="!canSearch || props.loading"
          >
            {{ props.loading ? "搜索中…" : "搜索" }}
          </button>
        </form>

      </div>
    </div>
  </section>
</template>
