<script setup lang="ts">
import type { Component } from "vue";
import { Home, Files } from "lucide-vue-next";

export interface AdminSidebarItem {
  label: string;
  to: string;
  icon?: Component;
  disabled?: boolean;
  hasAlert?: boolean;
}

const props = withDefaults(
  defineProps<{
    title?: string;
    subtitle?: string;
    avatarUrl?: string;
    avatarFallback?: string;
    currentPath: string;
    items: AdminSidebarItem[];
    homeTo?: string;
    homeLabel?: string;
    logoutLabel?: string;
  }>(),
  {
    title: "Superadmin",
    subtitle: "内容治理与系统管理",
    avatarUrl: "",
    avatarFallback: "A",
    homeTo: "/",
    homeLabel: "返回首页",
    logoutLabel: "退出登录",
  },
);

function isActive(path: string) {
  return props.currentPath === path || (path !== "/admin" && props.currentPath.startsWith(`${path}/`));
}

const fallbackIcon = Files;

const emit = defineEmits<{
  logout: [];
}>();
</script>

<template>
  <aside class="flex h-auto flex-col border-b border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-950 lg:h-full lg:border-b-0 lg:border-r">
    <div class="border-b border-slate-200 px-4 py-4 dark:border-slate-800 sm:px-5">
      <div class="flex items-center gap-3">
        <div class="flex h-12 w-12 items-center justify-center overflow-hidden rounded-full border border-slate-200 bg-[#fafafa] text-lg font-semibold text-slate-900 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-100">
          <img v-if="avatarUrl" :src="avatarUrl" alt="管理员头像" class="h-full w-full object-cover" />
          <span v-else>{{ avatarFallback }}</span>
        </div>
        <div class="min-w-0">
          <p class="truncate text-lg font-semibold text-slate-900 dark:text-slate-100">{{ title }}</p>
          <p v-if="subtitle" class="truncate text-xs text-slate-500 dark:text-slate-400">{{ subtitle }}</p>
        </div>
      </div>
    </div>

    <nav class="flex-1 overflow-x-auto px-3 py-3 lg:overflow-visible lg:py-4">
      <div class="flex gap-2 lg:block lg:space-y-1">
      <template v-for="item in items" :key="`${item.label}-${item.to}`">
        <RouterLink
          v-if="!item.disabled"
          :to="item.to"
          class="group flex shrink-0 items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition"
          :class="isActive(item.to) ? 'bg-slate-200/70 text-slate-900 dark:bg-slate-800 dark:text-slate-100' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-900 dark:hover:text-slate-100'"
        >
          <span
            class="h-1.5 w-1.5 shrink-0 rounded-full transition"
            :class="isActive(item.to) ? 'bg-slate-900 dark:bg-slate-100' : 'bg-slate-300 group-hover:bg-slate-400 dark:bg-slate-700 dark:group-hover:bg-slate-600'"
          />
          <component :is="item.icon || fallbackIcon" class="h-4 w-4 shrink-0 text-slate-400 dark:text-slate-500" />
          <span>{{ item.label }}</span>
          <span
            v-if="item.hasAlert"
            class="ml-auto h-2.5 w-2.5 rounded-full bg-rose-500"
          />
        </RouterLink>

        <div
          v-else
          class="flex shrink-0 items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-slate-400 dark:text-slate-600"
        >
          <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-slate-200 dark:bg-slate-800" />
          <component :is="item.icon || fallbackIcon" class="h-4 w-4 shrink-0 text-slate-300 dark:text-slate-700" />
          <span>{{ item.label }}</span>
          <span
            v-if="item.hasAlert"
            class="ml-auto h-2.5 w-2.5 rounded-full bg-rose-300 dark:bg-rose-900"
          />
        </div>
      </template>
      </div>
    </nav>

    <div class="border-t border-slate-200 p-3 dark:border-slate-800">
      <div class="flex flex-wrap gap-2 lg:block lg:space-y-1">
        <slot name="footer-actions" />
        <RouterLink
          :to="homeTo"
          class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-slate-600 transition hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-900 dark:hover:text-slate-100"
        >
          <Home class="h-4 w-4 shrink-0" />
          <span>{{ homeLabel }}</span>
        </RouterLink>
        <button
          type="button"
          class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-left text-sm font-medium text-slate-600 transition hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-900 dark:hover:text-slate-100 lg:w-full"
          @click="emit('logout')"
        >
          <span class="flex h-4 w-4 shrink-0 items-center justify-center text-base leading-none">↪</span>
          <span>{{ logoutLabel }}</span>
        </button>
      </div>
    </div>
  </aside>
</template>
