<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Download, Flag } from "lucide-vue-next";

import SurfaceCard from "../../components/ui/SurfaceCard.vue";
import { HttpError, httpClient } from "../../lib/http/client";
import { readApiError } from "../../lib/http/helpers";
import { ensureSessionReceiptCode, readStoredReceiptCode } from "../../lib/receiptCode";
import { hasAdminPermission } from "../../lib/admin/session";
import { renderSimpleMarkdown } from "../../lib/markdown";

interface FileDetailResponse {
  id: string;
  title: string;
  extension: string;
  folder_id: string;
  path: string;
  description: string;
  original_name: string;
  mime_type: string;
  size: number;
  uploaded_at: string;
  download_count: number;
}

const route = useRoute();
const router = useRouter();
const detail = ref<FileDetailResponse | null>(null);
const loading = ref(false);
const error = ref("");
const message = ref("");
const saveError = ref("");
const saving = ref(false);
const editFileName = ref("");
const editDescription = ref("");
const descriptionEditorOpen = ref(false);
const canManageResourceDescriptions = ref(false);
const deleteDialogOpen = ref(false);
const deletePassword = ref("");
const deleteSubmitting = ref(false);
const deleteError = ref("");
const feedbackModalOpen = ref(false);
const feedbackSuccessModalOpen = ref(false);
const feedbackDescription = ref("");
const feedbackSubmitting = ref(false);
const feedbackMessage = ref("");
const feedbackError = ref("");
const currentReceiptCode = ref("");
const fileID = computed(() => String(route.params.fileID ?? ""));
const downloadURL = computed(() => `/api/public/files/${encodeURIComponent(fileID.value)}/download`);
const descriptionHTML = computed(() => renderSimpleMarkdown(detail.value?.description ?? ""));
const feedbackSubmitDisabled = computed(() => feedbackSubmitting.value || !feedbackDescription.value.trim());
const primaryDetailRows = computed(() => {
  if (!detail.value) {
    return [];
  }

  return [
    { label: "文件名", value: detail.value.original_name },
    { label: "所属文件夹", value: detail.value.path || "主页根目录" },
  ];
});
const secondaryDetailRows = computed(() => {
  if (!detail.value) {
    return [];
  }

  return [
    { label: "下载量", value: String(detail.value.download_count) },
    { label: "文件大小", value: formatSize(detail.value.size) },
    { label: "更新时间", value: formatDate(detail.value.uploaded_at) },
  ];
});
const editorDirty = computed(() => {
  if (!detail.value) {
    return false;
  }

  return (
    editFileName.value.trim() !== detail.value.original_name ||
    editDescription.value.trim() !== (detail.value.description ?? "")
  );
});

onMounted(() => {
  void Promise.all([loadDetail(), loadAdminPermission(), syncSessionReceiptCode()]);
});

watch(fileID, () => {
  void Promise.all([loadDetail(), loadAdminPermission(), syncSessionReceiptCode()]);
});

async function loadDetail() {
  loading.value = true;
  error.value = "";
  detail.value = null;
  try {
    detail.value = await httpClient.get<FileDetailResponse>(`/public/files/${encodeURIComponent(fileID.value)}`);
    if (detail.value) {
      editFileName.value = detail.value.original_name;
      editDescription.value = detail.value.description;
    }
  } catch (err: unknown) {
    if (err instanceof HttpError && err.status === 404) {
      error.value = "文件不存在或未公开。";
    } else {
      error.value = "加载文件详情失败。";
    }
  } finally {
    loading.value = false;
  }
}

async function loadAdminPermission() {
  canManageResourceDescriptions.value = await hasAdminPermission("resource_moderation");
}

function openDescriptionEditor() {
  editFileName.value = detail.value?.original_name ?? "";
  editDescription.value = detail.value?.description ?? "";
  saveError.value = "";
  message.value = "";
  descriptionEditorOpen.value = true;
}

function closeDescriptionEditor() {
  descriptionEditorOpen.value = false;
  saving.value = false;
  saveError.value = "";
  editFileName.value = detail.value?.original_name ?? "";
  editDescription.value = detail.value?.description ?? "";
}

function openDeleteDialog() {
  deletePassword.value = "";
  deleteError.value = "";
  deleteDialogOpen.value = true;
}

function openFeedbackModal() {
  feedbackDescription.value = "";
  feedbackError.value = "";
  feedbackMessage.value = "";
  feedbackModalOpen.value = true;
  void syncSessionReceiptCode();
}

function closeFeedbackModal() {
  feedbackModalOpen.value = false;
}

function closeFeedbackSuccessModal() {
  feedbackSuccessModalOpen.value = false;
}

function closeDeleteDialog() {
  deleteDialogOpen.value = false;
  deletePassword.value = "";
  deleteError.value = "";
  deleteSubmitting.value = false;
}

async function saveDescription() {
  if (!detail.value || !editorDirty.value) return;
  const parsed = splitEditableFileName(editFileName.value);
  if (!parsed) {
    saveError.value = "请输入有效的文件名。";
    return;
  }
  saving.value = true;
  saveError.value = "";
  message.value = "";
  try {
    await httpClient.request(`/admin/resources/files/${encodeURIComponent(detail.value.id)}`, {
      method: "PUT",
      body: {
        title: parsed.title,
        extension: parsed.extension,
        description: editDescription.value.trim(),
      },
    });
    message.value = "文件信息已更新。";
    await loadDetail();
    descriptionEditorOpen.value = false;
  } catch (err: unknown) {
    saveError.value = readApiError(err, "更新文件简介失败。");
  } finally {
    saving.value = false;
  }
}

async function confirmDeleteFile() {
  if (!detail.value) {
    return;
  }
  if (!deletePassword.value.trim()) {
    deleteError.value = "请输入当前管理员密码。";
    return;
  }

  deleteSubmitting.value = true;
  deleteError.value = "";
  try {
    await httpClient.request(`/admin/resources/files/${encodeURIComponent(detail.value.id)}`, {
      method: "DELETE",
      body: { password: deletePassword.value },
    });
    closeDeleteDialog();
    goBack();
  } catch (err: unknown) {
    deleteError.value = readApiError(err, "删除文件失败。");
  } finally {
    deleteSubmitting.value = false;
  }
}

async function submitFeedback() {
  if (!detail.value || !feedbackDescription.value.trim()) {
    return;
  }

  feedbackSubmitting.value = true;
  feedbackMessage.value = "";
  feedbackError.value = "";
  try {
    const response = await httpClient.post<{ receipt_code: string }>("/public/reports", {
      file_id: detail.value.id,
      folder_id: "",
      reason: "content_error",
      description: feedbackDescription.value.trim(),
    });
    feedbackMessage.value = `反馈已提交，请保存回执码 ${response.receipt_code}。`;
    currentReceiptCode.value = response.receipt_code;
    window.sessionStorage.setItem("openshare_receipt_code", response.receipt_code);
    closeFeedbackModal();
    feedbackSuccessModalOpen.value = true;
  } catch (err: unknown) {
    if (err instanceof HttpError && err.status === 400) {
      feedbackError.value = "请填写问题说明。";
    } else if (err instanceof HttpError && err.status === 404) {
      feedbackError.value = "文件不存在或已下线。";
    } else {
      feedbackError.value = "提交反馈失败。";
    }
  } finally {
    feedbackSubmitting.value = false;
  }
}

async function syncSessionReceiptCode() {
  try {
    currentReceiptCode.value = await ensureSessionReceiptCode();
  } catch {
    currentReceiptCode.value = readStoredReceiptCode();
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

function normalizeExtensionInput(value: string) {
  return value.trim().replace(/^\.+/, "").toLowerCase();
}

function splitEditableFileName(value: string) {
  const normalized = value.trim();
  if (!normalized) {
    return null;
  }
  const lastDot = normalized.lastIndexOf(".");
  if (lastDot <= 0 || lastDot === normalized.length - 1) {
    return {
      title: normalized,
      extension: "",
    };
  }
  return {
    title: normalized.slice(0, lastDot).trim(),
    extension: normalizeExtensionInput(normalized.slice(lastDot + 1)),
  };
}

function goBack() {
  const folderID = detail.value?.folder_id?.trim() ?? "";
  if (folderID) {
    void router.push({ name: "public-home", query: { folder: folderID } });
    return;
  }
  void router.push({ name: "public-home" });
}

function downloadFile() {
  const link = document.createElement("a");
  link.href = downloadURL.value;
  link.rel = "noopener";
  document.body.appendChild(link);
  link.click();
  link.remove();

  if (detail.value) {
    detail.value = {
      ...detail.value,
      download_count: detail.value.download_count + 1,
    };
  }
}
</script>

<template>
  <section class="app-container py-6 sm:py-8 sm:py-10">
    <div class="mx-auto w-full max-w-4xl space-y-6">
      <SurfaceCard>
        <p v-if="loading" class="text-sm text-slate-500">加载中…</p>

        <div v-else-if="error" class="space-y-4">
          <p class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{{ error }}</p>
          <div class="flex flex-col gap-3 sm:flex-row">
            <button type="button" class="btn-secondary w-full sm:w-auto" @click="goBack">返回上一页</button>
            <button type="button" class="btn-primary w-full sm:w-auto" @click="$router.push({ name: 'public-home' })">返回首页</button>
          </div>
        </div>

        <template v-else-if="detail">
          <p v-if="message" class="mb-5 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">{{ message }}</p>

          <section>
            <div class="space-y-4">
              <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                <div class="space-y-2">
                  <p class="text-xs font-semibold uppercase tracking-[0.18em] text-blue-600">File Info</p>
                  <h3 class="text-2xl font-semibold tracking-tight text-slate-900 sm:text-3xl">文件详情</h3>
                </div>
                <div class="flex flex-wrap items-start gap-3 sm:flex-nowrap lg:justify-end">
                  <button type="button" class="btn-secondary w-full sm:w-auto" @click="goBack">返回文件夹</button>
                  <button
                    v-if="canManageResourceDescriptions"
                    type="button"
                    class="btn-secondary w-full sm:w-auto"
                    @click="openDescriptionEditor"
                  >
                    编辑
                  </button>
                  <button
                    v-if="canManageResourceDescriptions"
                    type="button"
                    class="btn-secondary w-full text-rose-600 hover:border-rose-200 hover:bg-rose-50 hover:text-rose-700 sm:w-auto"
                    @click="openDeleteDialog"
                  >
                    删除
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-slate-200 bg-white text-slate-500 transition-[transform,background-color,border-color,box-shadow,color] duration-200 hover:-translate-y-0.5 hover:border-slate-300 hover:bg-[#fafafa] hover:text-slate-900 hover:shadow-sm hover:shadow-slate-950/[0.08]"
                    aria-label="反馈文件"
                    @click="openFeedbackModal"
                  >
                    <Flag class="h-4 w-4" />
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-slate-200 bg-white text-slate-700 transition-[transform,background-color,border-color,box-shadow,color] duration-200 hover:-translate-y-0.5 hover:border-slate-300 hover:bg-[#fafafa] hover:text-slate-900 hover:shadow-sm hover:shadow-slate-950/[0.08]"
                    aria-label="下载文件"
                    @click="downloadFile"
                  >
                    <Download class="h-4 w-4" />
                  </button>
                </div>
              </div>

              <div class="min-w-0 space-y-3">
                <div class="grid gap-x-8 gap-y-3 lg:grid-cols-2">
                  <div
                    v-for="item in primaryDetailRows"
                    :key="item.label"
                    class="grid min-w-0 grid-cols-[88px_minmax(0,1fr)] items-baseline gap-x-3 text-sm"
                  >
                    <span class="text-slate-500">{{ item.label }}</span>
                    <span
                      class="min-w-0 truncate font-medium text-slate-900"
                      :title="item.value"
                    >
                      {{ item.value }}
                    </span>
                  </div>
                </div>
                <div class="grid gap-x-8 gap-y-3 sm:grid-cols-2 lg:grid-cols-3">
                  <div
                    v-for="item in secondaryDetailRows"
                    :key="item.label"
                    class="grid min-w-0 grid-cols-[88px_minmax(0,1fr)] items-baseline gap-x-3 text-sm"
                  >
                    <span class="text-slate-500">{{ item.label }}</span>
                    <span
                      class="min-w-0 truncate font-medium text-slate-900"
                      :title="item.value"
                    >
                      {{ item.value }}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <div class="mt-4 rounded-3xl border border-slate-200 bg-white px-4 py-4 sm:px-5 sm:py-5">
              <div
                v-if="descriptionHTML"
                class="markdown-content"
                v-html="descriptionHTML"
              />
              <p v-else class="text-sm text-slate-400">该文件暂无简介orz</p>
            </div>
          </section>
        </template>
      </SurfaceCard>
    </div>

    <Teleport to="body">
      <Transition name="modal-shell">
      <div v-if="deleteDialogOpen && detail" class="fixed inset-0 z-[120] flex items-center justify-center bg-slate-950/30 px-4">
        <div class="modal-card w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
          <div>
            <h3 class="text-lg font-semibold text-slate-900">确认删除文件</h3>
            <p class="mt-2 text-sm leading-6 text-slate-500">
              删除后将无法恢复。确认删除
              <span class="font-medium text-slate-900">{{ detail.original_name }}</span>
              吗？
            </p>
          </div>
          <div class="mt-6 space-y-4">
            <input v-model="deletePassword" type="password" class="field" placeholder="输入当前管理员密码确认删除" />
            <p v-if="deleteError" class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
              {{ deleteError }}
            </p>
            <div class="flex justify-end gap-3">
              <button type="button" class="btn-secondary" @click="closeDeleteDialog">取消</button>
              <button
                type="button"
                class="inline-flex h-11 items-center rounded-xl bg-rose-600 px-5 text-sm font-medium text-white transition hover:bg-rose-700"
                :disabled="deleteSubmitting"
                @click="confirmDeleteFile"
              >
                {{ deleteSubmitting ? "删除中…" : "确认删除" }}
              </button>
            </div>
          </div>
        </div>
      </div>
      </Transition>
    </Teleport>

    <Teleport to="body">
      <Transition name="modal-shell">
      <div v-if="feedbackSuccessModalOpen" class="fixed inset-0 z-[120] bg-slate-950/40 backdrop-blur-sm">
        <div class="flex min-h-screen items-center justify-center px-4 py-6">
          <div class="modal-card w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
            <div class="space-y-3">
              <h3 class="text-lg font-semibold text-slate-900">提交成功</h3>
              <p class="text-sm leading-6 text-slate-600">{{ feedbackMessage }}</p>
            </div>
            <div class="mt-6 flex justify-end">
              <button type="button" class="btn-primary" @click="closeFeedbackSuccessModal">知道了</button>
            </div>
          </div>
        </div>
      </div>
      </Transition>
    </Teleport>

    <Teleport to="body">
      <Transition name="modal-shell">
      <div v-if="feedbackModalOpen && detail" class="fixed inset-0 z-[120] bg-slate-950/40 backdrop-blur-sm">
        <div class="flex min-h-screen items-center justify-center px-4 py-6">
          <div class="modal-card panel w-full max-w-2xl overflow-hidden p-6">
            <div class="flex items-start justify-between gap-4 border-b border-slate-200 pb-5">
              <div class="space-y-1">
                <h3 class="text-lg font-semibold text-slate-900">反馈中心</h3>
                <p class="text-sm text-slate-500">填写问题说明后提交，我们会尽快处理。</p>
              </div>
              <button type="button" class="btn-secondary" @click="closeFeedbackModal">关闭</button>
            </div>

            <div class="mt-6 space-y-5">
              <div class="rounded-2xl border border-slate-200 bg-[#fafafafa] px-4 py-3">
                <p class="text-xs font-semibold uppercase tracking-[0.12em] text-slate-400">当前对象</p>
                <p class="mt-1 text-sm leading-6 text-slate-700">{{ detail.original_name }}</p>
              </div>

              <label class="space-y-2">
                <span class="text-sm font-medium text-slate-700">回执码</span>
                <div class="rounded-2xl border border-slate-200 bg-[#fafafafa] px-4 py-3">
                  <p class="text-sm font-semibold tracking-[0.12em] text-slate-900">
                    {{ currentReceiptCode || "当前会话回执码暂未同步" }}
                  </p>
                </div>
              </label>

              <label class="space-y-2">
                <span class="text-sm font-medium text-slate-700">问题说明</span>
                <textarea
                  v-model="feedbackDescription"
                  rows="5"
                  class="field-area"
                  placeholder="信息不当/侵权/内容错误……描述您遇到的问题，我们会尽快改进！"
                />
              </label>

              <p v-if="feedbackMessage" class="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
                {{ feedbackMessage }}
              </p>
              <p v-if="feedbackError" class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
                {{ feedbackError }}
              </p>

              <div class="flex justify-end gap-3 pt-1">
                <button type="button" class="btn-secondary" @click="closeFeedbackModal">取消</button>
                <button type="button" class="btn-primary" :disabled="feedbackSubmitDisabled" @click="submitFeedback">
                  {{ feedbackSubmitting ? "提交中…" : "提交反馈" }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
      </Transition>
    </Teleport>

    <Teleport to="body">
      <Transition name="modal-shell">
      <div v-if="descriptionEditorOpen" class="fixed inset-0 z-[120] bg-slate-950/40 backdrop-blur-sm">
        <div class="flex min-h-screen items-center justify-center px-4 py-6">
          <div class="modal-card panel w-full max-w-3xl overflow-hidden p-6">
            <div class="border-b border-slate-200 pb-4">
              <div>
                <h3 class="text-lg font-semibold text-slate-900">编辑文件信息</h3>
              </div>
            </div>

            <div class="mt-5 space-y-4">
              <label class="space-y-2">
                <span class="text-sm font-medium text-slate-700">文件名</span>
                <input
                  v-model="editFileName"
                  class="field"
                  :disabled="!canManageResourceDescriptions"
                  placeholder="输入完整文件名，例如 example.xlsx"
                />
              </label>

              <textarea
                v-model="editDescription"
                rows="10"
                class="field-area"
                placeholder="输入文件简介，简介支持简单 Markdown。"
              />

              <p v-if="saveError" class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
                {{ saveError }}
              </p>

              <div class="flex justify-end gap-3">
                <button type="button" class="btn-secondary" @click="closeDescriptionEditor">取消</button>
                <button type="button" class="btn-primary" :disabled="saving || !editorDirty" @click="saveDescription">
                  {{ saving ? "保存中…" : "保存更改" }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
      </Transition>
    </Teleport>
  </section>
</template>
