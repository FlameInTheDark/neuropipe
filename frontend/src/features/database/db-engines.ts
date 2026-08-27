/**
 * Catalog of supported database engines and how each one connects,
 * restricted to the drivers the Desktop service actually persists
 * (sqlite | postgres | mysql). Keeping this declarative means the
 * connection form renders itself from the engine definition instead of
 * hard-coding fields per engine. All user-visible copy lives in i18n keys.
 */

export type ConnMode = "file" | "server" | "embedded";

export interface EngineField {
  key: string;
  /** i18n key for the field label */
  labelKey: string;
  placeholderKey?: string;
  type?: "text" | "number" | "password";
  optional?: boolean;
  /** only show this field for the given connection mode */
  mode?: ConnMode;
  /** sensible starting value */
  default?: string;
}

export interface DbEngine {
  id: "sqlite" | "postgres" | "mysql" | "duckdb" | "redis" | "sugardb";
  name: string;
  icon: string;
  /** i18n key of the short blurb shown in the picker */
  blurbKey: string;
  /** which connection modes this engine supports */
  modes: ConnMode[];
  /** default TCP port, used to prefill server forms */
  defaultPort?: number;
}

export const DB_ENGINES: DbEngine[] = [
  {
    id: "sqlite",
    name: "SQLite",
    icon: "HardDrive",
    blurbKey: "dbnew.blurbSqlite",
    modes: ["file"],
  },  {
    id: "duckdb",
    name: "DuckDB",
    icon: "Database",
    blurbKey: "dbnew.blurbDuckdb",
    modes: ["file"],
  },
  {
    id: "postgres",
    name: "PostgreSQL",
    icon: "Database",
    blurbKey: "dbnew.blurbPostgres",
    modes: ["server"],
    defaultPort: 5432,
  },
  {
    id: "mysql",
    name: "MySQL",
    icon: "Database",
    blurbKey: "dbnew.blurbMysql",
    modes: ["server"],
    defaultPort: 3306,
  },
  {
    id: "redis",
    name: "Redis / Valkey / KeyDB",
    icon: "Database",
    blurbKey: "dbnew.blurbRedis",
    modes: ["server"],
    defaultPort: 6379,
  },
  {
    id: "sugardb",
    name: "SugarDB",
    icon: "Database",
    blurbKey: "dbnew.blurbSugardb",
    modes: ["embedded"],
  },
];

export function engineById(id: string): DbEngine {
  return DB_ENGINES.find((e) => e.id === id) ?? DB_ENGINES[0];
}

/** Shared connection fields for any server-mode engine (i18n label keys). */
export function serverFields(engine: DbEngine): EngineField[] {
  return [
    { key: "host", labelKey: "databases.host", placeholderKey: "dbnew.hostPlaceholder", default: "localhost", mode: "server" },
    { key: "port", labelKey: "databases.port", type: "number", default: String(engine.defaultPort ?? ""), mode: "server" },
    { key: "database", labelKey: "databases.database", placeholderKey: "dbnew.databasePlaceholder", mode: "server" },
    { key: "username", labelKey: "databases.username", placeholderKey: "dbnew.userPlaceholder", mode: "server" },
    { key: "password", labelKey: "databases.password", type: "password", optional: true, mode: "server" },
  ];
}

/** Postgres-only extras appended after the shared server fields. */
export function postgresExtras(): EngineField[] {
  return [
    { key: "schema", labelKey: "databases.schema", default: "public", mode: "server" },
    { key: "sslmode", labelKey: "databases.sslMode", default: "prefer", mode: "server" },
  ];
}

/** File-mode field for engines backed by a path. */
export function fileField(): EngineField {
  return { key: "path", labelKey: "databases.path", placeholderKey: "databases.pathPlaceholder", mode: "file" };
}

/** Redis-only extras appended after the shared server fields. */
export function redisExtras(): EngineField[] {
  return [
    { key: "dbIndex", labelKey: "databases.dbIndex", type: "number", default: "0", mode: "server" },
    { key: "address", labelKey: "databases.address", placeholderKey: "databases.addressPlaceholder", optional: true, mode: "server" },
    { key: "clientName", labelKey: "databases.clientName", optional: true, mode: "server" },
  ];
}

/** SugarDB-only fields for the embedded engine: a persistence directory, a
 * logical database index, and an optional password protecting the loopback
 * listener. */
export function sugardbFields(): EngineField[] {
  return [
    { key: "dbIndex", labelKey: "databases.dbIndex", type: "number", default: "0" },
    { key: "password", labelKey: "databases.password", type: "password", optional: true },
  ];
}
