<script setup lang="ts">
import { computed, onMounted, ref } from "vue";

import EmptyState from "../../components/ui/EmptyState.vue";
import PageHeader from "../../components/ui/PageHeader.vue";
import SurfaceCard from "../../components/ui/SurfaceCard.vue";
import { HttpError, httpClient } from "../../lib/http/client";
import { clearStoredReceiptCode, ensureSessionReceiptCode, readStoredReceiptCode } from "../../lib/receiptCode";

interface SubmissionLookupResponse {
  receipt_code: string;
  items: Array<{
    title: string;
    relative_path: string;
    status: string;
    uploaded_at: string;
    download_count: number;
    reject_reason?: string;
  }>;
}

interface FeedbackLookupResponse {
  receipt_code: string;
  items: Array<{
    target_name: string;
    target_path: string;
    description: string;
    review_reason: string;
    status: string;
    created_at: string;
    reviewed_at: string | null;
  }>;
}

const receiptCode = ref("");
const lookupLoading = ref(false);
const lookupError = ref("");
const submissionLookupResult = ref<SubmissionLookupResponse | null>(null);
const feedbackLookupResult = ref<FeedbackLookupResponse | null>(null);

const receiptRecords = computed(() => {
  const submissionItems = (submissionLookupResult.value?.items ?? []).map((item) => ({
    kind: "submission" as const,
    key: `submission-${item.title}-${item.uploaded_at}`,
    status: item.status,
    title: submissionDisplayName(item),
    createdAt: item.uploaded_at,
    relativePath: item.relative_path,
    description: "",
    reviewReason: item.reject_reason ?? "",
  }));

  const feedbackItems = (feedbackLookupResult.value?.items ?? []).map((item) => ({
    kind: "feedback" as const,
    key: `feedback-${item.target_name}-${item.created_at}`,
    status: item.status,
    title: item.target_name || "-",
    createdAt: item.created_at,
    relativePath: item.target_path,
    description: item.description,
    reviewReason: item.review_reason,
  }));

  return [...submissionItems, ...feedbackItems].sort(
    (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime(),
  );
});

onMounted(() => {
  void syncSessionReceiptCode();
  localStorage.removeItem("openshare_feedback_receipt_code");
});

async function lookupReceipt() {
  const code = receiptCode.value.trim();
  if (!code) {
    lookupError.value = "请输入回执码。";
    submissionLookupResult.value = null;
    feedbackLookupResult.value = null;
    return;
  }

  lookupLoading.value = true;
  lookupError.value = "";
  submissionLookupResult.value = null;
  feedbackLookupResult.value = null;

  const [submissionResult, feedbackResult] = await Promise.allSettled([
    httpClient.get<SubmissionLookupResponse>(`/public/submissions/${encodeURIComponent(code)}`),
    httpClient.get<FeedbackLookupResponse>(`/public/reports/${encodeURIComponent(code)}`),
  ]);

  const submissionError = submissionResult.status === "rejected" ? submissionResult.reason : null;
  const feedbackError = feedbackResult.status === "rejected" ? feedbackResult.reason : null;
  const fatalSubmissionError =
    submissionError instanceof HttpError ? submissionError.status !== 404 : Boolean(submissionError);
  const fatalFeedbackError =
    feedbackError instanceof HttpError ? feedbackError.status !== 404 : Boolean(feedbackError);

  if (fatalSubmissionError || fatalFeedbackError) {
    lookupError.value = "查询回执失败。";
    lookupLoading.value = false;
    return;
  }

  if (submissionResult.status === "fulfilled") {
    submissionLookupResult.value = submissionResult.value;
    sessionStorage.setItem("openshare_receipt_code", submissionResult.value.receipt_code);
  }
  if (feedbackResult.status === "fulfilled") {
    feedbackLookupResult.value = feedbackResult.value;
    sessionStorage.setItem("openshare_receipt_code", feedbackResult.value.receipt_code);
  }

  if (!submissionLookupResult.value && !feedbackLookupResult.value) {
    lookupError.value = "未找到对应信息。";
  }
  lookupLoading.value = false;
}

function clearReceipt() {
  clearStoredReceiptCode();
  submissionLookupResult.value = null;
  feedbackLookupResult.value = null;
  lookupError.value = "";
  localStorage.removeItem("openshare_feedback_receipt_code");
  void syncSessionReceiptCode();
}

async function syncSessionReceiptCode() {
  try {
    receiptCode.value = await ensureSessionReceiptCode();
  } catch {
    receiptCode.value = readStoredReceiptCode();
  }
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function submissionDisplayName(item: SubmissionLookupResponse["items"][number]) {
  const relativePath = item.relative_path?.trim();
  if (!relativePath) {
    return item.title;
  }

  const segments = relativePath.split("/").filter(Boolean);
  return segments[segments.length - 1] || item.title;
}

function submissionStatusLabel(status: string) {
  const labels: Record<string, string> = {
    pending: "待审核",
    approved: "已通过",
    rejected: "已驳回",
  };
  return labels[status] ?? status;
}

function feedbackStatusLabel(status: string) {
  const labels: Record<string, string> = {
    pending: "待处理",
    approved: "已处理",
    rejected: "已驳回",
  };
  return labels[status] ?? status;
}

function statusBadgeClass(status: string) {
  const styles: Record<string, string> = {
    pending: "bg-amber-50 text-amber-700 ring-1 ring-inset ring-amber-200",
    approved: "bg-emerald-50 text-emerald-700 ring-1 ring-inset ring-emerald-200",
    rejected: "bg-rose-50 text-rose-700 ring-1 ring-inset ring-rose-200",
  };
  return styles[status] ?? "bg-slate-100 text-slate-700 ring-1 ring-inset ring-slate-200";
}

</script>

<template>
  <div class="app-container py-8 sm:py-10">
    <div class="mx-auto w-full max-w-4xl">
      <SurfaceCard>
        <PageHeader
          eyebrow="Receipt"
          title="回执查询"
        />

        <div class="mt-6 rounded-xl border border-slate-200 bg-[#fafafa] px-4 py-3 text-sm leading-7 text-slate-600">
          本会话回执码为：<span class="font-semibold text-slate-900">{{ receiptCode || "暂未同步" }}</span>。请妥善保存该回执码，若清除浏览器缓存或更换浏览器/设备，该回执码将会改变。
        </div>

        <div class="mt-6 flex flex-col gap-3 sm:flex-row">
          <input
            v-model="receiptCode"
            class="field flex-1"
            placeholder="输入回执码"
            readonly
          />
          <button class="btn-secondary w-full sm:w-auto" :disabled="lookupLoading" @click="lookupReceipt">
            {{ lookupLoading ? "查询中…" : "查询" }}
          </button>
        </div>

        <div class="mt-4 flex flex-wrap gap-3">
          <button class="text-sm text-slate-500 transition hover:text-slate-900" @click="clearReceipt">
            清除本地回执码
          </button>
        </div>

        <p v-if="lookupError" class="mt-4 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
          {{ lookupError }}
        </p>
        <p v-else-if="lookupLoading" class="mt-4 text-sm text-slate-500">正在查询…</p>

        <div v-if="receiptRecords.length" class="mt-6 space-y-3">
          <article
            v-for="item in receiptRecords"
            :key="item.key"
            class="rounded-xl border border-slate-200 bg-white px-4 py-4 sm:px-5 sm:py-5"
          >
            <div class="space-y-4">
              <div>
                <span
                  class="rounded-md px-2.5 py-1 text-xs font-medium"
                  :class="statusBadgeClass(item.status)"
                >
                  {{ item.kind === "submission" ? submissionStatusLabel(item.status) : feedbackStatusLabel(item.status) }}
                </span>
                <p class="mt-3 text-sm text-slate-500">
                  当前类型：<span class="font-medium text-slate-900">{{ item.kind === "submission" ? "上传记录" : "反馈记录" }}</span>
                </p>
              </div>
              <div class="space-y-3 text-sm text-slate-500">
                <p class="text-xl font-semibold tracking-tight text-slate-900">{{ item.title }}</p>
                <p><span class="font-medium text-slate-900">提交时间：</span>{{ formatDate(item.createdAt) }}</p>
                <p v-if="item.relativePath" class="break-all">
                  <span class="font-medium text-slate-900">文件位置：</span>{{ item.relativePath }}
                </p>
              </div>
              <div class="space-y-3 text-sm text-slate-500">
                <p v-if="item.kind === 'feedback' && item.description"><span class="font-medium text-slate-900">说明：</span>{{ item.description }}</p>
                <p v-if="item.reviewReason"><span class="font-medium text-slate-900">处理说明：</span>{{ item.reviewReason }}</p>
              </div>
            </div>
          </article>
        </div>

        <div v-if="!receiptRecords.length" class="mt-6">
          <EmptyState title="输入回执码后查看记录" />
        </div>
      </SurfaceCard>
    </div>
  </div>
</template>
