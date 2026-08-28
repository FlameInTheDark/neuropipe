/* Browser entry for the KV grouped-keys live test: mounts the REAL KVBrowser
   (Keys tab) against a stubbed desktop binding that serves a deterministic
   page of keys covering the grouping edge cases (nested namespaces, a key
   that is also a prefix, lone root keys, numeric ordering). Bundled to IIFE
   by render-kv-grouping-live.mts and inlined into a standalone page next to
   the app's compiled CSS. */

import React from "react";
import { createRoot } from "react-dom/client";
import { I18nextProvider } from "react-i18next";
import i18n from "../src/i18n";
import { KVBrowser } from "../src/features/database/KVBrowser";
import { desktop } from "../src/lib/bridge";
import type { Database, KVKey, KVKeyPage, KVKeyValue } from "../src/lib/types";

/* A realistic keyspace: namespaces with children, prefix-is-also-a-key,
   numeric siblings, deep nesting, root-level lone keys. */
const keys: KVKey[] = [
  { name: "user:1", type: "hash", ttl: -1, size: 128 },
  { name: "user:2", type: "hash", ttl: -1, size: 132 },
  { name: "user:10", type: "hash", ttl: -1, size: 140 },
  { name: "user:profile:avatar", type: "string", ttl: 3600, size: 2048 },
  { name: "session:abc", type: "string", ttl: 45, size: 64 },
  { name: "session:def", type: "string", ttl: 120, size: 64 },
  { name: "cache:page:home", type: "string", ttl: 12, size: 4096 },
  { name: "cache:page:about", type: "string", ttl: 8, size: 2048 },
  { name: "queue:jobs", type: "list", ttl: -1, size: 512 },
  { name: "leaders", type: "zset", ttl: -1, size: 96 },
  { name: "counter", type: "string", ttl: -1, size: 8 },
  { name: "test", type: "string", ttl: -1, size: 4 },
  { name: "test:0", type: "string", ttl: -1, size: 4 },
  { name: "test:1", type: "string", ttl: -1, size: 4 },
];

/* Stub the bridge before any component can call it. */
const scans: Array<{ cursor: number; match: string }> = [];
Object.assign(desktop, {
  kvScanKeys: async (_id: string, request: { cursor: number; match?: string }): Promise<KVKeyPage> => {
    scans.push({ cursor: request.cursor, match: request.match ?? "" });
    return { keys: [...keys], nextCursor: 0, totalSeen: keys.length };
  },
  kvKeyValue: async (_id: string, keyName: string): Promise<KVKeyValue> => {
    const found = keys.find((k) => k.name === keyName);
    return { type: found?.type ?? "string", value: `value of ${keyName}`, ttl: found?.ttl ?? -1 };
  },
  kvDeleteKeys: async (): Promise<number> => 1,
  kvSetTTL: async (): Promise<void> => undefined,
});

const database: Database = {
  id: "db-kv-live",
  name: "Live test KV",
  driver: "redis",
  host: "127.0.0.1",
  port: 6379,
  dbIndex: 0,
  status: "connected",
} as Database;

(window as unknown as Record<string, unknown>).__scans = scans;

const el = document.getElementById("root");
if (!el) throw new Error("#root missing");
createRoot(el).render(
  <I18nextProvider i18n={i18n}>
    <div className="flex h-screen flex-col bg-ink-950 p-4 text-ink-100">
      <div className="min-h-0 flex-1">
        <KVBrowser database={database} />
      </div>
    </div>
  </I18nextProvider>,
);
(window as unknown as Record<string, unknown>).__ready = true;
