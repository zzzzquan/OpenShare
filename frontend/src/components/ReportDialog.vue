<script setup lang="ts">
import { ref, watch } from "vue";

import { HttpError, httpClient } from "../lib/http/client";

const props = defineProps<{
  visible: boolean;
  targetType: "file" | "folder";
  targetId: string;
  targetName: string;
}>();

const emit = defineEmits<{
  (e: "update:visible", value: boolean): void;
  (e: "submitted"): void;
}>();

const reasons = [
  { value: "copyright", label: "侵权" },
  { value: "content_error", label: "内容错误" },
  { value: "file_corrupted", label: "文件损坏" },
  { value: "irrelevant", label: "无关资料" },
] as const;

const selectedReason = ref("");
const description = ref("");
const submitting = ref(false);
const successMessage = ref("");
const errorMessage = ref("");

watch(
  () => props.visible,
  (val) => {
    if (val) {
      selectedReason.value = "";
      description.value = "";
      successMessage.value = "";
      errorMessage.value = "";
    }
  },
);

function close() {
  emit("update:visible", false);
}

async function submit() {
  if (!selectedReason.value) {
    errorMessage.value = "请选择举报原因。";
    return;
  }

  submitting.value = true;
  errorMessage.value = "";
  successMessage.value = "";

  try {
    const body: Record<string, string> = {
      reason: selectedReason.value,
      description: description.value.trim(),
    };
    if (props.targetType === "file") {
      body.file_id = props.targetId;
    } else {
      body.folder_id = props.targetId;
    }

    await httpClient.post("/public/reports", body);
    successMessage.value = "举报已提交，管理员会尽快处理。";
    emit("submitted");
  } catch (error: unknown) {
    if (
      error instanceof HttpError &&
      typeof error.payload === "object" &&
      error.payload &&
      "error" in error.payload
    ) {
      errorMessage.value = String(error.payload.error);
    } else {
      errorMessage.value = "提交举报失败，请稍后重试。";
    }
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="visible"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4 backdrop-blur-sm"
        @click.self="close"
      >
        <div class="w-full max-w-md rounded-[24px] border border-slate-200 bg-white p-6 shadow-xl">
          <div class="flex items-start justify-between gap-4">
            <div>
              <h3 class="text-lg font-semibold text-slate-900">举报资料</h3>
              <p class="mt-1 text-sm text-slate-500">
                {{ targetType === "file" ? "文件" : "文件夹" }}：{{ targetName }}
              </p>
            </div>
            <button
              class="rounded-full p-1.5 text-slate-400 transition hover:bg-slate-100 hover:text-slate-600"
              @click="close"
            >
              <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <form class="mt-5 space-y-4" @submit.prevent="submit">
            <fieldset>
              <legend class="mb-2 text-sm font-medium text-slate-700">举报原因</legend>
              <div class="grid grid-cols-2 gap-2">
                <label
                  v-for="reason in reasons"
                  :key="reason.value"
                  class="flex cursor-pointer items-center gap-2 rounded-xl border px-3 py-2.5 text-sm transition"
                  :class="
                    selectedReason === reason.value
                      ? 'border-blue-500 bg-blue-50 text-blue-800'
                      : 'border-slate-200 text-slate-700 hover:bg-slate-50'
                  "
                >
                  <input
                    v-model="selectedReason"
                    type="radio"
                    name="report-reason"
                    :value="reason.value"
                    class="sr-only"
                  />
                  <span
                    class="h-4 w-4 shrink-0 rounded-full border-2"
                    :class="
                      selectedReason === reason.value
                        ? 'border-blue-500 bg-blue-500'
                        : 'border-slate-300'
                    "
                  />
                  {{ reason.label }}
                </label>
              </div>
            </fieldset>

            <label class="block">
              <span class="mb-2 block text-sm font-medium text-slate-700">补充说明（可选）</span>
              <textarea
                v-model="description"
                rows="3"
                maxlength="500"
                placeholder="请描述具体问题..."
                class="w-full rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm outline-none transition focus:border-blue-500 focus:bg-white"
              />
            </label>

            <div class="flex items-center justify-end gap-3">
              <button
                type="button"
                class="rounded-xl border border-slate-200 px-4 py-2.5 text-sm font-medium text-slate-700 transition hover:bg-slate-100"
                @click="close"
              >
                取消
              </button>
              <button
                type="submit"
                class="rounded-xl bg-rose-600 px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-rose-700 disabled:cursor-not-allowed disabled:bg-slate-400"
                :disabled="submitting || !selectedReason"
              >
                {{ submitting ? "提交中..." : "提交举报" }}
              </button>
            </div>
          </form>

          <p
            v-if="successMessage"
            class="mt-4 rounded-xl bg-emerald-50 px-4 py-3 text-sm text-emerald-700"
          >
            {{ successMessage }}
          </p>
          <p
            v-if="errorMessage"
            class="mt-4 rounded-xl bg-rose-50 px-4 py-3 text-sm text-rose-700"
          >
            {{ errorMessage }}
          </p>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
