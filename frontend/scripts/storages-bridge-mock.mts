/**
 * Desktop-bridge mock for the storages live harness. Implements only the
 * storage surface plus the handful of generic calls StoragesView touches.
 * State is mutable so the harness can flip it at runtime via
 * window.__storageMock.
 */
import type { SaveStorageRequest, Storage, StorageEntry, StorageListResult } from "../src/lib/types";

interface MockState {
  storages: Storage[];
  listings: Record<string, StorageEntry[]>;
  registered: SaveStorageRequest[];
  lastTestRequest: SaveStorageRequest | null;
  testResult: "connected" | "error";
}

const now = new Date().toISOString();
const hourAgo = new Date(Date.now() - 3600_000).toISOString();

const state: MockState = {
  storages: [
    {
      id: "stg-1",
      name: "Company Backups",
      driver: "s3",
      endpoint: "s3.eu-central-1.amazonaws.com",
      region: "eu-central-1",
      bucket: "neuropipe-backups",
      accessKey: "AKIAIOSFODNN7EXAMPLE",
      secretRef: "stgsec:11111111-1111-1111-1111-111111111111",
      secure: true,
      publicBaseUrl: "https://cdn.neuropipe.dev",
      status: "connected",
      lastPingAt: hourAgo,
      createdAt: now,
      updatedAt: now,
    },
    {
      id: "stg-2",
      name: "Nightly FTP",
      driver: "ftp",
      host: "ftp.example.com",
      port: 21,
      username: "automation",
      passwordRef: "stgpw:22222222-2222-2222-2222-222222222222",
      tlsMode: "explicit",
      baseDir: "uploads",
      status: "unverified",
      createdAt: now,
      updatedAt: now,
    },
  ],
  listings: {
    "": [
      { name: "reports", path: "reports", isDir: true, size: 0, modTime: hourAgo },
      { name: "photos", path: "photos", isDir: true, size: 0, modTime: hourAgo },
      { name: "uploads", path: "uploads", isDir: true, size: 0, modTime: now },
      { name: "pipeline.json", path: "pipeline.json", isDir: false, size: 2480, modTime: hourAgo },
      { name: "chart.png", path: "chart.png", isDir: false, size: 151_730, modTime: hourAgo },
      { name: "readme.md", path: "readme.md", isDir: false, size: 1_124, modTime: now },
    ],
    reports: [
      { name: "2026", path: "reports/2026", isDir: true, size: 0, modTime: hourAgo },
      { name: "2025", path: "reports/2025", isDir: true, size: 0, modTime: hourAgo },
      { name: "summary.csv", path: "reports/summary.csv", isDir: false, size: 18_442, modTime: hourAgo },
    ],
  },
  registered: [],
  lastTestRequest: null,
  testResult: "connected",
};

const delay = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

const entriesFor = (storageId: string, path: string): StorageEntry[] => {
  if (storageId !== "stg-1") return [];
  return state.listings[path] ?? [];
};

export const desktop = {
  listStorages: async () => {
    await delay(30);
    return state.storages;
  },
  pingStorage: async (id: string) => {
    await delay(200);
    const item = state.storages.find((s) => s.id === id);
    if (item) item.status = "connected";
    return "connected" as const;
  },
  testStorage: async (request: SaveStorageRequest) => {
    state.lastTestRequest = request;
    await delay(250);
    return state.testResult;
  },
  registerStorage: async (request: SaveStorageRequest) => {
    state.registered.push(request);
    const created: Storage = {
      id: `stg-${state.storages.length + 1}`,
      name: request.name,
      driver: request.driver,
      endpoint: request.endpoint,
      region: request.region,
      bucket: request.bucket,
      accessKey: request.accessKey,
      secure: request.secure,
      host: request.host,
      port: request.port,
      username: request.username,
      tlsMode: request.tlsMode,
      baseDir: request.baseDir,
      publicBaseUrl: request.publicBaseUrl,
      status: "connected",
      createdAt: now,
      updatedAt: now,
    };
    state.storages.push(created);
    return created;
  },
  updateStorage: async (request: SaveStorageRequest) => {
    const item = state.storages.find((s) => s.id === request.id);
    if (item) item.name = request.name;
    return item ?? state.storages[0];
  },
  deleteStorage: async (id: string) => {
    state.storages = state.storages.filter((s) => s.id !== id);
  },
  storageListFiles: async (id: string, path: string): Promise<StorageListResult> => {
    await delay(80);
    return { path, entries: entriesFor(id, path) };
  },
  storageUploadFile: async (id: string, localPath: string, remotePath: string) => {
    await delay(150);
    const fileName = localPath.split(/[\\/]/).pop() ?? "upload.bin";
    const target = remotePath.endsWith("/") ? remotePath + fileName : remotePath;
    const dir = target.includes("/") ? target.slice(0, target.lastIndexOf("/")) : "";
    state.listings[dir] = [
      ...(state.listings[dir] ?? []),
      { name: fileName, path: target, isDir: false, size: 42_000, modTime: new Date().toISOString() },
    ];
    return { key: target, size: 42_000, driver: "s3" };
  },
  storageDownloadFile: async (id: string, remotePath: string, localPath: string) => ({
    path: localPath,
    name: remotePath.split("/").pop() ?? "file",
    bytes: 1024,
  }),
  storageDeleteEntry: async (id: string, path: string) => {
    await delay(120);
    for (const [dir, list] of Object.entries(state.listings)) {
      state.listings[dir] = list.filter((e) => e.path !== path);
    }
    return { deleted: true, count: 1 };
  },
  storageMakeDir: async (id: string, path: string) => {
    await delay(120);
    const [dir, name] = path.includes("/") ? [path.slice(0, path.lastIndexOf("/")), path.slice(path.lastIndexOf("/") + 1)] : ["", path];
    state.listings[dir] = [
      ...(state.listings[dir] ?? []),
      { name, path, isDir: true, size: 0, modTime: new Date().toISOString() },
    ];
    return { path, created: true };
  },
  storageMoveEntry: async (id: string, from: string, to: string) => {
    await delay(120);
    for (const [dir, list] of Object.entries(state.listings)) {
      const entry = list.find((e) => e.path === from);
      if (entry) {
        entry.name = to.split("/").pop() ?? to;
        entry.path = to;
      }
    }
    return { from, to, moved: true };
  },
  chooseStorageUploadFile: async () => "C:\\Users\\demo\\Downloads\\export.csv",
  chooseStorageSaveFile: async () => "C:\\Users\\demo\\Downloads\\save.csv",
};

/* Expose mock state for the harness driver. */
declare global {
  interface Window {
    __storageMock: {
      state: MockState;
      setListing: (path: string, entries: StorageEntry[]) => void;
    };
  }
}

window.__storageMock = {
  state,
  setListing: (path, entries) => {
    state.listings[path] = entries;
  },
};

export function wailsUnavailable(): Error {
  return new Error("wails unavailable");
}
