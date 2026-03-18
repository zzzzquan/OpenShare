<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { Check, Github, UserRound } from "lucide-vue-next";
import { useRouter } from "vue-router";

import { HttpError, httpClient } from "../lib/http/client";
import { useSessionStore } from "../stores/session";

export interface NavbarItem {
  label: string;
  to: string;
}

interface AdminMeResponse {
  admin: {
    id: string;
    username: string;
    display_name: string;
    avatar_url: string;
    role: string;
    status: string;
    permissions: string[];
  };
}

interface AdminDashboardStatsResponse {
  pending_audit_count: number;
}

const props = withDefaults(
  defineProps<{
    items?: NavbarItem[];
    currentPath?: string;
    githubHref?: string;
  }>(),
  {
    items: () => [
      { label: "首页", to: "/" },
      { label: "回执查询", to: "/upload" },
    ],
    currentPath: "/",
    githubHref: "https://github.com/zzzzquan/OpenShare",
  },
);

const router = useRouter();
const sessionStore = useSessionStore();
const panelRef = ref<HTMLElement | null>(null);
const panelOpen = ref(false);
const username = ref("");
const password = ref("");
const loginLoading = ref(false);
const loginSuccess = ref(false);
const loginError = ref("");

const userButtonLabel = computed(() => {
  if (sessionStore.authenticated) {
    return sessionStore.displayName.slice(0, 1).toUpperCase() || "A";
  }
  return "";
});

onMounted(async () => {
  document.addEventListener("pointerdown", onPointerDown);

  try {
    const response = await httpClient.get<AdminMeResponse>("/admin/me");
    applySession(response);
    await loadPendingAuditCount();
  } catch {
    sessionStore.reset();
    sessionStore.setPendingAuditCount(0);
  }
});

onUnmounted(() => {
  document.removeEventListener("pointerdown", onPointerDown);
});

function isActive(path: string) {
  return props.currentPath === path;
}

async function onUserAction() {
  if (sessionStore.authenticated) {
    await router.push("/admin");
    return;
  }

  panelOpen.value = !panelOpen.value;
  loginError.value = "";
}

async function login() {
  loginLoading.value = true;
  loginError.value = "";

  try {
    const response = await httpClient.post<AdminMeResponse>("/admin/session/login", {
      username: username.value,
      password: password.value,
    });
    applySession(response);
    await loadPendingAuditCount();
    password.value = "";
    loginSuccess.value = true;

    window.setTimeout(() => {
      loginSuccess.value = false;
      panelOpen.value = false;
    }, 1100);
  } catch (error: unknown) {
    loginError.value = readApiError(error) ?? "登录失败，请检查账号或密码。";
  } finally {
    loginLoading.value = false;
  }
}

function applySession(response: AdminMeResponse) {
  sessionStore.setAuthenticated(true, response.admin.display_name || response.admin.username, {
    username: response.admin.username,
    adminId: response.admin.id,
    avatarUrl: response.admin.avatar_url,
    role: response.admin.role,
    status: response.admin.status,
    permissions: response.admin.permissions,
  });
}

async function loadPendingAuditCount() {
  try {
    const response = await httpClient.get<AdminDashboardStatsResponse>("/admin/dashboard/stats");
    sessionStore.setPendingAuditCount(response.pending_audit_count ?? 0);
  } catch {
    sessionStore.setPendingAuditCount(0);
  }
}

function readApiError(error: unknown) {
  if (!(error instanceof HttpError) || typeof error.payload !== "object" || error.payload === null) {
    return null;
  }

  const payload = error.payload as Record<string, unknown>;
  return typeof payload.error === "string" ? payload.error : null;
}

function onPointerDown(event: PointerEvent) {
  if (!panelOpen.value || !panelRef.value) {
    return;
  }

  const target = event.target;
  if (target instanceof Node && !panelRef.value.contains(target)) {
    panelOpen.value = false;
    loginError.value = "";
  }
}
</script>

<template>
  <header class="fixed inset-x-0 top-0 z-[60] border-b border-slate-200 bg-white/95 backdrop-blur after:pointer-events-none after:absolute after:bottom-[-1px] after:left-full after:top-0 after:w-screen after:border-b after:border-slate-200 after:bg-white/95 dark:border-slate-800 dark:bg-slate-950/95 dark:after:border-slate-800 dark:after:bg-slate-950/95">
    <div class="app-container grid h-16 grid-cols-[1fr_auto_1fr] items-center gap-4">
      <div class="flex items-center justify-start">
        <RouterLink to="/" class="inline-flex items-center gap-2.5">
          <img src="/favicon.svg" alt="OpenShare" class="h-8 w-8" />
          <span
            class="text-[15px] font-extrabold tracking-tight text-slate-900 dark:text-slate-100"
            style="font-family: 'Roboto Slab', serif"
          >
            OpenShare
          </span>
        </RouterLink>
      </div>

      <nav class="flex items-center justify-center gap-1">
        <RouterLink
          v-for="item in items"
          :key="item.to"
          :to="item.to"
          class="rounded-lg px-4 py-2 text-sm font-medium transition"
          :class="
            isActive(item.to)
              ? 'bg-slate-200/70 text-slate-900 dark:bg-slate-800 dark:text-slate-100'
              : 'text-slate-600 hover:bg-slate-200/60 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-900 dark:hover:text-slate-100'
          "
        >
          {{ item.label }}
        </RouterLink>
      </nav>

      <div ref="panelRef" class="relative flex items-center justify-end gap-2">
        <a
          :href="githubHref"
          target="_blank"
          rel="noreferrer"
          aria-label="Open GitHub"
          class="inline-flex h-9 w-9 items-center justify-center rounded-full bg-black text-white transition hover:bg-neutral-800"
        >
          <Github class="h-[17.2px] w-[17.2px]" />
        </a>

        <div class="relative">
          <button
            type="button"
            aria-label="管理员入口"
            class="inline-flex h-9 w-9 items-center justify-center overflow-hidden rounded-full border border-slate-200 bg-white text-slate-600 transition hover:bg-slate-100 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-300 dark:hover:bg-slate-900 dark:hover:text-slate-100"
            @click="onUserAction"
          >
            <img
              v-if="sessionStore.authenticated && sessionStore.avatarUrl"
              :src="sessionStore.avatarUrl"
              alt="管理员头像"
              class="h-full w-full object-cover"
            />
            <span
              v-else-if="sessionStore.authenticated && userButtonLabel"
              class="text-xs font-semibold leading-none"
            >
              {{ userButtonLabel }}
            </span>
            <UserRound v-else class="h-4.5 w-4.5" />
          </button>
          <span
            v-if="sessionStore.authenticated && sessionStore.pendingAuditCount > 0"
            class="absolute right-[-1px] top-[-1px] h-2.5 w-2.5 rounded-full bg-rose-500 ring-2 ring-white"
          />
        </div>

        <section
          v-if="panelOpen"
          class="absolute right-0 top-[calc(100%+12px)] z-20 w-[320px] rounded-xl border border-slate-200 bg-white p-4 shadow-sm shadow-slate-950/[0.06] dark:border-slate-800 dark:bg-slate-950 dark:shadow-none"
        >
            <div v-if="loginSuccess" class="flex min-h-[184px] flex-col items-center justify-center gap-3 text-center">
              <div class="flex h-12 w-12 items-center justify-center rounded-full bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900">
                <Check class="h-5 w-5 animate-pulse" />
              </div>
              <div>
                <p class="text-sm font-semibold text-slate-900 dark:text-slate-100">登录成功</p>
                <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">再次点击右上角头像进入管理后台。</p>
              </div>
            </div>

            <template v-else>
              <div class="space-y-1">
                <p class="text-sm font-semibold text-slate-900 dark:text-slate-100">管理员登录</p>
                <p class="text-sm text-slate-500 dark:text-slate-400">输入标示ID和密码进入 OpenShare 后台。</p>
              </div>

              <form class="mt-4 space-y-3" @submit.prevent="login">
                <input v-model="username" class="field h-10" placeholder="标示ID" autocomplete="username" />
                <input
                  v-model="password"
                  type="password"
                  class="field h-10"
                  placeholder="密码"
                  autocomplete="current-password"
                />
                <button type="submit" class="btn-primary h-10 w-full" :disabled="loginLoading">
                  {{ loginLoading ? "登录中…" : "登录后台" }}
                </button>
              </form>

              <p
                v-if="loginError"
                class="mt-3 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700 dark:border-rose-900 dark:bg-rose-950/50 dark:text-rose-300"
              >
                {{ loginError }}
              </p>
            </template>
        </section>
      </div>
    </div>
  </header>
</template>
