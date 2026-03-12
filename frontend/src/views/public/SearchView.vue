<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";

import ReportDialog from "../../components/ReportDialog.vue";
import { httpClient } from "../../lib/http/client";

interface SearchResultItem {
  entity_type: "file" | "folder";
  id: string;
  name: string;
  tags: string[];
  size?: number;
  download_count?: number;
  uploaded_at?: string;
}

interface SearchResponse {
  items: SearchResultItem[];
  page: number;
  page_size: number;
  total: number;
}

interface PublicFolderItem {
  id: string;
  name: string;
}

interface PublicFolderListResponse {
  items: PublicFolderItem[];
}

const route = useRoute();
const router = useRouter();

const keyword = ref("");
const activeTags = ref<string[]>([]);
const tagInput = ref("");
const folderID = ref("");
const page = ref(1);
const pageSize = 20;

const results = ref<SearchResultItem[]>([]);
const total = ref(0);
const loading = ref(false);
const searched = ref(false);

const allFolders = ref<{ id: string; name: string }[]>([]);

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)));

let debounceTimer: ReturnType<typeof setTimeout> | null = null;

onMounted(async () => {
  await loadFolders();

  // Restore from URL query params
  if (route.query.q) keyword.value = String(route.query.q);
  if (route.query.tag) {
    const raw = Array.isArray(route.query.tag) ? route.query.tag : [route.query.tag];
    activeTags.value = raw.filter(Boolean).map(String);
  }
  if (route.query.folder_id) folderID.value = String(route.query.folder_id);
  if (route.query.page) page.value = Math.max(1, Number(route.query.page) || 1);

  if (keyword.value || activeTags.value.length > 0) {
    await doSearch();
  }
});

watch([keyword], () => {
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => {
    page.value = 1;
    doSearch();
  }, 350);
});

watch([activeTags, folderID], () => {
  page.value = 1;
  doSearch();
});

async function doSearch() {
  const q = keyword.value.trim();
  if (!q && activeTags.value.length === 0) {
    results.value = [];
    total.value = 0;
    searched.value = false;
    syncURL();
    return;
  }

  loading.value = true;
  searched.value = true;

  try {
    const params = new URLSearchParams();
    if (q) params.set("q", q);
    for (const tag of activeTags.value) params.append("tag", tag);
    if (folderID.value) params.set("folder_id", folderID.value);
    params.set("page", String(page.value));
    params.set("page_size", String(pageSize));

    const response = await httpClient.get<SearchResponse>(`/public/search?${params.toString()}`);
    results.value = response.items ?? [];
    total.value = response.total ?? 0;
  } catch {
    results.value = [];
    total.value = 0;
  } finally {
    loading.value = false;
    syncURL();
  }
}

function syncURL() {
  const query: Record<string, string | string[]> = {};
  if (keyword.value.trim()) query.q = keyword.value.trim();
  if (activeTags.value.length > 0) query.tag = activeTags.value;
  if (folderID.value) query.folder_id = folderID.value;
  if (page.value > 1) query.page = String(page.value);
  router.replace({ query });
}

function addTag() {
  const tag = tagInput.value.trim();
  if (tag && !activeTags.value.includes(tag)) {
    activeTags.value = [...activeTags.value, tag];
  }
  tagInput.value = "";
}

function removeTag(tag: string) {
  activeTags.value = activeTags.value.filter((t) => t !== tag);
}

function onTagInputKeydown(event: KeyboardEvent) {
  if (event.key === "Enter" || event.key === ",") {
    event.preventDefault();
    addTag();
  }
  if (event.key === "Backspace" && tagInput.value === "" && activeTags.value.length > 0) {
    activeTags.value = activeTags.value.slice(0, -1);
  }
}

function clickTag(tag: string) {
  if (!activeTags.value.includes(tag)) {
    activeTags.value = [...activeTags.value, tag];
  }
}

function goToPage(p: number) {
  if (p < 1 || p > totalPages.value) return;
  page.value = p;
  doSearch();
}

async function loadFolders() {
  try {
    const result: { id: string; name: string }[] = [];
    async function loadLevel(parentId: string | null, prefix: string) {
      let url = "/public/folders";
      if (parentId) url += `?parent_id=${encodeURIComponent(parentId)}`;
      const response = await httpClient.get<PublicFolderListResponse>(url);
      for (const item of response.items ?? []) {
        const displayName = prefix ? `${prefix} / ${item.name}` : item.name;
        result.push({ id: item.id, name: displayName });
        await loadLevel(item.id, displayName);
      }
    }
    await loadLevel(null, "");
    allFolders.value = result;
  } catch {
    allFolders.value = [];
  }
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function formatSize(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function clearAll() {
  keyword.value = "";
  activeTags.value = [];
  folderID.value = "";
  page.value = 1;
  results.value = [];
  total.value = 0;
  searched.value = false;
  syncURL();
}

// Report dialog
const reportVisible = ref(false);
const reportTargetType = ref<"file" | "folder">("file");
const reportTargetId = ref("");
const reportTargetName = ref("");

function openReport(item: SearchResultItem) {
  reportTargetType.value = item.entity_type;
  reportTargetId.value = item.id;
  reportTargetName.value = item.name;
  reportVisible.value = true;
}
</script>

<template>
  <section class="space-y-6">
    <!-- Search hero -->
    <header class="rounded-[32px] bg-slate-950 px-8 py-10 text-white">
      <p class="text-sm font-semibold uppercase tracking-[0.28em] text-blue-300">Search</p>
      <h2 class="mt-4 text-3xl font-semibold leading-tight">搜索资料</h2>

      <!-- Search bar -->
      <div class="mt-6 flex gap-3">
        <div class="relative flex-1">
          <svg class="absolute left-4 top-1/2 h-5 w-5 -translate-y-1/2 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            v-model="keyword"
            type="text"
            placeholder="搜索文件名或文件夹名..."
            class="w-full rounded-2xl border border-slate-700 bg-slate-900 py-4 pl-12 pr-4 text-base text-white placeholder-slate-500 outline-none transition focus:border-blue-400 focus:ring-1 focus:ring-blue-400"
          />
        </div>
        <button
          v-if="keyword || activeTags.length > 0 || folderID"
          class="rounded-2xl border border-slate-700 px-5 py-4 text-sm font-medium text-slate-300 transition hover:bg-slate-800 hover:text-white"
          @click="clearAll"
        >
          清除
        </button>
      </div>

      <!-- Tag filter chips -->
      <div class="mt-4 flex flex-wrap items-center gap-2">
        <span
          v-for="tag in activeTags"
          :key="tag"
          class="inline-flex items-center gap-1 rounded-full bg-blue-500/20 px-3 py-1.5 text-sm font-medium text-blue-200"
        >
          {{ tag }}
          <button class="ml-0.5 rounded-full p-0.5 transition hover:bg-blue-500/30" @click="removeTag(tag)">
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </span>
        <input
          v-model="tagInput"
          type="text"
          placeholder="添加 Tag 过滤（回车确认）"
          class="min-w-[180px] flex-1 rounded-xl border-none bg-transparent px-2 py-1.5 text-sm text-slate-300 placeholder-slate-500 outline-none"
          @keydown="onTagInputKeydown"
        />
      </div>

      <!-- Folder scope -->
      <div class="mt-3">
        <select
          v-model="folderID"
          class="rounded-xl border border-slate-700 bg-slate-900 px-4 py-2.5 text-sm text-slate-300 outline-none transition focus:border-blue-400"
        >
          <option value="">全部目录</option>
          <option v-for="folder in allFolders" :key="folder.id" :value="folder.id">
            {{ folder.name }}
          </option>
        </select>
      </div>
    </header>

    <!-- Results area -->
    <div v-if="loading" class="flex items-center justify-center py-16">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-slate-300 border-t-blue-600" />
      <span class="ml-3 text-sm text-slate-500">搜索中...</span>
    </div>

    <div v-else-if="searched && results.length === 0" class="rounded-[28px] border border-slate-200 bg-white px-8 py-16 text-center shadow-sm">
      <svg class="mx-auto h-12 w-12 text-slate-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
      </svg>
      <p class="mt-4 text-lg font-medium text-slate-600">没有找到匹配的资料</p>
      <p class="mt-2 text-sm text-slate-400">试试其他关键词或减少过滤条件</p>
    </div>

    <div v-else-if="results.length > 0" class="space-y-4">
      <div class="flex items-center justify-between">
        <p class="text-sm text-slate-500">
          找到 <span class="font-semibold text-slate-900">{{ total }}</span> 条结果
          <template v-if="totalPages > 1">，第 {{ page }} / {{ totalPages }} 页</template>
        </p>
      </div>

      <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <article
          v-for="item in results"
          :key="`${item.entity_type}-${item.id}`"
          class="rounded-[24px] border border-slate-200 bg-white p-5 shadow-sm transition hover:shadow-md"
        >
          <!-- Type badge -->
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span
                  class="shrink-0 rounded-lg px-2 py-0.5 text-xs font-semibold"
                  :class="item.entity_type === 'file' ? 'bg-blue-100 text-blue-700' : 'bg-amber-100 text-amber-700'"
                >
                  {{ item.entity_type === "file" ? "文件" : "文件夹" }}
                </span>
                <h4 class="truncate text-base font-semibold text-slate-900">{{ item.name }}</h4>
              </div>

              <div v-if="item.uploaded_at" class="mt-2 text-xs text-slate-400">
                {{ formatDate(item.uploaded_at) }}
              </div>
            </div>

            <a
              v-if="item.entity_type === 'file'"
              :href="`/api/public/files/${item.id}/download`"
              class="shrink-0 rounded-full bg-slate-900 px-4 py-2 text-xs font-semibold text-white transition hover:bg-slate-800"
            >
              下载
            </a>
            <button
              v-else
              class="shrink-0 rounded-full border border-slate-200 px-4 py-2 text-xs font-semibold text-slate-700 transition hover:bg-slate-100"
              @click="folderID = item.id"
            >
              进入
            </button>
          </div>

          <!-- Tags -->
          <div class="mt-3 flex flex-wrap gap-1.5">
            <button
              v-for="tag in item.tags"
              :key="tag"
              class="rounded-full bg-blue-50 px-2.5 py-0.5 text-xs font-medium text-blue-700 transition hover:bg-blue-100"
              @click="clickTag(tag)"
            >
              {{ tag }}
            </button>
            <span v-if="item.tags.length === 0" class="text-xs text-slate-400">无 Tag</span>
          </div>

          <!-- File stats -->
          <div v-if="item.entity_type === 'file'" class="mt-4 flex items-center gap-4 text-xs text-slate-500">
            <span v-if="item.size != null">{{ formatSize(item.size) }}</span>
            <span v-if="item.download_count != null">下载 {{ item.download_count }}</span>
            <button
              class="ml-auto text-xs text-slate-400 transition hover:text-rose-500"
              @click="openReport(item)"
            >
              举报
            </button>
          </div>
          <div v-else class="mt-4 flex justify-end">
            <button
              class="text-xs text-slate-400 transition hover:text-rose-500"
              @click="openReport(item)"
            >
              举报
            </button>
          </div>
        </article>
      </div>

      <!-- Pagination -->
      <nav v-if="totalPages > 1" class="flex items-center justify-center gap-2 pt-4">
        <button
          class="rounded-xl border border-slate-200 px-4 py-2 text-sm font-medium text-slate-700 transition hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-40"
          :disabled="page <= 1"
          @click="goToPage(page - 1)"
        >
          上一页
        </button>
        <template v-for="p in totalPages" :key="p">
          <button
            v-if="p === 1 || p === totalPages || (p >= page - 2 && p <= page + 2)"
            class="rounded-xl px-3.5 py-2 text-sm font-medium transition"
            :class="p === page ? 'bg-blue-700 text-white' : 'text-slate-600 hover:bg-slate-100'"
            @click="goToPage(p)"
          >
            {{ p }}
          </button>
          <span
            v-else-if="p === page - 3 || p === page + 3"
            class="px-1 text-slate-400"
          >
            ...
          </span>
        </template>
        <button
          class="rounded-xl border border-slate-200 px-4 py-2 text-sm font-medium text-slate-700 transition hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-40"
          :disabled="page >= totalPages"
          @click="goToPage(page + 1)"
        >
          下一页
        </button>
      </nav>
    </div>

    <!-- Empty state before search -->
    <div v-else class="rounded-[28px] border border-slate-200 bg-white px-8 py-16 text-center shadow-sm">
      <svg class="mx-auto h-14 w-14 text-slate-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
      </svg>
      <p class="mt-4 text-lg font-medium text-slate-600">输入关键词或 Tag 开始搜索</p>
      <p class="mt-2 text-sm text-slate-400">支持文件名、文件夹名、Tag 搜索，支持组合过滤</p>
    </div>

    <ReportDialog
      v-model:visible="reportVisible"
      :target-type="reportTargetType"
      :target-id="reportTargetId"
      :target-name="reportTargetName"
    />
  </section>
</template>
