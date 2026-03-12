import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";

import AdminLayout from "@/layouts/AdminLayout.vue";
import PublicLayout from "@/layouts/PublicLayout.vue";
import AdminDashboardView from "@/views/admin/AdminDashboardView.vue";
import AdminReportsView from "@/views/admin/AdminReportsView.vue";
import HomeView from "@/views/public/HomeView.vue";
import SearchView from "@/views/public/SearchView.vue";

const routes: RouteRecordRaw[] = [
  {
    path: "/",
    component: PublicLayout,
    children: [
      {
        path: "",
        name: "public-home",
        component: HomeView,
      },
      {
        path: "search",
        name: "public-search",
        component: SearchView,
      },
    ],
  },
  {
    path: "/admin",
    component: AdminLayout,
    children: [
      {
        path: "",
        name: "admin-dashboard",
        component: AdminDashboardView,
      },
      {
        path: "reports",
        name: "admin-reports",
        component: AdminReportsView,
      },
    ],
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior() {
    return { top: 0 };
  },
});

export default router;
