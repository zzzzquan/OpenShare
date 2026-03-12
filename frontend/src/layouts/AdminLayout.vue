<template>
  <div class="min-h-screen bg-slate-950 px-4 py-6 text-slate-100 sm:px-6 lg:px-8">
    <div class="mx-auto grid min-h-[calc(100vh-3rem)] max-w-7xl overflow-hidden rounded-[32px] border border-slate-800 bg-slate-900 shadow-panel lg:grid-cols-[240px_1fr]">
      <aside class="border-b border-slate-800 px-6 py-6 lg:border-b-0 lg:border-r">
        <p class="text-xs font-semibold uppercase tracking-[0.3em] text-blue-300">OpenShare Admin</p>
        <h1 class="mt-2 text-2xl font-semibold text-white">管理后台</h1>
        <nav class="mt-8 flex flex-col gap-2 text-sm text-slate-300">
          <RouterLink class="rounded-2xl px-4 py-3 transition hover:bg-slate-800 hover:text-white" to="/admin">控制台</RouterLink>
          <RouterLink class="rounded-2xl px-4 py-3 transition hover:bg-slate-800 hover:text-white" to="/admin/reports">举报管理</RouterLink>
          <RouterLink class="rounded-2xl px-4 py-3 transition hover:bg-slate-800 hover:text-white" to="/">返回用户端</RouterLink>
        </nav>
      </aside>

      <main class="px-6 py-8">
        <RouterView />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { RouterLink, RouterView } from "vue-router";
</script>
