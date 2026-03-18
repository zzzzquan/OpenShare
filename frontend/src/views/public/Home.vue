<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  ChevronLeft,
  ChevronRight,
  Clock3,
  Download,
  FileArchive,
  FileAudio,
  FileCode2,
  FileImage,
  FileSpreadsheet,
  FileText,
  FileType2,
  FileVideo,
  Flag,
  Folder,
  Home,
  LayoutGrid,
  List,
  Upload,
} from "lucide-vue-next";

import InfoPanelCard, { type InfoPanelCardItem } from "../../components/InfoPanelCard.vue";
import SearchSection from "../../components/SearchSection.vue";
import { HttpError, httpClient } from "../../lib/http/client";
import { readApiError } from "../../lib/http/helpers";
import { ensureSessionReceiptCode, readStoredReceiptCode } from "../../lib/receiptCode";
import { hasAdminPermission } from "../../lib/admin/session";
import { renderSimpleMarkdown } from "../../lib/markdown";
import { collectDroppedEntries, normalizeFiles, type UploadSelectionEntry } from "../../lib/uploads/fileDrop";

interface AnnouncementItem {
  id: string;
  title: string;
  content: string;
  is_pinned: boolean;
  creator: {
    id: string;
    username: string;
    avatar_url: string;
    role: string;
  };
  published_at?: string;
  updated_at: string;
}

interface PublicFolderItem {
  id: string;
  name: string;
  updated_at: string;
  file_count: number;
  download_count: number;
  total_size: number;
}

interface PublicFileItem {
  id: string;
  title: string;
  original_name: string;
  description: string;
  uploaded_at: string;
  download_count: number;
  size: number;
}

interface HotDownloadItem {
  id: string;
  name: string;
  downloadCount: number;
}

interface LatestItem {
  id: string;
  name: string;
}

interface SidebarDetailItem {
  id: string;
  label: string;
  meta?: string;
}

interface SidebarDetailModalState {
  eyebrow: string;
  title: string;
  description: string;
  items: SidebarDetailItem[];
}

interface SearchResultResponse {
  items: Array<{
    entity_type: "file" | "folder";
    id: string;
    name: string;
    original_name?: string;
    extension?: string;
    size?: number;
    download_count?: number;
    uploaded_at?: string;
  }>;
  page: number;
  page_size: number;
  total: number;
}

interface FolderDetailResponse {
  id: string;
  name: string;
  description: string;
  parent_id: string | null;
  file_count: number;
  download_count: number;
  total_size: number;
  updated_at: string;
  breadcrumbs: Array<{
    id: string;
    name: string;
  }>;
}

const route = useRoute();
const router = useRouter();

const announcements = ref<AnnouncementItem[]>([]);
const announcementDetail = ref<AnnouncementItem | null>(null);
const announcementListOpen = ref(false);
const hotDownloadItems = ref<HotDownloadItem[]>([]);
const latestItems = ref<LatestItem[]>([]);
const sidebarDetailModal = ref<SidebarDetailModalState | null>(null);
const viewMode = ref<"cards" | "table">("cards");
const sortMode = ref<"name" | "download" | "format">("name");
const sortMenuOpen = ref(false);
const viewMenuOpen = ref(false);
const transientWarning = ref("");
const transientWarningTimer = ref<number | null>(null);
const downloadTimestamps = ref<number[]>([]);
const transientWarningLeaving = ref(false);
const uploadModalOpen = ref(false);
const uploadSubmitting = ref(false);
const uploadMessage = ref("");
const uploadError = ref("");
const uploadFileInput = ref<HTMLInputElement | null>(null);
const currentReceiptCode = ref("");
const uploadForm = ref({
  description: "",
  entries: [] as UploadSelectionEntry[],
});
const uploadDropActive = ref(false);
const uploadCollecting = ref(false);
const feedbackModalOpen = ref(false);
const feedbackTarget = ref<{ id: string; type: "file" | "folder"; name: string } | null>(null);
const feedbackDescription = ref("");
const feedbackSubmitting = ref(false);
const feedbackMessage = ref("");
const feedbackError = ref("");
const feedbackSubmitDisabled = computed(() => feedbackSubmitting.value || !feedbackDescription.value.trim());

const loading = ref(false);
const error = ref("");
const actionMessage = ref("");
const actionError = ref("");
const batchDownloadSubmitting = ref(false);
const folders = ref<PublicFolderItem[]>([]);
const files = ref<PublicFileItem[]>([]);
const searchKeyword = ref("");
const searchLoading = ref(false);
const searchError = ref("");
const searchRows = ref<DirectoryRow[]>([]);
const breadcrumbs = ref<Array<{ id: string; name: string }>>([]);
const currentFolderDetail = ref<FolderDetailResponse | null>(null);
const selectedResourceKeys = ref<string[]>([]);
const canManageResourceDescriptions = ref(false);
const folderDescriptionEditorOpen = ref(false);
const folderNameDraft = ref("");
const folderDescriptionDraft = ref("");
const folderDescriptionSaving = ref(false);
const folderDescriptionError = ref("");
const deleteResourceTarget = ref<{ id: string; kind: "folder"; name: string } | null>(null);
const deleteResourcePassword = ref("");
const deleteResourceSubmitting = ref(false);
const deleteResourceError = ref("");
const currentFolderID = computed(() => {
  const raw = route.query.folder;
  return typeof raw === "string" && raw.trim() ? raw.trim() : "";
});
const rootViewLocked = computed(() => route.query.root === "1");
const hotDownloads = computed(() => hotDownloadItems.value.slice(0, 5).map((item) => ({
  id: item.id,
  label: item.name,
})));
const latestTitles = computed(() => latestItems.value.slice(0, 5).map((item) => ({
  id: item.id,
  label: item.name,
})));
const recentAnnouncements = computed(() => announcements.value.slice(0, 5).map((item) => ({
  id: item.id,
  label: item.title,
  badge: item.is_pinned ? "置顶" : undefined,
})));

type DirectoryRow = {
  id: string;
  kind: "folder" | "file";
  name: string;
  extension: string;
  description: string;
  downloadCount: number;
  fileCount: number;
  sizeText: string;
  updatedAt: string;
  downloadURL: string;
};

const rows = computed<DirectoryRow[]>(() => [
  ...folders.value.map((folder) => ({
    id: folder.id,
    kind: "folder" as const,
    name: folder.name,
    extension: "",
    description: "",
    downloadCount: folder.download_count ?? 0,
    fileCount: folder.file_count ?? 0,
    sizeText: formatSize(folder.total_size ?? 0),
    updatedAt: formatDateTime(folder.updated_at),
    downloadURL: `/api/public/folders/${encodeURIComponent(folder.id)}/download`,
  })),
  ...(currentFolderID.value
    ? files.value.map((file) => ({
        id: file.id,
        kind: "file" as const,
        name: file.original_name || file.title,
        extension: extractExtension(file.original_name),
        description: (file.description ?? "").trim(),
        downloadCount: file.download_count ?? 0,
        fileCount: 0,
        sizeText: formatSize(file.size),
        updatedAt: formatDateTime(file.uploaded_at),
        downloadURL: `/api/public/files/${encodeURIComponent(file.id)}/download`,
      }))
    : []),
]);
const displayedRows = computed<DirectoryRow[]>(() => (searchKeyword.value ? searchRows.value : rows.value));

const sortedRows = computed(() => {
  const next = [...displayedRows.value];
  next.sort((left, right) => compareRows(left, right, sortMode.value));
  return next;
});
const selectedRows = computed(() => sortedRows.value.filter((row) => selectedResourceKeys.value.includes(selectionKey(row))));
const hasSelectedRows = computed(() => selectedRows.value.length > 0);
const allVisibleRowsSelected = computed(() => sortedRows.value.length > 0 && selectedRows.value.length === sortedRows.value.length);
const currentFolderDescriptionHTML = computed(() => renderSimpleMarkdown(currentFolderDetail.value?.description ?? ""));
const folderEditorDirty = computed(() => {
  if (!currentFolderDetail.value) {
    return false;
  }

  return (
    folderNameDraft.value.trim() !== currentFolderDetail.value.name ||
    folderDescriptionDraft.value.trim() !== (currentFolderDetail.value.description ?? "")
  );
});
const currentFolderStats = computed(() => {
  if (!currentFolderDetail.value) {
    return [];
  }

  return [
    { label: "文件夹名", value: currentFolderDetail.value.name },
    { label: "下载量", value: String(currentFolderDetail.value.download_count ?? 0) },
    { label: "文件数", value: `${currentFolderDetail.value.file_count ?? 0} 个文件` },
    { label: "文件夹大小", value: formatSize(currentFolderDetail.value.total_size ?? 0) },
    { label: "更新时间", value: formatDateTime(currentFolderDetail.value.updated_at) },
  ];
});

const canGoUp = computed(() => currentFolderID.value.length > 0);

function downloadResource(row: DirectoryRow) {
  actionMessage.value = "";
  actionError.value = "";
  if (!allowDownloadRequest()) {
    showTransientWarning("下载请求过于频繁，请稍后再试。");
    return;
  }

  const link = document.createElement("a");
  link.href = row.downloadURL;
  link.rel = "noopener";
  document.body.appendChild(link);
  link.click();
  link.remove();

  applyDownloadCountUpdate(row);
  if (row.kind === "folder") {
    void loadHotDownloads();
  }
}

function selectionKey(row: DirectoryRow) {
  return `${row.kind}:${row.id}`;
}

function isRowSelected(row: DirectoryRow) {
  return selectedResourceKeys.value.includes(selectionKey(row));
}

function toggleRowSelection(row: DirectoryRow) {
  const key = selectionKey(row);
  if (selectedResourceKeys.value.includes(key)) {
    selectedResourceKeys.value = selectedResourceKeys.value.filter((item) => item !== key);
    return;
  }
  selectedResourceKeys.value = [...selectedResourceKeys.value, key];
}

function clearSelection() {
  selectedResourceKeys.value = [];
}

function selectAllVisibleRows() {
  selectedResourceKeys.value = sortedRows.value.map((row) => selectionKey(row));
}

function toggleSelectAllVisibleRows() {
  if (allVisibleRowsSelected.value) {
    clearSelection();
    return;
  }
  selectAllVisibleRows();
}

async function downloadSelectedResources() {
  if (!hasSelectedRows.value || batchDownloadSubmitting.value) {
    return;
  }

  actionMessage.value = "";
  actionError.value = "";
  if (!allowDownloadRequest()) {
    showTransientWarning("下载请求过于频繁，请稍后再试。");
    return;
  }

  const fileIDs = selectedRows.value.filter((row) => row.kind === "file").map((row) => row.id);
  const folderIDs = selectedRows.value.filter((row) => row.kind === "folder").map((row) => row.id);

  batchDownloadSubmitting.value = true;
  try {
    const response = await fetch("/api/public/resources/batch-download", {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/zip",
      },
      body: JSON.stringify({
        file_ids: fileIDs,
        folder_ids: folderIDs,
      }),
    });

    if (!response.ok) {
      throw new Error("batch download failed");
    }

    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "openshare-selection.zip";
    document.body.appendChild(link);
    link.click();
    link.remove();
    window.URL.revokeObjectURL(url);

    for (const row of selectedRows.value) {
      applyDownloadCountUpdate(row);
    }
    await loadHotDownloads();
    clearSelection();
  } catch (err: unknown) {
    actionError.value = readApiError(err, "批量下载失败。");
  } finally {
    batchDownloadSubmitting.value = false;
  }
}

function syncBodyScrollLock() {
  const shouldLock = Boolean(
    announcementDetail.value
      || announcementListOpen.value
      || sidebarDetailModal.value
      || uploadModalOpen.value
      || feedbackModalOpen.value
      || folderDescriptionEditorOpen.value
      || deleteResourceTarget.value,
  );
  document.body.style.overflow = shouldLock ? "hidden" : "";
}

onMounted(async () => {
  const storedViewMode = window.localStorage.getItem("public-home-view-mode");
  if (storedViewMode === "cards" || storedViewMode === "table") {
    viewMode.value = storedViewMode;
  }
  const storedSortMode = window.localStorage.getItem("public-home-sort-mode");
  if (storedSortMode === "name" || storedSortMode === "download" || storedSortMode === "format") {
    sortMode.value = storedSortMode;
  }
  currentReceiptCode.value = await syncSessionReceiptCode();
  await Promise.all([loadAnnouncements(), loadHotDownloads(), loadLatestTitles(), loadDirectory(), loadAdminPermission()]);
});

onBeforeUnmount(() => {
  if (transientWarningTimer.value !== null) {
    window.clearTimeout(transientWarningTimer.value);
  }
  document.body.style.overflow = "";
});

watch(currentFolderID, () => {
  clearSearchState();
  void loadDirectory();
});

async function loadAnnouncements() {
  try {
    const response = await httpClient.get<{ items: AnnouncementItem[] }>("/public/announcements");
    announcements.value = response.items ?? [];
  } catch {
    announcements.value = [];
  }
}

function openAnnouncementDetail(item: InfoPanelCardItem) {
  const target = announcements.value.find((entry) => entry.id === item.id);
  if (!target) {
    return;
  }
  announcementListOpen.value = false;
  announcementDetail.value = target;
  syncBodyScrollLock();
}

function closeAnnouncementDetail() {
  announcementDetail.value = null;
  syncBodyScrollLock();
}

function returnToAnnouncementList() {
  announcementDetail.value = null;
  announcementListOpen.value = true;
  syncBodyScrollLock();
}

function openAnnouncementList() {
  announcementListOpen.value = true;
  syncBodyScrollLock();
}

function closeAnnouncementList() {
  announcementListOpen.value = false;
  syncBodyScrollLock();
}

function announcementAuthorName(item: AnnouncementItem) {
  return item.creator?.username?.trim() || "未知用户";
}

function announcementAuthorInitial(item: AnnouncementItem) {
  return announcementAuthorName(item).slice(0, 1).toUpperCase() || "A";
}

function announcementAuthorIsSuperAdmin(item: AnnouncementItem) {
  return item.creator?.role === "super_admin";
}

function openSidebarDetailModal(modal: SidebarDetailModalState) {
  sidebarDetailModal.value = modal;
  syncBodyScrollLock();
}

function closeSidebarDetailModal() {
  sidebarDetailModal.value = null;
  syncBodyScrollLock();
}

function openSidebarDetailItem(item: InfoPanelCardItem) {
  sidebarDetailModal.value = null;
  syncBodyScrollLock();
  openFile(item.id);
}

function openHotDownloadsModal() {
  openSidebarDetailModal({
    eyebrow: "Hot Downloads",
    title: "热门下载",
    description: "展示当前下载量最高的前 20 份资料，点击标题可跳转文件详情页。",
    items: hotDownloadItems.value.map((item) => ({
      id: item.id,
      label: item.name,
      meta: `${item.downloadCount} 次下载`,
    })),
  });
}

function openLatestItemsModal() {
  openSidebarDetailModal({
    eyebrow: "Latest Files",
    title: "资料上新",
    description: "展示最新发布的前 20 份资料，点击标题可跳转文件详情页。",
    items: latestItems.value.map((item) => ({
      id: item.id,
      label: item.name,
    })),
  });
}

async function loadHotDownloads() {
  try {
    const response = await httpClient.get<{ items: PublicFileItem[] }>("/public/files?sort=download_count_desc&page=1&page_size=20");
    hotDownloadItems.value = (response.items ?? []).map((item) => ({
      id: item.id,
      name: item.original_name || item.title,
      downloadCount: item.download_count ?? 0,
    }));
  } catch {
    hotDownloadItems.value = [];
  }
}

async function loadLatestTitles() {
  try {
    const response = await httpClient.get<{ items: PublicFileItem[] }>("/public/files?sort=created_at_desc&page=1&page_size=20");
    latestItems.value = (response.items ?? []).map((item) => ({
      id: item.id,
      name: item.original_name || item.title,
    }));
  } catch {
    latestItems.value = [];
  }
}

async function loadDirectory() {
  loading.value = true;
  error.value = "";
  actionMessage.value = "";
  actionError.value = "";
  try {
    const folderParams = new URLSearchParams({
      page: "1",
      page_size: "100",
      sort: "title_asc",
    });
    if (currentFolderID.value) {
      folderParams.set("folder_id", currentFolderID.value);
    }

    const directoryParams = new URLSearchParams();
    if (currentFolderID.value) {
      directoryParams.set("parent_id", currentFolderID.value);
    }

    const requests: Array<Promise<unknown>> = [
      httpClient.get<{ items: PublicFolderItem[] }>(`/public/folders${directoryParams.toString() ? `?${directoryParams.toString()}` : ""}`),
      httpClient.get<{ items: PublicFileItem[] }>(`/public/files?${folderParams.toString()}`),
    ];

    if (currentFolderID.value) {
      requests.push(httpClient.get<FolderDetailResponse>(`/public/folders/${encodeURIComponent(currentFolderID.value)}`));
    }

    const [folderResponse, fileResponse, folderDetail] = await Promise.all(requests);
    folders.value = (folderResponse as { items: PublicFolderItem[] }).items ?? [];
    files.value = (fileResponse as { items: PublicFileItem[] }).items ?? [];

    if (!currentFolderID.value && !rootViewLocked.value && folders.value.length === 1) {
      void router.replace({ name: "public-home", query: { folder: folders.value[0].id } });
      return;
    }

    if (folderDetail) {
      const detail = folderDetail as FolderDetailResponse;
      currentFolderDetail.value = detail;
      folderNameDraft.value = detail.name;
      folderDescriptionDraft.value = detail.description ?? "";
      breadcrumbs.value = detail.breadcrumbs ?? [];
    } else {
      currentFolderDetail.value = null;
      folderNameDraft.value = "";
      folderDescriptionDraft.value = "";
      breadcrumbs.value = [];
    }
  } catch (err: unknown) {
    folders.value = [];
    files.value = [];
    breadcrumbs.value = [];
    currentFolderDetail.value = null;
    folderNameDraft.value = "";
    folderDescriptionDraft.value = "";
    if (err instanceof HttpError && err.status === 404) {
      error.value = "目录不存在或未公开。";
    } else {
      error.value = "加载目录失败。";
    }
  } finally {
    loading.value = false;
  }
}

async function loadAdminPermission() {
  canManageResourceDescriptions.value = await hasAdminPermission("resource_moderation");
}

function openRoot() {
  clearSearchState();
  void router.push({ name: "public-home", query: { root: "1" } });
}

function goUpOneLevel() {
  if (!currentFolderID.value) {
    return;
  }
  clearSearchState();
  const parent = breadcrumbs.value.at(-2);
  if (parent) {
    void router.push({ name: "public-home", query: { folder: parent.id } });
    return;
  }
  openRoot();
}

function openFolder(folderID: string) {
  clearSearchState();
  void router.push({ name: "public-home", query: { folder: folderID } });
}

function openFile(fileID: string) {
  void router.push({ name: "public-file-detail", params: { fileID } });
}

function downloadCurrentFolder() {
  if (!currentFolderDetail.value) {
    return;
  }
  downloadResource({
    id: currentFolderDetail.value.id,
    kind: "folder",
    name: currentFolderDetail.value.name,
    extension: "",
    description: "",
    downloadCount: currentFolderDetail.value.download_count ?? 0,
    fileCount: currentFolderDetail.value.file_count ?? 0,
    sizeText: formatSize(currentFolderDetail.value.total_size ?? 0),
    updatedAt: formatDateTime(currentFolderDetail.value.updated_at),
    downloadURL: `/api/public/folders/${encodeURIComponent(currentFolderDetail.value.id)}/download`,
  });
}

function openDeleteFolderDialog() {
  if (!currentFolderDetail.value) {
    return;
  }
  deleteResourceTarget.value = {
    id: currentFolderDetail.value.id,
    kind: "folder",
    name: currentFolderDetail.value.name,
  };
  deleteResourcePassword.value = "";
  deleteResourceError.value = "";
}

function closeDeleteResourceDialog() {
  deleteResourceTarget.value = null;
  deleteResourcePassword.value = "";
  deleteResourceError.value = "";
  deleteResourceSubmitting.value = false;
}

async function confirmDeleteResource() {
  if (!deleteResourceTarget.value) {
    return;
  }
  if (!deleteResourcePassword.value.trim()) {
    deleteResourceError.value = "请输入当前管理员密码。";
    return;
  }

  deleteResourceSubmitting.value = true;
  deleteResourceError.value = "";
  try {
    await httpClient.request(`/admin/resources/folders/${encodeURIComponent(deleteResourceTarget.value.id)}`, {
      method: "DELETE",
      body: { password: deleteResourcePassword.value },
    });
    const parentID = currentFolderDetail.value?.parent_id ?? "";
    closeDeleteResourceDialog();
    actionMessage.value = `文件夹 ${currentFolderDetail.value?.name ?? ""} 已删除。`;
    clearSearchState();
    if (parentID) {
      await router.push({ name: "public-home", query: { folder: parentID } });
    } else {
      await router.push({ name: "public-home", query: { root: "1" } });
    }
  } catch (err: unknown) {
    deleteResourceError.value = readApiError(err, "删除文件夹失败。");
  } finally {
    deleteResourceSubmitting.value = false;
  }
}

async function runSearch(keyword: string) {
  searchKeyword.value = keyword;
  searchLoading.value = true;
  searchError.value = "";
  try {
    const query = new URLSearchParams({
      q: keyword,
      page: "1",
      page_size: "50",
    });
    if (currentFolderID.value) {
      query.set("folder_id", currentFolderID.value);
    }
    const response = await httpClient.get<SearchResultResponse>(`/public/search?${query.toString()}`);
    searchRows.value = response.items.map((item) => ({
      id: item.id,
      kind: item.entity_type,
      name: item.entity_type === "file" ? (item.original_name || item.name) : item.name,
      extension: item.entity_type === "file" ? (item.extension || extractExtension(item.original_name || item.name)) : "",
      description: "",
      downloadCount: item.download_count ?? 0,
      fileCount: 0,
      sizeText: item.entity_type === "file" ? formatSize(item.size ?? 0) : "-",
      updatedAt: item.uploaded_at ? formatDateTime(item.uploaded_at) : "-",
      downloadURL: item.entity_type === "file"
        ? `/api/public/files/${encodeURIComponent(item.id)}/download`
        : `/api/public/folders/${encodeURIComponent(item.id)}/download`,
    }));
  } catch (err: unknown) {
    searchRows.value = [];
    searchError.value = readApiError(err, "搜索失败。");
  } finally {
    searchLoading.value = false;
  }
}

function clearSearchState() {
  searchKeyword.value = "";
  searchLoading.value = false;
  searchError.value = "";
  searchRows.value = [];
  clearSelection();
}

function openUpload() {
  uploadModalOpen.value = true;
  uploadError.value = "";
  uploadMessage.value = "";
  uploadForm.value.description = "";
  uploadForm.value.entries = [];
  void syncSessionReceiptCode();
  if (uploadFileInput.value) {
    uploadFileInput.value.value = "";
  }
  syncBodyScrollLock();
}

function closeUploadModal() {
  uploadModalOpen.value = false;
  syncBodyScrollLock();
}

function onUploadFileChange(event: Event) {
  const target = event.target as HTMLInputElement;
  uploadForm.value.entries = normalizeFiles(Array.from(target.files ?? []).slice(0, 1));
  if (uploadForm.value.entries.length === 0 && (target.files?.length ?? 0) > 0) {
    uploadError.value = "已自动忽略 .DS_Store，请重新选择可上传文件。";
  }
}

function triggerUploadFileSelect() {
  uploadFileInput.value?.click();
}

function clearUploadEntries() {
  uploadForm.value.entries = [];
  if (uploadFileInput.value) {
    uploadFileInput.value.value = "";
  }
}

function onUploadDragEnter() {
  uploadDropActive.value = true;
}

function onUploadDragLeave(event: DragEvent) {
  const currentTarget = event.currentTarget as HTMLElement | null;
  if (currentTarget && event.relatedTarget instanceof Node && currentTarget.contains(event.relatedTarget)) {
    return;
  }
  uploadDropActive.value = false;
}

async function onUploadDrop(event: DragEvent) {
  event.preventDefault();
  uploadDropActive.value = false;
  uploadCollecting.value = true;
  uploadError.value = "";
  try {
    const entries = await collectDroppedEntries(event);
    uploadForm.value.entries = entries;
    if (entries.length === 0 && (event.dataTransfer?.files.length ?? 0) > 0) {
      uploadError.value = "检测到的内容仅包含 .DS_Store，已自动忽略。";
    }
  } catch {
    uploadError.value = "解析拖拽内容失败，请重试。";
  } finally {
    uploadCollecting.value = false;
  }
}

async function submitUpload() {
  if (uploadForm.value.entries.length === 0) {
    uploadError.value = "请选择文件，或直接拖入多文件/文件夹。";
    return;
  }

  uploadSubmitting.value = true;
  uploadError.value = "";
  uploadMessage.value = "";
  try {
    const formData = new FormData();
    formData.set("folder_id", currentFolderID.value);
    formData.set("description", uploadForm.value.description.trim());
    formData.set("manifest", JSON.stringify(uploadForm.value.entries.map((entry) => ({
      relative_path: entry.relativePath,
    }))));
    uploadForm.value.entries.forEach((entry) => {
      formData.append("files", entry.file, entry.file.name);
    });
    const response = await httpClient.post<{ receipt_code: string; item_count: number; status: string }>("/public/submissions", formData);
    uploadMessage.value = response.status === "approved"
      ? `已上传 ${response.item_count} 个文件，请保存回执码 ${response.receipt_code}。`
      : `已提交 ${response.item_count} 个文件进入审核，请保存回执码 ${response.receipt_code}。`;
    window.sessionStorage.setItem("openshare_receipt_code", response.receipt_code);
    currentReceiptCode.value = response.receipt_code;
    uploadForm.value.description = "";
    clearUploadEntries();
    if (response.status === "approved") {
      await loadDirectory();
    }
  } catch (err) {
    if (err instanceof HttpError && err.status === 400) {
      uploadError.value = "上传参数无效。";
    } else {
      uploadError.value = "提交上传失败。";
    }
  } finally {
    uploadSubmitting.value = false;
  }
}

function applyDownloadCountUpdate(row: DirectoryRow) {
  if (row.kind === "file") {
    let nextDownloadCount = 0;
    files.value = files.value.map((item) => {
      if (item.id !== row.id) {
        return item;
      }
      nextDownloadCount = item.download_count + 1;
      return {
        ...item,
        download_count: nextDownloadCount,
      };
    });
    if (nextDownloadCount > 0) {
      syncHotDownloads(row.id, row.name, nextDownloadCount);
    }
    return;
  }

  folders.value = folders.value.map((item) => {
    if (item.id !== row.id) {
      return item;
    }
    return {
      ...item,
      download_count: item.download_count + Math.max(1, item.file_count),
    };
  });
}

function syncHotDownloads(fileID: string, fileName: string, downloadCount: number) {
  const next = [...hotDownloadItems.value];
  const index = next.findIndex((item) => item.id === fileID);
  if (index >= 0) {
    next[index] = {
      ...next[index],
      name: fileName,
      downloadCount,
    };
  } else {
    next.push({
      id: fileID,
      name: fileName,
      downloadCount,
    });
  }

  next.sort((left, right) => {
    if (right.downloadCount !== left.downloadCount) {
      return right.downloadCount - left.downloadCount;
    }
    return left.name.localeCompare(right.name, "zh-CN");
  });
  hotDownloadItems.value = next.slice(0, 20);
}

function allowDownloadRequest() {
  const now = Date.now();
  const windowMs = 10_000;
  const limit = 10;
  downloadTimestamps.value = downloadTimestamps.value.filter((timestamp) => now - timestamp < windowMs);
  if (downloadTimestamps.value.length >= limit) {
    return false;
  }
  downloadTimestamps.value.push(now);
  return true;
}

function showTransientWarning(message: string) {
  transientWarning.value = message;
  transientWarningLeaving.value = false;
  if (transientWarningTimer.value !== null) {
    window.clearTimeout(transientWarningTimer.value);
  }
  transientWarningTimer.value = window.setTimeout(() => {
    transientWarningLeaving.value = true;
    transientWarningTimer.value = window.setTimeout(() => {
      transientWarning.value = "";
      transientWarningLeaving.value = false;
      transientWarningTimer.value = null;
    }, 1200);
  }, 400);
}

function setViewMode(mode: "cards" | "table") {
  viewMode.value = mode;
  viewMenuOpen.value = false;
  window.localStorage.setItem("public-home-view-mode", mode);
}

watch(sortedRows, (rows) => {
  const allowedKeys = new Set(rows.map((row) => selectionKey(row)));
  selectedResourceKeys.value = selectedResourceKeys.value.filter((key) => allowedKeys.has(key));
}, { immediate: true });

function setSortMode(mode: "name" | "download" | "format") {
  sortMode.value = mode;
  sortMenuOpen.value = false;
  window.localStorage.setItem("public-home-sort-mode", mode);
}

function sortModeLabel(mode: "name" | "download" | "format") {
  switch (mode) {
    case "download":
      return "下载量排序";
    case "format":
      return "格式排序";
    default:
      return "名称排序";
  }
}

function viewModeLabel(mode: "cards" | "table") {
  return mode === "cards" ? "卡片" : "表格";
}

function openFeedbackModal(target: { id: string; type: "file" | "folder"; name: string }) {
  feedbackModalOpen.value = true;
  feedbackTarget.value = target;
  feedbackDescription.value = "";
  feedbackMessage.value = "";
  feedbackError.value = "";
  void syncSessionReceiptCode();
  syncBodyScrollLock();
}

function closeFeedbackModal() {
  feedbackModalOpen.value = false;
  feedbackTarget.value = null;
  syncBodyScrollLock();
}

function openFolderDescriptionEditor() {
  folderNameDraft.value = currentFolderDetail.value?.name ?? "";
  folderDescriptionDraft.value = currentFolderDetail.value?.description ?? "";
  folderDescriptionError.value = "";
  folderDescriptionEditorOpen.value = true;
  syncBodyScrollLock();
}

function closeFolderDescriptionEditor() {
  folderDescriptionEditorOpen.value = false;
  folderDescriptionSaving.value = false;
  folderDescriptionError.value = "";
  folderNameDraft.value = currentFolderDetail.value?.name ?? "";
  folderDescriptionDraft.value = currentFolderDetail.value?.description ?? "";
  syncBodyScrollLock();
}

async function saveFolderDescription() {
  if (!currentFolderDetail.value || !folderEditorDirty.value) {
    return;
  }

  folderDescriptionSaving.value = true;
  folderDescriptionError.value = "";
  try {
    await httpClient.request(`/admin/resources/folders/${encodeURIComponent(currentFolderDetail.value.id)}`, {
      method: "PUT",
      body: {
        name: folderNameDraft.value.trim(),
        description: folderDescriptionDraft.value.trim(),
      },
    });
    currentFolderDetail.value = {
      ...currentFolderDetail.value,
      name: folderNameDraft.value.trim(),
      description: folderDescriptionDraft.value.trim(),
    };
    breadcrumbs.value = breadcrumbs.value.map((item, index) => (
      index === breadcrumbs.value.length - 1
        ? { ...item, name: folderNameDraft.value.trim() }
        : item
    ));
    folderDescriptionEditorOpen.value = false;
    syncBodyScrollLock();
  } catch (err: unknown) {
    folderDescriptionError.value = readApiError(err, "更新文件夹简介失败。");
  } finally {
    folderDescriptionSaving.value = false;
  }
}

async function submitFeedback() {
  if (!feedbackTarget.value) {
    return;
  }
  if (!feedbackDescription.value.trim()) {
    feedbackError.value = "请填写问题说明。";
    return;
  }

  feedbackSubmitting.value = true;
  feedbackMessage.value = "";
  feedbackError.value = "";
  try {
    const response = await httpClient.post<{ receipt_code: string }>("/public/reports", {
      file_id: feedbackTarget.value.type === "file" ? feedbackTarget.value.id : "",
      folder_id: feedbackTarget.value.type === "folder" ? feedbackTarget.value.id : "",
      reason: "content_error",
      description: feedbackDescription.value.trim(),
    });
    feedbackMessage.value = `反馈已提交，请保存回执码 ${response.receipt_code}。`;
    window.sessionStorage.setItem("openshare_receipt_code", response.receipt_code);
    currentReceiptCode.value = response.receipt_code;
  } catch (err: unknown) {
    if (err instanceof HttpError && err.status === 400) {
      feedbackError.value = "反馈原因无效。";
    } else if (err instanceof HttpError && err.status === 404) {
      feedbackError.value = "文件不存在或已下线。";
    } else {
      feedbackError.value = "提交反馈失败。";
    }
  } finally {
    feedbackSubmitting.value = false;
  }
}

function formatSize(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(2)} KB`;
  if (size < 1024 * 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(2)} MB`;
  return `${(size / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function formatDateTime(value: string) {
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(new Date(value));
}

function extractExtension(name: string) {
  const index = name.lastIndexOf(".");
  if (index <= 0 || index === name.length - 1) {
    return "";
  }
  return name.slice(index + 1).toLowerCase();
}

function fileIconComponent(extension: string) {
  const ext = extension.toLowerCase();
  if (["png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico"].includes(ext)) return FileImage;
  if (["mp4", "mov", "avi", "mkv", "webm"].includes(ext)) return FileVideo;
  if (["mp3", "wav", "flac", "aac", "m4a", "ogg"].includes(ext)) return FileAudio;
  if (["zip", "rar", "7z", "tar", "gz", "bz2", "xz"].includes(ext)) return FileArchive;
  if (["xls", "xlsx", "csv", "numbers"].includes(ext)) return FileSpreadsheet;
  if (["js", "ts", "jsx", "tsx", "json", "html", "css", "go", "py", "java", "c", "cpp", "h", "hpp", "rs", "sh", "yaml", "yml", "toml", "xml"].includes(ext)) return FileCode2;
  if (["pdf", "doc", "docx", "ppt", "pptx", "txt", "md", "rtf"].includes(ext)) return FileText;
  return FileType2;
}

function compareRows(left: DirectoryRow, right: DirectoryRow, mode: "name" | "download" | "format") {
  if (mode === "download") {
    if (right.downloadCount !== left.downloadCount) {
      return right.downloadCount - left.downloadCount;
    }
    return left.name.localeCompare(right.name, "zh-CN");
  }

  if (mode === "format") {
    const leftRank = formatSortRank(left);
    const rightRank = formatSortRank(right);
    if (leftRank !== rightRank) {
      return leftRank - rightRank;
    }
    return left.name.localeCompare(right.name, "zh-CN");
  }

  return left.name.localeCompare(right.name, "zh-CN");
}

function formatSortRank(row: DirectoryRow) {
  if (row.kind === "folder") {
    return 0;
  }

  const extension = row.extension.toLowerCase();
  if (extension === "pdf") {
    return 1;
  }
  if (["doc", "docx", "xls", "xlsx", "ppt", "pptx"].includes(extension)) {
    return 2;
  }
  return 3;
}

async function syncSessionReceiptCode() {
  try {
    const receiptCode = await ensureSessionReceiptCode();
    currentReceiptCode.value = receiptCode || readStoredReceiptCode();
    return currentReceiptCode.value;
  } catch {
    currentReceiptCode.value = readStoredReceiptCode();
    return currentReceiptCode.value;
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="transientWarning" class="fixed inset-0 z-[130] flex items-center justify-center px-4">
      <div
        class="rounded-2xl border border-rose-200 bg-white px-4 py-3 text-sm text-rose-700 shadow-lg shadow-rose-100/70"
        :class="transientWarningLeaving ? 'animate-[warning-fade-out_1.2s_ease_forwards]' : 'animate-[warning-fade-in_0.18s_ease-out_forwards]'"
      >
        {{ transientWarning }}
      </div>
    </div>
  </Teleport>

  <main class="app-container py-8 lg:py-10">
    <div class="grid gap-6 xl:grid-cols-[248px_minmax(0,1fr)]">
      <aside class="space-y-4 xl:pt-2">
        <InfoPanelCard
          title="公告栏"
          :items="recentAnnouncements"
          clickable
          action-label="详情"
          empty-text="暂无公告"
          @select="openAnnouncementDetail"
          @action="openAnnouncementList"
        />
        <InfoPanelCard
          title="热门下载"
          :items="hotDownloads"
          clickable
          action-label="详情"
          empty-text="暂无下载数据"
          @select="openSidebarDetailItem"
          @action="openHotDownloadsModal"
        />
        <InfoPanelCard
          title="资料上新"
          :items="latestTitles"
          clickable
          action-label="详情"
          empty-text="暂无最新资料"
          @select="openSidebarDetailItem"
          @action="openLatestItemsModal"
        />
      </aside>

      <section class="min-w-0">
        <div class="panel overflow-hidden">
          <div class="border-b border-slate-200 px-5 py-4 sm:px-6 dark:border-slate-800">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div class="flex flex-wrap items-center gap-2 text-sm text-slate-500 dark:text-slate-400">
                <button type="button" class="inline-flex items-center gap-2 rounded-full px-2 py-1 transition hover:bg-slate-100 hover:text-slate-900" @click="openRoot">
                  <Home class="h-4 w-4" />
                  <span>主页</span>
                </button>
                <template v-for="item in breadcrumbs" :key="item.id">
                  <ChevronRight class="h-4 w-4 text-slate-300" />
                  <button
                    type="button"
                    class="rounded-full px-2 py-1 transition hover:bg-slate-100 hover:text-slate-900"
                    @click="openFolder(item.id)"
                  >
                    {{ item.name }}
                  </button>
                </template>
              </div>

            </div>
          </div>

          <div>
            <SearchSection embedded :loading="searchLoading" @search="runSearch" @clear="clearSearchState" />
          </div>

          <p v-if="searchError" class="mx-5 mt-4 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 sm:mx-6">
            {{ searchError }}
          </p>
          <div
            v-else-if="searchKeyword"
            class="mx-5 mt-4 rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600 sm:mx-6"
          >
            当前搜索：<span class="font-medium text-slate-900">{{ searchKeyword }}</span>
            <span class="ml-2">共 {{ searchRows.length }} 条结果</span>
          </div>

          <div class="px-5 pb-5 sm:px-6">
            <div class="flex flex-wrap items-center gap-3 border-t border-slate-100 pt-4">
              <button
                type="button"
                class="inline-flex items-center gap-2 rounded-xl border border-slate-200 px-3 py-2 text-sm font-medium text-slate-600 transition hover:border-slate-300 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-45"
                :disabled="!canGoUp"
                @click="goUpOneLevel"
              >
                <ChevronLeft class="h-4 w-4" />
                返回上一级
              </button>

              <button
                type="button"
                class="inline-flex items-center gap-2 rounded-xl border border-slate-200 px-3 py-2 text-sm font-medium text-slate-600 transition hover:border-slate-300 hover:text-slate-900"
                @click="openUpload"
              >
                <Upload class="h-4 w-4" />
                在该目录上传
              </button>

              <button
                v-if="sortedRows.length > 0"
                type="button"
                class="inline-flex items-center gap-2 rounded-xl border border-slate-200 px-3 py-2 text-sm font-medium text-slate-600 transition hover:border-slate-300 hover:text-slate-900"
                @click="toggleSelectAllVisibleRows"
              >
                {{ allVisibleRowsSelected ? "取消全选" : "全选" }}
              </button>

              <div class="ml-auto flex flex-wrap items-center gap-3">
              <div class="relative">
                <button
                  type="button"
                  class="inline-flex items-center gap-2 rounded-xl border border-slate-200 px-3 py-2 text-sm font-medium text-slate-600 transition hover:border-slate-300 hover:text-slate-900"
                  @click="sortMenuOpen = !sortMenuOpen; viewMenuOpen = false"
                >
                  {{ sortModeLabel(sortMode) }}
                  <ChevronRight class="h-4 w-4 rotate-90" />
                </button>
                <div v-if="sortMenuOpen" class="absolute left-0 top-full z-20 mt-2 min-w-[156px] rounded-2xl border border-slate-200 bg-white p-1 shadow-lg">
                  <button
                    type="button"
                    class="block w-full rounded-xl px-3 py-2 text-left text-sm transition"
                    :class="sortMode === 'download' ? 'bg-slate-100 font-medium text-slate-900' : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900'"
                    @click="setSortMode('download')"
                  >
                    下载量排序
                  </button>
                  <button
                    type="button"
                    class="block w-full rounded-xl px-3 py-2 text-left text-sm transition"
                    :class="sortMode === 'name' ? 'bg-slate-100 font-medium text-slate-900' : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900'"
                    @click="setSortMode('name')"
                  >
                    名称排序
                  </button>
                  <button
                    type="button"
                    class="block w-full rounded-xl px-3 py-2 text-left text-sm transition"
                    :class="sortMode === 'format' ? 'bg-slate-100 font-medium text-slate-900' : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900'"
                    @click="setSortMode('format')"
                  >
                    格式排序
                  </button>
                </div>
              </div>

              <div class="relative">
                <button
                  type="button"
                  class="inline-flex items-center gap-2 rounded-xl border border-slate-200 px-3 py-2 text-sm font-medium text-slate-600 transition hover:border-slate-300 hover:text-slate-900"
                  @click="viewMenuOpen = !viewMenuOpen; sortMenuOpen = false"
                >
                  <LayoutGrid v-if="viewMode === 'cards'" class="h-4 w-4" />
                  <List v-else class="h-4 w-4" />
                  {{ viewModeLabel(viewMode) }}
                  <ChevronRight class="h-4 w-4 rotate-90" />
                </button>
                <div v-if="viewMenuOpen" class="absolute left-0 top-full z-20 mt-2 min-w-[124px] rounded-2xl border border-slate-200 bg-white p-1 shadow-lg">
                  <button
                    type="button"
                    class="flex w-full items-center gap-2 rounded-xl px-3 py-2 text-left text-sm transition"
                    :class="viewMode === 'cards' ? 'bg-slate-100 font-medium text-slate-900' : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900'"
                    @click="setViewMode('cards')"
                  >
                    <LayoutGrid class="h-4 w-4" />
                    卡片
                  </button>
                  <button
                    type="button"
                    class="flex w-full items-center gap-2 rounded-xl px-3 py-2 text-left text-sm transition"
                    :class="viewMode === 'table' ? 'bg-slate-100 font-medium text-slate-900' : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900'"
                    @click="setViewMode('table')"
                  >
                    <List class="h-4 w-4" />
                    表格
                  </button>
                </div>
              </div>
              </div>
            </div>
          </div>

          <p v-if="actionMessage" class="mx-5 mt-5 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700 sm:mx-6">{{ actionMessage }}</p>
          <p v-if="actionError" class="mx-5 mt-5 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 sm:mx-6">{{ actionError }}</p>

          <div v-if="loading" class="px-5 py-8 text-sm text-slate-500 sm:px-6">加载中…</div>
          <div v-else-if="error" class="px-5 py-8 text-sm text-rose-600 sm:px-6">{{ error }}</div>
          <div v-else-if="sortedRows.length === 0" class="px-5 py-8 text-sm text-slate-500 sm:px-6">
            {{ searchKeyword ? "没有找到匹配结果。" : "当前目录为空。" }}
          </div>
          <div v-else-if="viewMode === 'cards'" class="grid gap-4 px-5 py-5 sm:grid-cols-2 sm:px-6 2xl:grid-cols-3">
            <article
              v-for="row in sortedRows"
              :key="`${row.kind}-${row.id}`"
              class="group relative flex h-[168px] cursor-pointer flex-col rounded-3xl border border-slate-200 bg-white px-5 pt-3.5 transition hover:border-slate-300 hover:shadow-sm"
              @click="row.kind === 'folder' ? openFolder(row.id) : openFile(row.id)"
            >
              <div class="absolute right-5 top-4 z-10">
                <input
                  :checked="isRowSelected(row)"
                  type="checkbox"
                  class="h-5 w-5 rounded-lg border-slate-300 text-slate-900 focus:ring-slate-300"
                  @click.stop
                  @change="toggleRowSelection(row)"
                />
              </div>
              <div class="flex items-start gap-4">
                <div class="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-slate-100 text-slate-500">
                  <Folder v-if="row.kind === 'folder'" class="h-7 w-7 text-blue-500" />
                  <component v-else :is="fileIconComponent(row.extension)" class="h-7 w-7" />
                </div>
                <div class="min-w-0 flex-1 pr-10 pt-0.5">
                  <h3 class="truncate text-base font-semibold leading-6 text-slate-900">{{ row.name }}</h3>
                  <p v-if="row.kind === 'file' && row.description" class="mt-1 line-clamp-1 text-sm leading-5 text-slate-500">
                    {{ row.description }}
                  </p>
                </div>
              </div>

              <div class="mt-3 flex flex-wrap items-center gap-x-5 gap-y-1 text-xs text-slate-500">
                <template v-if="row.kind === 'file'">
                  <span class="inline-flex items-center gap-1.5">
                    <Download class="h-3.5 w-3.5" />
                    {{ row.downloadCount }}
                  </span>
                  <span>{{ row.sizeText }}</span>
                </template>
                <template v-else>
                  <span class="inline-flex items-center gap-1.5">
                    <Download class="h-3.5 w-3.5" />
                    {{ row.downloadCount }}
                  </span>
                  <span>{{ row.fileCount }} 个文件</span>
                  <span>{{ row.sizeText }}</span>
                </template>
                <span class="inline-flex items-center gap-1.5">
                  <Clock3 class="h-3.5 w-3.5" />
                  {{ row.updatedAt }}
                </span>
              </div>

              <div class="mt-auto flex items-center justify-between border-t border-slate-100 py-2.5">
                <button
                  type="button"
                  class="inline-flex items-center gap-1.5 rounded-xl border border-slate-200 px-3.5 py-1.5 text-sm font-medium text-slate-500 transition hover:border-slate-300 hover:text-slate-900"
                  @click.stop="openFeedbackModal({ id: row.id, type: row.kind, name: row.name })"
                >
                  <Flag class="h-4 w-4" />
                  反馈
                </button>
                <button
                  type="button"
                  class="inline-flex items-center justify-center rounded-xl bg-slate-900 p-2.5 text-white transition hover:bg-slate-800"
                  @click.stop="downloadResource(row)"
                >
                  <Download class="h-4 w-4" />
                </button>
              </div>
            </article>
          </div>
          <div v-else class="px-5 py-5 sm:px-6">
            <table class="data-table">
              <thead>
                <tr>
                  <th class="w-10"></th>
                  <th>名称</th>
                  <th class="text-right">大小</th>
                  <th class="text-right">修改时间</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="row in sortedRows"
                  :key="`${row.kind}-${row.id}`"
                  class="cursor-pointer transition hover:bg-slate-50 dark:hover:bg-slate-800/40"
                  @click="row.kind === 'folder' ? openFolder(row.id) : openFile(row.id)"
                >
                  <td @click.stop>
                    <input
                      :checked="isRowSelected(row)"
                      type="checkbox"
                      class="h-5 w-5 rounded-lg border-slate-300 text-slate-900 focus:ring-slate-300"
                      @change="toggleRowSelection(row)"
                    />
                  </td>
                  <td>
                    <div
                      v-if="row.kind === 'folder'"
                      class="flex min-w-0 items-center gap-3 text-left"
                    >
                      <Folder class="h-5 w-5 shrink-0 text-blue-500" />
                      <span class="truncate text-slate-900 dark:text-slate-100">{{ row.name }}</span>
                    </div>
                    <div
                      v-else
                      class="flex min-w-0 items-center gap-3 text-left"
                    >
                      <component :is="fileIconComponent(row.extension)" class="h-5 w-5 shrink-0 text-slate-500" />
                      <span class="truncate text-slate-900 dark:text-slate-100">{{ row.name }}</span>
                    </div>
                  </td>
                  <td class="text-right">{{ row.sizeText }}</td>
                  <td class="text-right">{{ row.updatedAt }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-if="currentFolderDetail" class="border-t border-slate-200 px-5 py-5 sm:px-6">
            <section>
              <div class="flex items-start justify-between gap-4">
                <div class="min-w-0 flex-1 space-y-3">
                  <p class="text-xs font-semibold uppercase tracking-[0.18em] text-blue-600">Folder Info</p>
                  <div class="flex flex-wrap items-center gap-x-8 gap-y-3 text-sm text-slate-500">
                    <div
                      v-for="item in currentFolderStats"
                      :key="item.label"
                      class="inline-flex items-center gap-2"
                    >
                      <span>{{ item.label }}</span>
                      <span class="font-medium text-slate-900">{{ item.value }}</span>
                    </div>
                  </div>
                </div>
                <div class="flex shrink-0 items-start gap-3">
                  <button
                    v-if="canManageResourceDescriptions"
                    type="button"
                    class="btn-secondary"
                    @click="openFolderDescriptionEditor"
                  >
                    编辑
                  </button>
                  <button
                    v-if="canManageResourceDescriptions"
                    type="button"
                    class="btn-secondary text-rose-600 hover:border-rose-200 hover:bg-rose-50 hover:text-rose-700"
                    @click="openDeleteFolderDialog"
                  >
                    删除
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-11 w-11 items-center justify-center rounded-xl border border-slate-200 text-slate-500 transition hover:border-slate-300 hover:text-slate-900"
                    aria-label="反馈文件夹"
                    @click="openFeedbackModal({ id: currentFolderDetail.id, type: 'folder', name: currentFolderDetail.name })"
                  >
                    <Flag class="h-4 w-4" />
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-11 w-11 items-center justify-center rounded-xl bg-slate-900 text-white transition hover:bg-slate-800"
                    aria-label="下载文件夹"
                    @click="downloadCurrentFolder"
                  >
                    <Download class="h-4 w-4" />
                  </button>
                </div>
              </div>

              <div class="mt-4 rounded-3xl border border-slate-200 bg-white px-5 py-5">
                <div
                  v-if="currentFolderDescriptionHTML"
                  class="markdown-content"
                  v-html="currentFolderDescriptionHTML"
                />
                <p v-else class="text-sm text-slate-400">该文件夹暂无简介orz</p>
              </div>
            </section>
          </div>

        </div>
      </section>
    </div>
  </main>

  <Teleport to="body">
    <Transition
      enter-active-class="transition duration-300 ease-out"
      enter-from-class="translate-y-6 opacity-0"
      enter-to-class="translate-y-0 opacity-100"
      leave-active-class="transition duration-200 ease-in"
      leave-from-class="translate-y-0 opacity-100"
      leave-to-class="translate-y-4 opacity-0"
    >
      <div
        v-if="hasSelectedRows"
        class="pointer-events-none fixed inset-x-0 bottom-6 z-[130] flex justify-center px-4"
      >
        <div class="pointer-events-auto flex w-full max-w-3xl items-center justify-between gap-4 rounded-3xl border border-slate-200 bg-white px-6 py-4 shadow-[0_0_0_1px_rgba(15,23,42,0.06),0_22px_60px_-18px_rgba(15,23,42,0.34)]">
          <p class="text-sm text-slate-600">
            已选 <span class="font-semibold text-slate-900">{{ selectedRows.length }}</span> 项
          </p>
          <div class="flex items-center gap-3">
            <button type="button" class="btn-secondary" @click="clearSelection">取消选择</button>
            <button type="button" class="btn-primary" :disabled="batchDownloadSubmitting" @click="downloadSelectedResources">
              {{ batchDownloadSubmitting ? "打包中…" : "批量下载" }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>

  <Teleport to="body">
    <Transition name="modal-shell">
    <div v-if="sidebarDetailModal" class="fixed inset-0 z-[120] flex items-center justify-center bg-slate-950/30 px-4">
      <div class="modal-card panel w-full max-w-3xl p-6">
        <div class="flex items-start justify-between gap-4 border-b border-slate-200 pb-4">
          <div>
            <p class="text-xs font-semibold uppercase tracking-[0.18em] text-blue-600">{{ sidebarDetailModal.eyebrow }}</p>
            <h3 class="mt-2 text-2xl font-semibold tracking-tight text-slate-900">{{ sidebarDetailModal.title }}</h3>
            <p class="mt-2 text-sm text-slate-500">{{ sidebarDetailModal.description }}</p>
          </div>
          <button type="button" class="btn-secondary" @click="closeSidebarDetailModal">关闭</button>
        </div>
        <div class="mt-5 max-h-[70vh] overflow-y-auto pr-1">
          <div v-if="sidebarDetailModal.items.length === 0" class="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-5 text-sm text-slate-500">
            暂无数据
          </div>
          <div v-else class="space-y-3">
            <button
              v-for="(item, index) in sidebarDetailModal.items"
              :key="item.id"
              type="button"
              class="flex w-full items-center gap-4 rounded-2xl border border-slate-200 px-4 py-3 text-left transition hover:border-slate-300 hover:bg-slate-50"
              @click="openSidebarDetailItem({ id: item.id, label: item.label })"
            >
              <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-slate-100 text-sm font-semibold text-slate-600">
                {{ index + 1 }}
              </span>
              <div class="min-w-0 flex-1">
                <p class="truncate text-sm font-medium text-slate-900">{{ item.label }}</p>
              </div>
              <span v-if="item.meta" class="shrink-0 text-sm text-slate-500">{{ item.meta }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
    </Transition>
  </Teleport>

  <Teleport to="body">
    <Transition name="modal-shell">
    <div v-if="announcementListOpen" class="fixed inset-0 z-[120] flex items-center justify-center bg-slate-950/30 px-4">
      <div class="modal-card panel w-full max-w-3xl p-6">
        <div class="flex items-start justify-between gap-4 border-b border-slate-200 pb-4">
          <div class="min-w-0">
            <p class="text-xs font-semibold uppercase tracking-[0.18em] text-blue-600">Announcements</p>
            <h3 class="mt-2 text-2xl font-semibold tracking-tight text-slate-900">全部公告</h3>
          </div>
          <button type="button" class="btn-secondary" @click="closeAnnouncementList">关闭</button>
        </div>
        <div class="mt-5 max-h-[70vh] space-y-3 overflow-auto pr-1">
          <button
            v-for="item in announcements"
            :key="item.id"
            type="button"
            class="flex w-full items-start justify-between gap-4 rounded-2xl border border-slate-200 bg-white px-4 py-4 text-left transition hover:border-blue-200 hover:bg-blue-50/40"
            @click="openAnnouncementDetail({ id: item.id, label: item.title })"
          >
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <span
                  v-if="item.is_pinned"
                  class="rounded-md bg-[#dcecff] px-2 py-0.5 text-xs font-semibold text-[#4f8ff7]"
                >
                  置顶
                </span>
                <p class="text-base font-semibold text-slate-900">{{ item.title }}</p>
              </div>
              <div class="mt-3 flex flex-wrap items-center gap-2">
                <div class="flex h-8 w-8 items-center justify-center overflow-hidden rounded-full bg-slate-100 text-xs font-semibold text-slate-600">
                  <img v-if="item.creator?.avatar_url" :src="item.creator.avatar_url" alt="发布人头像" class="h-full w-full object-cover" />
                  <span v-else>{{ announcementAuthorInitial(item) }}</span>
                </div>
                <span class="text-sm font-medium text-slate-700">{{ announcementAuthorName(item) }}</span>
                <span
                  v-if="announcementAuthorIsSuperAdmin(item)"
                  class="rounded-full bg-[#fff1e4] px-2.5 py-1 text-xs font-semibold text-[#d07a2d]"
                >
                  超级管理员
                </span>
              </div>
              <p class="mt-2 line-clamp-2 text-sm text-slate-500">{{ item.content }}</p>
            </div>
            <span class="shrink-0 text-sm text-slate-400">
              {{ formatDateTime(item.published_at || item.updated_at) }}
            </span>
          </button>
          <p v-if="announcements.length === 0" class="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-6 text-center text-sm text-slate-500">
            暂无公告
          </p>
        </div>
      </div>
    </div>
    </Transition>
  </Teleport>

  <Teleport to="body">
    <Transition name="modal-shell">
    <div v-if="announcementDetail" class="fixed inset-0 z-[120] flex items-center justify-center bg-slate-950/30 px-4">
      <div class="modal-card panel w-full max-w-2xl p-6">
        <div class="flex items-start justify-between gap-4 border-b border-slate-200 pb-4">
          <div class="min-w-0">
            <p class="text-xs font-semibold uppercase tracking-[0.18em] text-blue-600">Announcement</p>
            <h3 class="mt-2 text-2xl font-semibold tracking-tight text-slate-900">{{ announcementDetail.title }}</h3>
            <div class="mt-3 flex flex-wrap items-center gap-3 text-sm text-slate-500">
              <div class="flex items-center gap-2">
                <div class="flex h-8 w-8 items-center justify-center overflow-hidden rounded-full bg-slate-100 text-xs font-semibold text-slate-600">
                  <img v-if="announcementDetail.creator?.avatar_url" :src="announcementDetail.creator.avatar_url" alt="发布人头像" class="h-full w-full object-cover" />
                  <span v-else>{{ announcementAuthorInitial(announcementDetail) }}</span>
                </div>
                <span class="font-medium text-slate-700">{{ announcementAuthorName(announcementDetail) }}</span>
              </div>
              <span
                v-if="announcementAuthorIsSuperAdmin(announcementDetail)"
                class="rounded-full bg-[#fff1e4] px-2.5 py-1 text-xs font-semibold text-[#d07a2d]"
              >
                超级管理员
              </span>
              <span>{{ formatDateTime(announcementDetail.published_at || announcementDetail.updated_at) }}</span>
            </div>
          </div>
          <div class="flex items-center gap-3">
            <button type="button" class="btn-secondary" @click="returnToAnnouncementList">返回</button>
            <button type="button" class="btn-secondary" @click="closeAnnouncementDetail">关闭</button>
          </div>
        </div>
        <div class="mt-5 rounded-3xl border border-slate-200 bg-white px-5 py-5">
          <div class="markdown-content" v-html="renderSimpleMarkdown(announcementDetail.content)" />
        </div>
      </div>
    </div>
    </Transition>
  </Teleport>

  <Teleport to="body">
    <Transition name="modal-shell">
    <div v-if="deleteResourceTarget" class="fixed inset-0 z-[120] flex items-center justify-center bg-slate-950/30 px-4">
      <div class="modal-card w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
        <div>
          <h3 class="text-lg font-semibold text-slate-900">确认删除文件夹</h3>
          <p class="mt-2 text-sm leading-6 text-slate-500">
            删除后会清除该文件夹及其子目录、文件，无法恢复。确认删除
            <span class="font-medium text-slate-900">{{ deleteResourceTarget.name }}</span>
            吗？
          </p>
        </div>
        <div class="mt-6 space-y-4">
          <input v-model="deleteResourcePassword" type="password" class="field" placeholder="输入当前管理员密码确认删除" />
          <p v-if="deleteResourceError" class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
            {{ deleteResourceError }}
          </p>
          <div class="flex justify-end gap-3">
            <button type="button" class="btn-secondary" @click="closeDeleteResourceDialog">取消</button>
            <button
              type="button"
              class="inline-flex h-11 items-center rounded-xl bg-rose-600 px-5 text-sm font-medium text-white transition hover:bg-rose-700"
              :disabled="deleteResourceSubmitting"
              @click="confirmDeleteResource"
            >
              {{ deleteResourceSubmitting ? "删除中…" : "确认删除" }}
            </button>
          </div>
        </div>
      </div>
    </div>
    </Transition>
  </Teleport>

  <Teleport to="body">
    <Transition name="modal-shell">
    <div v-if="uploadModalOpen" class="fixed inset-0 z-[120] overflow-y-auto bg-slate-950/40 backdrop-blur-sm">
      <div class="flex min-h-screen items-start justify-center px-4 py-6">
        <div class="modal-card panel w-full max-w-2xl overflow-hidden">
          <div class="max-h-[calc(100vh-3rem)] overflow-y-auto p-6">
            <div class="flex items-start justify-between gap-4 border-b border-slate-200 pb-4">
              <div>
                <h3 class="text-lg font-semibold text-slate-900">上传资料</h3>
                <p class="mt-1 text-sm text-slate-500">当前目录下直接上传资料，提交后会进入审核池。</p>
              </div>
              <button type="button" class="btn-secondary" @click="closeUploadModal">关闭</button>
            </div>

            <form class="mt-5 space-y-4" @submit.prevent="submitUpload">
            <div class="panel-muted px-4 py-3 text-sm text-slate-600">
              <p class="text-xs text-slate-400">目标目录</p>
              <p class="mt-1 font-medium text-slate-900">{{ breadcrumbs.length ? breadcrumbs.map((item) => item.name).join(" / ") : "主页根目录" }}</p>
            </div>

            <label class="space-y-2">
              <span class="text-sm font-medium text-slate-700">回执码</span>
              <div class="rounded-xl bg-slate-50 px-4 py-3">
                <p class="text-sm font-semibold tracking-[0.12em] text-slate-900">
                  {{ currentReceiptCode || "当前会话回执码暂未同步" }}
                </p>
              </div>
            </label>

            <label class="space-y-2">
              <span class="text-sm font-medium text-slate-700">资料简介</span>
              <textarea
                v-model="uploadForm.description"
                rows="4"
                class="field-area"
                placeholder="可选，简要介绍资料内容和适用场景，支持简单 Markdown 语法"
              />
            </label>

            <div class="space-y-2">
              <div class="flex items-center justify-between gap-3">
                <span class="text-sm font-medium text-slate-700">上传内容</span>
              </div>

              <input ref="uploadFileInput" type="file" class="hidden" @change="onUploadFileChange" />

              <div
                class="rounded-[28px] border-2 border-dashed px-6 py-10 text-center transition"
                :class="uploadDropActive ? 'border-blue-400 bg-blue-50/60' : 'border-slate-200 bg-slate-50/60'"
                @dragenter.prevent="onUploadDragEnter"
                @dragover.prevent="uploadDropActive = true"
                @dragleave="onUploadDragLeave"
                @drop="onUploadDrop"
              >
                <div class="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-white text-slate-300 shadow-sm">
                  <Upload class="h-8 w-8" />
                </div>
                <p class="mt-5 text-lg text-slate-600">
                  拖拽文件或整个文件夹到这里，或
                  <button type="button" class="font-semibold text-blue-600 transition hover:text-blue-700" @click="triggerUploadFileSelect">点击选择</button>
                </p>
                <p class="mt-2 text-sm text-slate-400">拖拽支持多文件和文件夹。</p>
                <p v-if="uploadCollecting" class="mt-4 text-sm text-slate-500">正在解析拖拽内容…</p>
              </div>

              <div class="panel-muted px-4 py-3 text-sm text-slate-600">
                <div class="flex flex-wrap items-center justify-between gap-3">
                  <p>
                    已选择
                    <span class="font-semibold text-slate-900">{{ uploadForm.entries.length }}</span>
                    个文件
                  </p>
                  <button v-if="uploadForm.entries.length > 0" type="button" class="text-sm text-slate-500 transition hover:text-slate-900" @click="clearUploadEntries">
                    清空列表
                  </button>
                </div>
                <div v-if="uploadForm.entries.length > 0" class="mt-3 max-h-48 space-y-2 overflow-auto pr-1">
                  <div
                    v-for="entry in uploadForm.entries"
                    :key="entry.relativePath"
                    class="rounded-xl bg-white px-3 py-2 text-sm text-slate-700"
                  >
                    {{ entry.relativePath }}
                  </div>
                </div>
                <p v-else class="mt-2 text-sm text-slate-400">当前还没有选择任何文件。</p>
              </div>
            </div>

            <p v-if="uploadMessage" class="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
              {{ uploadMessage }}
            </p>
            <p v-if="uploadError" class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
              {{ uploadError }}
            </p>

              <div class="flex justify-end gap-3">
                <button type="button" class="btn-secondary" @click="closeUploadModal">取消</button>
                <button type="submit" class="btn-primary" :disabled="uploadSubmitting || uploadCollecting || uploadForm.entries.length === 0">
                  {{ uploadSubmitting ? "提交中…" : "提交上传" }}
                </button>
              </div>
            </form>
          </div>
        </div>
      </div>
    </div>
    </Transition>
  </Teleport>

  <Teleport to="body">
    <Transition name="modal-shell">
    <div v-if="feedbackModalOpen" class="fixed inset-0 z-[120] bg-slate-950/40 backdrop-blur-sm">
      <div class="flex min-h-screen items-center justify-center px-4 py-6">
        <div class="modal-card panel w-full max-w-2xl overflow-hidden p-6">
          <div class="flex items-start justify-between gap-4 border-b border-slate-200 pb-4">
            <div>
              <h3 class="text-lg font-semibold text-slate-900">反馈中心</h3>
            </div>
            <button type="button" class="btn-secondary" @click="closeFeedbackModal">关闭</button>
          </div>

          <div class="mt-5 space-y-4">
            <div>
              <p v-if="feedbackTarget" class="mt-2 text-sm text-slate-600">当前对象：{{ feedbackTarget.name }}</p>
            </div>

            <label class="space-y-2">
              <span class="text-sm font-medium text-slate-700">回执码</span>
              <div class="rounded-xl bg-slate-50 px-4 py-3">
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

            <p v-if="feedbackMessage" class="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">{{ feedbackMessage }}</p>
            <p v-if="feedbackError" class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{{ feedbackError }}</p>

            <div class="flex justify-end gap-3">
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
    <div v-if="folderDescriptionEditorOpen" class="fixed inset-0 z-[120] bg-slate-950/40 backdrop-blur-sm">
      <div class="flex min-h-screen items-center justify-center px-4 py-6">
          <div class="modal-card panel w-full max-w-3xl overflow-hidden p-6">
            <div class="border-b border-slate-200 pb-4">
                  <div>
                    <h3 class="text-lg font-semibold text-slate-900">编辑文件夹信息</h3>
                  </div>
            </div>

          <div class="mt-5 space-y-4">
            <label class="space-y-2">
              <span class="text-sm font-medium text-slate-700">文件夹名</span>
              <input
                v-model="folderNameDraft"
                class="field"
                :disabled="!canManageResourceDescriptions"
                placeholder="输入文件夹名"
              />
            </label>

            <textarea
              v-model="folderDescriptionDraft"
              rows="10"
              class="field-area"
              placeholder="输入文件夹简介，简介支持简单 Markdown。"
            />

            <p v-if="folderDescriptionError" class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
              {{ folderDescriptionError }}
            </p>

              <div class="flex justify-end gap-3">
                <button type="button" class="btn-secondary" @click="closeFolderDescriptionEditor">取消</button>
                <button type="button" class="btn-primary" :disabled="folderDescriptionSaving || !folderEditorDirty" @click="saveFolderDescription">
                  {{ folderDescriptionSaving ? "保存中…" : "保存更改" }}
                </button>
              </div>
          </div>
        </div>
      </div>
    </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
@keyframes warning-fade-in {
  0% {
    opacity: 0;
    transform: translateY(8px) scale(0.98);
  }

  100% {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes warning-fade-out {
  0% {
    opacity: 1;
    transform: translateY(0);
  }

  100% {
    opacity: 0;
    transform: translateY(-6px);
  }
}
</style>
