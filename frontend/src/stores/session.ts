import { defineStore } from "pinia";
import { computed, ref } from "vue";

export const useSessionStore = defineStore("session", () => {
  const authenticated = ref(false);
  const username = ref("");
  const displayName = ref("");
  const adminId = ref("");
  const avatarUrl = ref("");
  const role = ref("");
  const status = ref("");
  const permissions = ref<string[]>([]);
  const pendingAuditCount = ref(0);
  const isSuperAdmin = computed(() => role.value === "super_admin");

  function setAuthenticated(value: boolean, name = "", payload?: {
    username?: string;
    adminId?: string;
    avatarUrl?: string;
    role?: string;
    status?: string;
    permissions?: string[];
  }) {
    authenticated.value = value;
    displayName.value = name;
    username.value = value ? (payload?.username ?? "") : "";
    adminId.value = value ? (payload?.adminId ?? "") : "";
    avatarUrl.value = value ? (payload?.avatarUrl ?? "") : "";
    role.value = value ? (payload?.role ?? "") : "";
    status.value = value ? (payload?.status ?? "") : "";
    permissions.value = value ? [...(payload?.permissions ?? [])] : [];
    pendingAuditCount.value = value ? pendingAuditCount.value : 0;
  }

  function reset() {
    setAuthenticated(false);
  }

  function setPendingAuditCount(count: number) {
    pendingAuditCount.value = Math.max(0, count);
  }

  function hasPermission(permission: string) {
    return isSuperAdmin.value || permissions.value.includes(permission);
  }

  return {
    authenticated,
    username,
    displayName,
    adminId,
    avatarUrl,
    role,
    status,
    permissions,
    pendingAuditCount,
    isSuperAdmin,
    setAuthenticated,
    setPendingAuditCount,
    hasPermission,
    reset,
  };
});
