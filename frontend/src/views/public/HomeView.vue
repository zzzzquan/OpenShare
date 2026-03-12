<script setup lang="ts">
import { computed, onMounted, ref } from "vue";

import ReportDialog from "../../components/ReportDialog.vue";
import { HttpError, httpClient } from "../../lib/http/client";

interface PublicFileItem {
  id: string;
  title: string;
  tags: string[];
  uploaded_at: string;
  download_count: number;
  size: number;
}

interface PublicFileListResponse {
  items: PublicFileItem[];
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

interface PublicSubmissionItem {
  title: string;
  status: "pending" | "approved" | "rejected";
  uploaded_at: string;
  download_count: number;
  reject_reason: string;
}

interface SubmissionLookupResult {
  receipt_code: string;
  items: PublicSubmissionItem[];
}

interface UploadResponse {
  receipt_code: string;
  status: "pending" | "approved" | "rejected";
  title: string;
  uploaded_at: string;
}

const cachedReceiptCodeKey = "openshare:last_receipt_code";

const uploadDescription = ref("");
const uploadTags = ref("");
const uploadReceiptCode = ref("");
const uploadFolderID = ref("");
const uploadFile = ref<File | null>(null);
const uploadMessage = ref("");
const uploadError = ref("");
const uploading = ref(false);

const files = ref<PublicFileItem[]>([]);
const listLoading = ref(false);
const listError = ref("");
const listSort = ref<"created_at_desc" | "download_count_desc" | "title_asc">("created_at_desc");

// Folder navigation
const folders = ref<PublicFolderItem[]>([]);
const currentFolderID = ref<string | null>(null);
const breadcrumbs = ref<{ id: string | null; name: string }[]>([]);

// Upload folder selector
const allFolders = ref<{ id: string; name: string; depth: number }[]>([]);

const receiptCode = ref("");
const records = ref<PublicSubmissionItem[]>([]);
const lookupLoading = ref(false);
const lookupError = ref("");

const totalFiles = computed(() => files.value.length);

onMounted(async () => {
  await Promise.all([loadFiles(), loadFolders(), loadAllFoldersForUpload()]);

  const cached = window.localStorage.getItem(cachedReceiptCodeKey);
  if (!cached) {
    return;
  }

  receiptCode.value = cached;
  // Auto-fill cached receipt code for next upload
  uploadReceiptCode.value = cached;
  void lookupSubmission();
});

async function loadFiles() {
  listLoading.value = true;
  listError.value = "";

  try {
    let url = `/public/files?sort=${listSort.value}&page=1&page_size=50`;
    if (currentFolderID.value) {
      url += `&folder_id=${encodeURIComponent(currentFolderID.value)}`;
    }
    const response = await httpClient.get<PublicFileListResponse>(url);
    files.value = response.items ?? [];
  } catch {
    listError.value = "加载公开资料失败，请稍后重试。";
  } finally {
    listLoading.value = false;
  }
}

async function loadFolders() {
  try {
    let url = "/public/folders";
    if (currentFolderID.value) {
      url += `?parent_id=${encodeURIComponent(currentFolderID.value)}`;
    }
    const response = await httpClient.get<PublicFolderListResponse>(url);
    folders.value = response.items ?? [];
  } catch {
    folders.value = [];
  }
}

async function loadAllFoldersForUpload() {
  try {
    // Load root folders, then recursively load subfolders for a flat select list
    const result: { id: string; name: string; depth: number }[] = [];
    async function loadLevel(parentId: string | null, depth: number, prefix: string) {
      let url = "/public/folders";
      if (parentId) {
        url += `?parent_id=${encodeURIComponent(parentId)}`;
      }
      const response = await httpClient.get<PublicFolderListResponse>(url);
      const items = response.items ?? [];
      for (const item of items) {
        const displayName = prefix ? `${prefix} / ${item.name}` : item.name;
        result.push({ id: item.id, name: displayName, depth });
        await loadLevel(item.id, depth + 1, displayName);
      }
    }
    await loadLevel(null, 0, "");
    allFolders.value = result;
  } catch {
    allFolders.value = [];
  }
}

async function navigateToFolder(folderId: string | null, folderName?: string) {
  if (folderId === null) {
    currentFolderID.value = null;
    breadcrumbs.value = [];
  } else {
    currentFolderID.value = folderId;
    // Check if we're going back in breadcrumbs
    const existingIndex = breadcrumbs.value.findIndex(b => b.id === folderId);
    if (existingIndex >= 0) {
      breadcrumbs.value = breadcrumbs.value.slice(0, existingIndex + 1);
    } else {
      breadcrumbs.value.push({ id: folderId, name: folderName ?? folderId });
    }
  }
  await Promise.all([loadFiles(), loadFolders()]);
}

async function submitUpload() {
  if (!uploadFile.value) {
    uploadError.value = "请选择要上传的文件。";
    uploadMessage.value = "";
    return;
  }

  const formData = new FormData();
  formData.set("description", uploadDescription.value);
  formData.set("file", uploadFile.value);

  const tags = uploadTags.value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);

  for (const tag of tags) {
    formData.append("tag", tag);
  }

  // Auto-fill receipt code from cache if empty
  let effectiveReceiptCode = uploadReceiptCode.value.trim();
  if (!effectiveReceiptCode) {
    const cached = window.localStorage.getItem(cachedReceiptCodeKey);
    if (cached) {
      effectiveReceiptCode = cached;
    }
  }
  if (effectiveReceiptCode) {
    formData.set("receipt_code", effectiveReceiptCode);
  }

  if (uploadFolderID.value) {
    formData.set("folder_id", uploadFolderID.value);
  }

  uploading.value = true;
  uploadError.value = "";
  uploadMessage.value = "";

  try {
    const response = await httpClient.post<UploadResponse>("/public/submissions", formData);
    uploadMessage.value = `上传成功，回执码：${response.receipt_code}`;
    receiptCode.value = response.receipt_code;
    uploadReceiptCode.value = response.receipt_code;
    window.localStorage.setItem(cachedReceiptCodeKey, response.receipt_code);
    uploadDescription.value = "";
    uploadTags.value = "";
    uploadFile.value = null;
    // Keep uploadFolderID and uploadReceiptCode for convenience
    await lookupSubmission();
  } catch (error: unknown) {
    if (error instanceof HttpError && typeof error.payload === "object" && error.payload && "error" in error.payload) {
      uploadError.value = String(error.payload.error);
    } else {
      uploadError.value = "上传失败，请稍后重试。";
    }
  } finally {
    uploading.value = false;
  }
}

async function lookupSubmission() {
  const normalized = receiptCode.value.trim();
  if (!normalized) {
    lookupError.value = "请输入回执码。";
    records.value = [];
    return;
  }

  lookupLoading.value = true;
  lookupError.value = "";

  try {
    const response = await httpClient.get<SubmissionLookupResult>(`/public/submissions/${encodeURIComponent(normalized)}`);
    records.value = response.items ?? [];
    receiptCode.value = response.receipt_code;
    uploadReceiptCode.value = response.receipt_code;
    window.localStorage.setItem(cachedReceiptCodeKey, response.receipt_code);
  } catch (error: unknown) {
    records.value = [];
    if (error instanceof HttpError && error.status === 404) {
      lookupError.value = "未找到对应回执码，请检查输入是否正确。";
    } else if (error instanceof HttpError && error.status === 400) {
      lookupError.value = "回执码格式无效。";
    } else {
      lookupError.value = "查询失败，请稍后重试。";
    }
  } finally {
    lookupLoading.value = false;
  }
}

function onFileSelected(event: Event) {
  const target = event.target as HTMLInputElement;
  uploadFile.value = target.files?.[0] ?? null;
}

function statusLabel(status: PublicSubmissionItem["status"]) {
  switch (status) {
    case "approved":
      return "已通过";
    case "rejected":
      return "已驳回";
    default:
      return "待审核";
  }
}

function statusClass(status: PublicSubmissionItem["status"]) {
  switch (status) {
    case "approved":
      return "bg-emerald-100 text-emerald-800";
    case "rejected":
      return "bg-rose-100 text-rose-800";
    default:
      return "bg-amber-100 text-amber-800";
  }
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function formatSize(size: number) {
  if (size < 1024) {
    return `${size} B`;
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`;
  }
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

// Report dialog
const reportVisible = ref(false);
const reportTargetType = ref<"file" | "folder">("file");
const reportTargetId = ref("");
const reportTargetName = ref("");

function openFileReport(file: PublicFileItem) {
  reportTargetType.value = "file";
  reportTargetId.value = file.id;
  reportTargetName.value = file.title;
  reportVisible.value = true;
}

function openFolderReport(folder: PublicFolderItem) {
  reportTargetType.value = "folder";
  reportTargetId.value = folder.id;
  reportTargetName.value = folder.name;
  reportVisible.value = true;
}
</script>

<template>
  <section class="space-y-8">
    <header class="rounded-[32px] bg-slate-950 px-8 py-10 text-white">
      <p class="text-sm font-semibold uppercase tracking-[0.28em] text-blue-300">Public Portal</p>
      <h2 class="mt-4 max-w-3xl text-4xl font-semibold leading-tight">
        OpenShare 学习资料共享平台
      </h2>
      <p class="mt-4 max-w-3xl text-base text-slate-300">
        浏览、下载和上传学习资料。上传的资料会经过管理员审核后公开。
      </p>
    </header>

    <div class="grid gap-6 xl:grid-cols-[1fr_1fr]">
      <article class="rounded-[28px] border border-slate-200 bg-white p-6 shadow-sm">
        <div class="flex items-center justify-between gap-4">
          <div>
            <p class="text-sm font-semibold uppercase tracking-[0.22em] text-blue-700">Upload</p>
            <h3 class="mt-2 text-2xl font-semibold text-slate-900">游客上传</h3>
          </div>
          <span class="rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-600">投稿后进入审核池</span>
        </div>

        <form class="mt-6 space-y-4" @submit.prevent="submitUpload">
          <label class="block">
            <span class="mb-2 block text-sm font-medium text-slate-700">描述</span>
            <textarea v-model="uploadDescription" rows="3" class="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm outline-none focus:border-blue-500 focus:bg-white" />
          </label>

          <label class="block">
            <span class="mb-2 block text-sm font-medium text-slate-700">上传到文件夹</span>
            <select v-model="uploadFolderID" class="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-900 outline-none focus:border-blue-500 focus:bg-white">
              <option value="">根目录（不选择文件夹）</option>
              <option v-for="folder in allFolders" :key="folder.id" :value="folder.id">
                {{ folder.name }}
              </option>
            </select>
          </label>

          <div class="grid gap-4 sm:grid-cols-2">
            <label class="block">
              <span class="mb-2 block text-sm font-medium text-slate-700">Tag（逗号分隔）</span>
              <input v-model="uploadTags" placeholder="数学, 期末" class="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm outline-none focus:border-blue-500 focus:bg-white" />
            </label>

            <label class="block">
              <span class="mb-2 block text-sm font-medium text-slate-700">回执码</span>
              <input v-model="uploadReceiptCode" placeholder="自动复用上次的回执码" class="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm outline-none focus:border-blue-500 focus:bg-white" />
            </label>
          </div>

          <label class="block">
            <span class="mb-2 block text-sm font-medium text-slate-700">文件</span>
            <input type="file" class="block w-full rounded-2xl border border-dashed border-slate-300 bg-slate-50 px-4 py-3 text-sm text-slate-600" @change="onFileSelected" />
          </label>

          <button type="submit" class="rounded-2xl bg-blue-700 px-5 py-3 text-sm font-semibold text-white transition hover:bg-blue-800 disabled:cursor-not-allowed disabled:bg-slate-400" :disabled="uploading">
            {{ uploading ? "上传中..." : "提交投稿" }}
          </button>
        </form>

        <p v-if="uploadMessage" class="mt-4 rounded-2xl bg-emerald-50 px-4 py-3 text-sm text-emerald-700">{{ uploadMessage }}</p>
        <p v-if="uploadError" class="mt-4 rounded-2xl bg-rose-50 px-4 py-3 text-sm text-rose-700">{{ uploadError }}</p>
      </article>

      <article class="rounded-[28px] border border-slate-200 bg-white p-6 shadow-sm">
        <div class="flex items-center justify-between gap-4">
          <div>
            <p class="text-sm font-semibold uppercase tracking-[0.22em] text-blue-700">Receipt Lookup</p>
            <h3 class="mt-2 text-2xl font-semibold text-slate-900">我的上传</h3>
          </div>
          <span class="rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-600">回执码自动缓存复用</span>
        </div>

        <form class="mt-6 space-y-4" @submit.prevent="lookupSubmission">
          <label class="block">
            <span class="mb-2 block text-sm font-medium text-slate-700">回执码</span>
            <input v-model="receiptCode" placeholder="例如：A8K2D7Q4M9P1" class="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm outline-none focus:border-blue-500 focus:bg-white" />
          </label>

          <button type="submit" class="rounded-2xl bg-slate-900 px-5 py-3 text-sm font-semibold text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:bg-slate-400" :disabled="lookupLoading">
            {{ lookupLoading ? "查询中..." : "查询投稿记录" }}
          </button>
        </form>

        <p v-if="lookupError" class="mt-4 rounded-2xl bg-rose-50 px-4 py-3 text-sm text-rose-700">{{ lookupError }}</p>

        <div v-if="records.length > 0" class="mt-5 space-y-3">
          <p class="text-xs uppercase tracking-[0.2em] text-slate-500">回执码：{{ receiptCode }}（共 {{ records.length }} 条投稿）</p>

          <div v-for="(record, index) in records" :key="index" class="rounded-[24px] bg-slate-50 p-5">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h4 class="text-lg font-semibold text-slate-900">{{ record.title }}</h4>
              </div>
              <span class="rounded-full px-3 py-1 text-xs font-semibold" :class="statusClass(record.status)">
                {{ statusLabel(record.status) }}
              </span>
            </div>

            <dl class="mt-4 grid gap-3 sm:grid-cols-3">
              <div class="rounded-2xl bg-white px-4 py-3">
                <dt class="text-xs uppercase tracking-[0.18em] text-slate-500">上传时间</dt>
                <dd class="mt-2 text-sm font-medium text-slate-900">{{ formatDate(record.uploaded_at) }}</dd>
              </div>
              <div class="rounded-2xl bg-white px-4 py-3">
                <dt class="text-xs uppercase tracking-[0.18em] text-slate-500">下载量</dt>
                <dd class="mt-2 text-sm font-medium text-slate-900">{{ record.download_count }}</dd>
              </div>
              <div class="rounded-2xl bg-white px-4 py-3">
                <dt class="text-xs uppercase tracking-[0.18em] text-slate-500">驳回原因</dt>
                <dd class="mt-2 text-sm font-medium text-slate-900">{{ record.reject_reason || "暂无" }}</dd>
              </div>
            </dl>
          </div>
        </div>
      </article>
    </div>

    <article class="rounded-[28px] border border-slate-200 bg-white p-6 shadow-sm">
      <div class="flex flex-wrap items-center justify-between gap-4">
        <div>
          <p class="text-sm font-semibold uppercase tracking-[0.22em] text-blue-700">Public Files</p>
          <h3 class="mt-2 text-2xl font-semibold text-slate-900">公开资料列表</h3>
        </div>
        <div class="flex items-center gap-3">
          <select v-model="listSort" class="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-900 outline-none focus:border-blue-500" @change="loadFiles">
            <option value="created_at_desc">最新上传</option>
            <option value="download_count_desc">下载量优先</option>
            <option value="title_asc">标题排序</option>
          </select>
          <button type="button" class="rounded-2xl border border-slate-200 px-4 py-3 text-sm font-medium text-slate-700 transition hover:bg-slate-100" @click="loadFiles">
            刷新
          </button>
        </div>
      </div>

      <!-- Breadcrumb navigation -->
      <div class="mt-4 flex flex-wrap items-center gap-1 text-sm">
        <button class="rounded-xl px-3 py-1 font-medium transition" :class="currentFolderID === null ? 'bg-blue-100 text-blue-800' : 'text-blue-600 hover:bg-blue-50'" @click="navigateToFolder(null)">
          全部资料
        </button>
        <template v-for="(crumb, index) in breadcrumbs" :key="crumb.id">
          <span class="text-slate-400">/</span>
          <button class="rounded-xl px-3 py-1 font-medium transition" :class="index === breadcrumbs.length - 1 ? 'bg-blue-100 text-blue-800' : 'text-blue-600 hover:bg-blue-50'" @click="navigateToFolder(crumb.id, crumb.name)">
            {{ crumb.name }}
          </button>
        </template>
      </div>

      <!-- Subfolders -->
      <div v-if="folders.length > 0" class="mt-4 flex flex-wrap gap-3">
        <div v-for="folder in folders" :key="folder.id" class="flex items-center gap-1">
          <button class="flex items-center gap-2 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm font-medium text-slate-700 transition hover:border-blue-300 hover:bg-blue-50" @click="navigateToFolder(folder.id, folder.name)">
            <svg class="h-4 w-4 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
            </svg>
            {{ folder.name }}
          </button>
          <button
            class="rounded-full p-1.5 text-slate-300 transition hover:text-rose-500"
            title="举报此文件夹"
            @click="openFolderReport(folder)"
          >
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01M3 12a9 9 0 1118 0 9 9 0 01-18 0z" />
            </svg>
          </button>
        </div>
      </div>

      <p class="mt-3 text-sm text-slate-500">当前展示 {{ totalFiles }} 条可下载公开资料。</p>
      <p v-if="listError" class="mt-4 rounded-2xl bg-rose-50 px-4 py-3 text-sm text-rose-700">{{ listError }}</p>
      <p v-else-if="listLoading" class="mt-4 text-sm text-slate-500">加载中...</p>

      <div v-else class="mt-6 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <article v-for="file in files" :key="file.id" class="rounded-[24px] border border-slate-200 bg-slate-50 p-5">
          <div class="flex items-start justify-between gap-4">
            <div>
              <h4 class="text-lg font-semibold text-slate-900">{{ file.title }}</h4>
              <p class="mt-2 text-sm text-slate-500">上传于 {{ formatDate(file.uploaded_at) }}</p>
            </div>
            <a :href="`/api/public/files/${file.id}/download`" class="rounded-full bg-slate-900 px-4 py-2 text-xs font-semibold text-white transition hover:bg-slate-800">
              下载
            </a>
          </div>

          <div class="mt-4 flex flex-wrap gap-2">
            <span v-for="tag in (file.tags ?? [])" :key="tag" class="rounded-full bg-blue-100 px-3 py-1 text-xs font-medium text-blue-800">
              {{ tag }}
            </span>
            <span v-if="(file.tags ?? []).length === 0" class="rounded-full bg-slate-200 px-3 py-1 text-xs font-medium text-slate-600">无 Tag</span>
          </div>

          <dl class="mt-5 grid grid-cols-2 gap-3 text-sm text-slate-600">
            <div class="rounded-2xl bg-white px-3 py-3">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-500">下载量</dt>
              <dd class="mt-1 font-semibold text-slate-900">{{ file.download_count }}</dd>
            </div>
            <div class="rounded-2xl bg-white px-3 py-3">
              <dt class="text-xs uppercase tracking-[0.16em] text-slate-500">大小</dt>
              <dd class="mt-1 font-semibold text-slate-900">{{ formatSize(file.size) }}</dd>
            </div>
          </dl>

          <div class="mt-3 flex justify-end">
            <button
              class="text-xs text-slate-400 transition hover:text-rose-500"
              @click="openFileReport(file)"
            >
              举报
            </button>
          </div>
        </article>
      </div>
    </article>

    <ReportDialog
      v-model:visible="reportVisible"
      :target-type="reportTargetType"
      :target-id="reportTargetId"
      :target-name="reportTargetName"
    />
  </section>
</template>
