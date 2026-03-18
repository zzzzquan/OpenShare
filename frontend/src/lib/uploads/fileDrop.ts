export interface UploadSelectionEntry {
  file: File;
  relativePath: string;
}

interface FileSystemEntry {
  isFile: boolean;
  isDirectory: boolean;
  name: string;
  fullPath: string;
}

interface FileSystemFileEntry extends FileSystemEntry {
  isFile: true;
  file: (callback: (file: File) => void, onError?: (error: DOMException) => void) => void;
}

interface FileSystemDirectoryEntry extends FileSystemEntry {
  isDirectory: true;
  createReader: () => {
    readEntries: (
      successCallback: (entries: FileSystemEntry[]) => void,
      errorCallback?: (error: DOMException) => void,
    ) => void;
  };
}

type DataTransferItemWithEntry = DataTransferItem & {
  webkitGetAsEntry?: () => FileSystemEntry | null;
};

export async function collectDroppedEntries(event: DragEvent) {
  const items = Array.from(event.dataTransfer?.items ?? []) as DataTransferItemWithEntry[];
  if (items.some((item) => typeof item.webkitGetAsEntry === "function")) {
    const entries = await Promise.all(items.map((item) => item.webkitGetAsEntry?.()).filter(Boolean).map((entry) => readEntry(entry!)));
    return entries.flat();
  }

  return normalizeFiles(Array.from(event.dataTransfer?.files ?? []));
}

export function normalizeFiles(files: File[]) {
  return files
    .map((file) => ({
      file,
      relativePath: normalizeRelativePath((file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name),
    }))
    .filter((entry) => !isIgnoredUploadPath(entry.relativePath));
}

async function readEntry(entry: FileSystemEntry, parentPath = ""): Promise<UploadSelectionEntry[]> {
  if (entry.isFile) {
    const file = await readFile(entry as FileSystemFileEntry);
    const relativePath = normalizeRelativePath(joinPath(parentPath, file.name));
    if (isIgnoredUploadPath(relativePath)) {
      return [];
    }
    return [{ file, relativePath }];
  }

  if (entry.isDirectory) {
    const directory = entry as FileSystemDirectoryEntry;
    const children = await readDirectoryEntries(directory);
    const nested = await Promise.all(children.map((child) => readEntry(child, joinPath(parentPath, entry.name))));
    return nested.flat();
  }

  return [];
}

function readFile(entry: FileSystemFileEntry) {
  return new Promise<File>((resolve, reject) => {
    entry.file(resolve, reject);
  });
}

async function readDirectoryEntries(entry: FileSystemDirectoryEntry) {
  const reader = entry.createReader();
  const result: FileSystemEntry[] = [];

  while (true) {
    const batch = await new Promise<FileSystemEntry[]>((resolve, reject) => {
      reader.readEntries(resolve, reject);
    });
    if (batch.length === 0) {
      return result;
    }
    result.push(...batch);
  }
}

function joinPath(base: string, name: string) {
  return base ? `${base}/${name}` : name;
}

function normalizeRelativePath(path: string) {
  return path
    .replaceAll("\\", "/")
    .split("/")
    .map((segment) => segment.trim())
    .filter((segment) => segment && segment !== "." && segment !== "..")
    .join("/");
}

function isIgnoredUploadPath(path: string) {
  const normalized = normalizeRelativePath(path);
  const segments = normalized.split("/").filter(Boolean);
  return segments.some((segment) => segment.toLowerCase() === ".ds_store");
}
