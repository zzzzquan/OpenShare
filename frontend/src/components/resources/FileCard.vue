<script setup lang="ts">
import { FileText } from "lucide-vue-next";

withDefaults(
  defineProps<{
    title: string;
    description?: string;
    size?: string;
    updatedAt?: string;
    uploader?: string;
    source?: string;
    badge?: string;
    actionText?: string;
    thumbnailLabel?: string;
    actionHref?: string;
  }>(),
  {
    description: "",
    size: "",
    updatedAt: "",
    uploader: "",
    source: "",
    badge: "",
    actionText: "下载",
    thumbnailLabel: "FILE",
    actionHref: "",
  },
);
</script>

<template>
  <article class="panel group p-4 sm:p-5 transition hover:border-slate-300 dark:hover:border-slate-700">
    <div class="flex flex-col gap-4 sm:flex-row">
      <div class="flex h-12 w-12 shrink-0 flex-col items-center justify-center rounded-lg border border-slate-200 bg-[#fafafa] text-slate-500 dark:border-slate-800 dark:bg-slate-800 dark:text-slate-400">
        <FileText class="h-5 w-5" />
        <span class="mt-1 text-[10px] font-semibold tracking-[0.08em] text-slate-400">
          {{ thumbnailLabel }}
        </span>
      </div>

      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <h3 class="truncate text-base font-semibold tracking-tight text-slate-900 dark:text-slate-100">
              {{ title }}
            </h3>
            <p v-if="description" class="mt-2 line-clamp-2 text-sm leading-6 text-slate-500 dark:text-slate-400">
              {{ description }}
            </p>
          </div>

          <span
            v-if="badge"
            class="inline-flex rounded-md border border-slate-200 bg-[#fafafa] px-2 py-1 text-xs font-medium text-slate-600 dark:border-slate-800 dark:bg-slate-800 dark:text-slate-300"
          >
            {{ badge }}
          </span>
        </div>

        <div class="mt-4 flex flex-wrap gap-x-5 gap-y-2 text-sm text-slate-500 dark:text-slate-400">
          <span v-if="size">{{ size }}</span>
          <span v-if="updatedAt">{{ updatedAt }}</span>
          <span v-if="uploader">{{ uploader }}</span>
        </div>
        <div class="mt-5 flex flex-col gap-3 border-t border-slate-100 pt-4 sm:flex-row sm:items-center sm:justify-between dark:border-slate-800">
          <div class="min-w-0 text-sm text-slate-500 dark:text-slate-400">
            <span v-if="source" class="truncate">{{ source }}</span>
            <span v-else>暂无来源信息</span>
          </div>

          <a :href="actionHref || undefined" class="btn-primary w-full justify-center sm:w-auto">
            {{ actionText }}
          </a>
        </div>
      </div>
    </div>
  </article>
</template>
