/* Local QA processes must not inherit application connections or credentials. */
const path = require("node:path");

function createDisposableEnvironment(parent, directory, sessionSecret) {
  const env = {};
  const systemKeys = new Set([
    "path",
    "systemroot",
    "windir",
    "comspec",
    "pathext",
    "temp",
    "tmp",
    "tmpdir",
    "home",
    "userprofile",
    "lang",
    "lc_all",
    "tz",
  ]);
  for (const [key, value] of Object.entries(parent)) {
    if (systemKeys.has(key.toLowerCase())) env[key] = value;
  }
  return {
    ...env,
    SQL_DSN: "local",
    LOG_SQL_DSN: "",
    SQLITE_PATH: path.join(path.resolve(directory), "qa.db"),
    REDIS_CONN_STRING: "",
    SESSION_SECRET: sessionSecret,
    COOKIE_SECURE: "false",
    GIN_MODE: "release",
    NODE_TYPE: "slave",
    MEMORY_CACHE_ENABLED: "false",
    OFFICIAL_PRICE_SYNC_ENABLED: "false",
    UPDATE_TASK: "false",
    BATCH_UPDATE_ENABLED: "false",
    GLOBAL_API_RATE_LIMIT: "5000",
    GLOBAL_WEB_RATE_LIMIT: "5000",
    PANEL_READ_RATE_LIMIT: "5000",
    PANEL_WRITE_RATE_LIMIT: "1000",
    SEARCH_RATE_LIMIT: "1000",
  };
}

module.exports = { createDisposableEnvironment };
