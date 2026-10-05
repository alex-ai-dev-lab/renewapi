const { test } = require("node:test");
const assert = require("node:assert/strict");
const path = require("node:path");
const { createDisposableEnvironment } = require("./disposable-environment.cjs");

test("external application connections and secrets cannot enter the QA backend", () => {
  const parent = {
    Path: "system-path",
    SYSTEMROOT: "system-root",
    SQL_DSN: "postgres://external.example/app",
    sql_dsn: "alternate-case",
    LOG_SQL_DSN: "external-log-db",
    SQLITE_PATH: "/external.db",
    REDIS_CONN_STRING: "redis://external.example",
    SESSION_SECRET: "external-secret",
    HTTPS_PROXY: "external-proxy",
    UNKNOWN_APPLICATION_SECRET: "external-secret",
  };
  const env = createDisposableEnvironment(
    parent,
    "./temporary-qa",
    "fixture-secret",
  );
  assert.equal(env.SQL_DSN, "local");
  assert.equal(env.LOG_SQL_DSN, "");
  assert.equal(env.REDIS_CONN_STRING, "");
  assert.equal(env.SQLITE_PATH, path.resolve("temporary-qa", "qa.db"));
  assert.equal(env.SESSION_SECRET, "fixture-secret");
  assert.equal(env.sql_dsn, undefined);
  assert.equal(env.UNKNOWN_APPLICATION_SECRET, undefined);
  assert.equal(env.HTTPS_PROXY, undefined);
  assert.equal(env.Path, parent.Path);
  assert.equal(parent.SQL_DSN, "postgres://external.example/app");
});
