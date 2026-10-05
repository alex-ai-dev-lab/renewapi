/* Local disposable QA only. Never contacts production, payments, or model endpoints. */
const { spawn, spawnSync } = require("node:child_process"),
  { createRequire } = require("node:module"),
  crypto = require("node:crypto"),
  fs = require("node:fs"),
  os = require("node:os"),
  path = require("node:path");
const net = require("node:net");
const { createDisposableEnvironment } = require("./disposable-environment.cjs");
const root = path.resolve(__dirname, "../.."),
  req = createRequire(path.join(root, "web/default/package.json"));
const { chromium } = req("playwright"),
  { default: AxeBuilder } = req("@axe-core/playwright");
const dir = fs.mkdtempSync(path.join(os.tmpdir(), "renewapi-repair-")),
  out = path.join(root, ".cache/frontend-repair-qa"),
  origin = "http://localhost:39011";
fs.mkdirSync(out, { recursive: true });
const secret = () => `Qa!${crypto.randomBytes(8).toString("hex")}`,
  accounts = [
    { name: "qa_root", role: 100, password: secret() },
    { name: "qa_admin", role: 10, password: secret() },
    { name: "qa_user", role: 1, password: secret() },
  ];
const report = { checks: [], issues: [], pageErrors: [] };
let child, browser, log;
const delay = (ms) => new Promise((r) => setTimeout(r, ms)),
  assert = (v, m) => {
    if (!v) throw Error(m);
  };
async function json(r) {
  const d = await r.json();
  assert(
    r.ok() && d.success,
    `Request ${new URL(r.url()).pathname} failed (${r.status()})`,
  );
  return d;
}
async function check(name, fn) {
  if (
    process.env.QA_CHECK_FILTER &&
    !name.includes(process.env.QA_CHECK_FILTER)
  )
    return;
  try {
    await fn();
    report.checks.push(name);
    console.log("PASS", name);
  } catch (e) {
    report.issues.push({ name, error: e.message });
    console.log("FAIL", name, e.message.slice(0, 240));
  }
  fs.writeFileSync(
    path.join(out, "report.json"),
    JSON.stringify(report, null, 2),
  );
}
async function context(theme = "light", width = 1440) {
  const c = await browser.newContext({
    baseURL: origin,
    locale: "en-US",
    colorScheme: theme,
    viewport: { width, height: 1000 },
    reducedMotion: "reduce",
  });
  observeContext(c);
  await c.addCookies([{ name: "vite-ui-theme", value: theme, url: origin }]);
  return c;
}
function observeContext(c) {
  c.on("page", (p) => {
    p.on("pageerror", (e) =>
      report.pageErrors.push({ route: p.url(), error: e.message }),
    );
  });
}
async function page(c, route) {
  const p = await c.newPage();
  await p.goto(route, { waitUntil: "networkidle" });
  return p;
}
async function login(c, a) {
  const p = await page(c, "/sign-in");
  await p
    .getByRole("button", { name: "Continue with password", exact: true })
    .click();
  await p.locator("input[name=username]").fill(a.name);
  await p.locator("input[name=password]").fill(a.password);
  const r = p.waitForResponse(
    (r) =>
      r.url().includes("/api/user/login") && r.request().method() === "POST",
  );
  await p.locator("button[type=submit]").click();
  const data = (await json(await r)).data;
  await p.waitForURL(/dashboard/);
  await p.locator("#astryx-app-shell-main").waitFor();
  await p.close();
  return data;
}
function totp(s) {
  let bits = "";
  for (const c of s.replace(/=+$/, "").toUpperCase())
    bits += "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
      .indexOf(c)
      .toString(2)
      .padStart(5, "0");
  const bytes = [];
  for (let i = 0; i + 8 <= bits.length; i += 8)
    bytes.push(parseInt(bits.slice(i, i + 8), 2));
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000)));
  const h = crypto
    .createHmac("sha1", Buffer.from(bytes))
    .update(counter)
    .digest();
  return String((h.readUInt32BE(h[19] & 15) & 0x7fffffff) % 1000000).padStart(
    6,
    "0",
  );
}
(async () => {
  const env = {
    ...createDisposableEnvironment(process.env, dir, secret()),
    PORT: "39011",
  };
  // Refuse an occupied port instead of running setup against an unrelated service.
  await new Promise((resolve, reject) => {
    const probe = net.createServer();
    probe.once("error", reject);
    probe.listen(39011, () => probe.close(resolve));
  });
  const binary = path.join(root, ".cache/renewapi-ui-repair-qa.exe");
  assert(
    spawnSync(binary, ["migrate", "--up"], { cwd: dir, env, timeout: 90000 })
      .status === 0,
    "Migration failed",
  );
  log = fs.openSync(path.join(dir, "backend.log"), "w");
  child = spawn(binary, [], { cwd: dir, env, stdio: ["ignore", log, log] });
  child.on("error", (error) =>
    report.issues.push({ name: "backend", error: error.message }),
  );
  for (let i = 0; i < 100; i++) {
    assert(child.exitCode === null, "QA backend exited before readiness");
    try {
      if ((await fetch(origin + "/api/setup")).ok) break;
    } catch {}
    if (i === 99) throw Error("Backend not ready");
    await delay(200);
  }
  browser = await chromium.launch({
    executablePath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
    headless: true,
  });
  const rc = await context();
  await json(
    await rc.request.post("/api/setup", {
      data: {
        username: accounts[0].name,
        password: accounts[0].password,
        confirmPassword: accounts[0].password,
        SelfUseModeEnabled: false,
        DemoSiteEnabled: false,
      },
    }),
  );
  const ru = await login(rc, accounts[0]),
    headers = { "New-Api-User": String(ru.id) };
  for (const a of accounts.slice(1))
    await json(
      await rc.request.post("/api/user/", {
        headers,
        data: {
          username: a.name,
          password: a.password,
          display_name: a.name,
          role: a.role,
        },
      }),
    );
  for (const [key, value] of [
    ["SystemName", "QA Gateway"],
    ["Logo", "/missing-review-logo.png"],
    ["ServerAddress", origin],
    ["passkey.allow_insecure_origin", "true"],
    ["passkey.rp_id", "localhost"],
    ["passkey.origins", origin],
    ["passkey.enabled", "true"],
  ])
    await json(
      await rc.request.put("/api/option/", { headers, data: { key, value } }),
    );
  await json(
    await rc.request.post("/api/channel/", {
      headers,
      data: {
        mode: "single",
        channel: {
          name: "local-unreachable",
          type: 1,
          status: 1,
          key: "sk-local-placeholder",
          base_url: "http://127.0.0.1:9",
          models: "gpt-4o-mini",
          group: "default",
          priority: 0,
          weight: 1,
          auto_ban: 0,
        },
      },
    }),
  );
  for (const a of accounts.slice(1)) {
    const c = await context(),
      u = await login(c, a);
    await check(`role ${a.role}: availability and profile`, async () => {
      const d = (
        await json(
          await c.request.get("/api/user/self", {
            headers: { "New-Api-User": String(u.id) },
          }),
        )
      ).data;
      assert(d.has_password === true, "Password state missing");
      const p = await page(c, "/model-list");
      assert(
        (await p.locator("body").innerText()).includes("Available"),
        "Allowed model marked unavailable",
      );
      await p.close();
    });
    await check(`role ${a.role}: palette permissions`, async () => {
      const p = await page(c, "/dashboard/overview");
      await p.keyboard.press("Control+k");
      await p.getByRole("dialog").waitFor();
      assert(
        (await p
          .getByRole("option", { name: /Platform settings|System Settings/ })
          .count()) === 0,
        "Root command exposed",
      );
      await p.close();
    });
    if (a.role === 10)
      await check("admin pages avoid root options", async () => {
        const p = await c.newPage(),
          calls = [];
        p.on("response", (r) => {
          if (new URL(r.url()).pathname === "/api/option/")
            calls.push(r.status());
        });
        for (const r of ["/models/metadata", "/subscriptions"])
          await p.goto(r, { waitUntil: "networkidle" });
        assert(calls.length === 0, "Root options requested");
        await p.close();
      });
    if (a.role === 1) {
      await check("password cancellation clears secrets", async () => {
        const p = await page(c, "/profile");
        await p
          .locator("#astryx-app-shell-main")
          .getByRole("button", { name: "Change Password", exact: true })
          .click();
        await p.locator("#currentPassword").fill("dummy-draft");
        await p
          .getByRole("dialog")
          .getByRole("button", { name: "Cancel", exact: true })
          .click();
        await p
          .locator("#astryx-app-shell-main")
          .getByRole("button", { name: "Change Password", exact: true })
          .click();
        assert(
          (await p.locator("#currentPassword").inputValue()) === "",
          "Cancelled draft remained",
        );
        await p.close();
      });
      await check("password change persists", async () => {
        const p = await page(c, "/profile");
        await p
          .locator("#astryx-app-shell-main")
          .getByRole("button", { name: "Change Password", exact: true })
          .click();
        const next = secret();
        await p.locator("#currentPassword").fill(a.password);
        await p.locator("#newPassword").fill(next);
        await p.locator("#confirmPassword").fill(next);
        const done = p.waitForResponse(
          (r) =>
            r.request().method() === "PUT" &&
            r.url().includes("/api/user/self"),
        );
        await p
          .getByRole("dialog")
          .getByRole("button", { name: "Change Password", exact: true })
          .click();
        await json(await done);
        a.password = next;
        await p.close();
        const verify = await context();
        await login(verify, a);
        await verify.close();
      });
      await check("2FA load error has retry", async () => {
        const p = await c.newPage();
        await p.route("**/api/user/2fa/status", (r) =>
          r.fulfill({
            status: 503,
            contentType: "application/json",
            body: JSON.stringify({
              success: false,
              message: "fixture unavailable",
            }),
          }),
        );
        await p.goto("/profile", { waitUntil: "networkidle" });
        const error = p
          .getByRole("alert")
          .filter({ hasText: "Failed to load 2FA status" });
        await error.waitFor();
        assert(
          (await error.getByRole("button", { name: "Retry" }).count()) === 1,
          "Missing retry",
        );
        await p.close();
      });
      await check("2FA setup and disable persist", async () => {
        const p = await page(c, "/profile");
        const setup = p.waitForResponse((r) =>
          r.url().includes("/api/user/2fa/setup"),
        );
        await p
          .getByRole("button", { name: "Enable 2FA", exact: true })
          .click();
        const d = (await json(await setup)).data;
        await p.locator("#twofa-enable-code").fill(totp(d.secret));
        const en = p.waitForResponse((r) =>
          r.url().includes("/api/user/2fa/enable"),
        );
        await p
          .getByRole("dialog")
          .getByRole("button", { name: "Verify", exact: true })
          .click();
        await json(await en);
        await p
          .getByRole("button", { name: "Disable 2FA", exact: true })
          .click();
        const dialog = p.getByRole("dialog");
        assert(
          await dialog
            .getByRole("button", { name: "Disable 2FA", exact: true })
            .isDisabled(),
          "No disable confirmation",
        );
        await p.locator("#twofa-disable-code").fill(d.backup_codes[0]);
        await dialog.getByRole("checkbox").click();
        const dis = p.waitForResponse((r) =>
          r.url().includes("/api/user/2fa/disable"),
        );
        await dialog
          .getByRole("button", { name: "Disable 2FA", exact: true })
          .click();
        await json(await dis);
        await p.close();
      });
      await check(
        "Passkey registration with a virtual authenticator",
        async () => {
          const p = await c.newPage();
          const notes = [];
          p.on("console", (m) => {
            if (m.type() === "error") notes.push(m.text());
          });
          p.on("response", async (r) => {
            if (
              r.url().includes("/passkey/") &&
              r.request().method() === "POST"
            ) {
              try {
                const d = await r.json();
                notes.push(
                  new URL(r.url()).pathname +
                    ": " +
                    String(d.success) +
                    " " +
                    String(d.message || ""),
                );
              } catch {}
            }
          });
          const client = await c.newCDPSession(p);
          await client.send("WebAuthn.enable");
          await client.send("WebAuthn.addVirtualAuthenticator", {
            options: {
              protocol: "ctap2",
              transport: "internal",
              hasResidentKey: true,
              hasUserVerification: true,
              isUserVerified: true,
              automaticPresenceSimulation: true,
            },
          });
          await p.goto("/profile", { waitUntil: "networkidle" });
          const finish = p.waitForResponse((r) =>
            r.url().includes("/passkey/register/finish"),
          );
          await p
            .getByRole("button", { name: "Enable Passkey", exact: true })
            .click();
          try {
            await json(await finish);
          } catch (e) {
            throw Error("Passkey registration: " + notes.join("; "));
          }
          await p
            .getByRole("button", { name: "Remove Passkey", exact: true })
            .waitFor();
          await client.detach();
          await p.close();
        },
      );
      for (const width of [1440, 390, 320])
        await check(`user layouts ${width}`, async () => {
          const p = await c.newPage();
          await p.setViewportSize({ width, height: 844 });
          for (const r of [
            "/dashboard/overview",
            "/keys",
            "/usage-logs/drawing",
            "/usage-logs/task",
            "/profile",
            "/wallet",
          ]) {
            await p.goto(r, { waitUntil: "networkidle" });
            assert(
              await p.evaluate(
                () => document.documentElement.scrollWidth <= innerWidth + 2,
              ),
              "Overflow " + r,
            );
            await p.screenshot({
              path: path.join(
                out,
                `user-${width}-${r.replaceAll("/", "-")}.png`,
              ),
            });
          }
          await p.close();
        });
    }
    await c.close();
  }
  for (const theme of ["light", "dark"])
    for (const width of [1440, 390])
      await check(`public ${theme} ${width}`, async () => {
        const c = await context(theme, width),
          p = await c.newPage();
        for (const r of ["/", "/docs", "/pricing", "/sign-in"]) {
          await p.goto(r, { waitUntil: "networkidle" });
          assert(
            await p.evaluate(
              () => document.documentElement.scrollWidth <= innerWidth + 2,
            ),
            "Overflow " + r,
          );
          if (r === "/sign-in")
            await p
              .getByRole("button", {
                name: "Sign in with Passkey",
                exact: true,
              })
              .waitFor();
          if (r === "/docs")
            assert(
              (await p.locator("h1").innerText()).includes("QA Gateway"),
              "Incorrect docs brand",
            );
          if (r === "/pricing") {
            const box = await p.locator("h1").boundingBox();
            assert(box && box.y >= 60, "Header overlap");
          }
          const a = await new AxeBuilder({ page: p })
            .withTags(["wcag2a", "wcag2aa", "wcag21aa"])
            .analyze();
          if (a.violations.length)
            fs.writeFileSync(
              path.join(
                out,
                `axe-${theme}-${width}-${r.replaceAll("/", "-")}.json`,
              ),
              JSON.stringify(a.violations, null, 2),
            );
          assert(
            a.violations.length === 0,
            "Accessibility " +
              r +
              ": " +
              a.violations.map((v) => v.id).join(","),
          );
          await p.screenshot({
            path: path.join(
              out,
              `public-${theme}-${width}-${r.replaceAll("/", "-")}.png`,
            ),
          });
        }
        await c.close();
      });
  await check("Chinese documentation and branded headings", async () => {
    const c = await browser.newContext({
      baseURL: origin,
      locale: "zh-CN",
      viewport: { width: 390, height: 844 },
    });
    observeContext(c);
    const p = await c.newPage();
    await p.goto("/docs", { waitUntil: "networkidle" });
    assert(
      (await p.locator("h1").innerText()).includes("QA Gateway"),
      "Brand missing",
    );
    assert(
      !(await p.locator("body").innerText()).includes(
        "Use environment variables for secrets.",
      ),
      "English instructions leaked",
    );
    await p.screenshot({ path: path.join(out, "docs-zh-mobile.png") });
    await c.close();
  });
  await check("blocked entry JS shows bootstrap", async () => {
    const c = await context(),
      p = await c.newPage();
    await p.route("**/static/js/**", (r) => r.abort());
    await p.goto("/", { waitUntil: "domcontentloaded" });
    assert(await p.locator("#app-bootstrap").isVisible(), "No bootstrap");
    await c.close();
  });
  await check(
    "slow route chunks show pending UI and then recover",
    async () => {
      const c = await context();
      await login(c, accounts[1]);
      const p = await c.newPage();
      await p.route("**/static/js/async/**", async (r) => {
        await delay(1800);
        await r.continue();
      });
      await p.goto("/wallet", { waitUntil: "domcontentloaded" });
      await p
        .locator("[role=status]")
        .filter({ hasText: "Loading..." })
        .first()
        .waitFor({ timeout: 10000 });
      await p.locator("#astryx-app-shell-main").waitFor({ timeout: 30000 });
      await c.close();
    },
  );
  await check("failed route chunks offer reload and recover", async () => {
    const c = await context();
    await login(c, accounts[1]);
    const p = await c.newPage();
    await p.route("**/static/js/async/**", (r) => r.abort());
    await p.goto("/wallet", { waitUntil: "domcontentloaded" });
    await p
      .getByRole("button", { name: "Reload page", exact: true })
      .waitFor({ timeout: 20000 });
    await p.unroute("**/static/js/async/**");
    await p.getByRole("button", { name: "Reload page", exact: true }).click();
    await p.locator("#astryx-app-shell-main").waitFor({ timeout: 30000 });
    await c.close();
  });
  await check(
    "catalog field faults render unknown, denied, and fallback access",
    async () => {
      const c = await context();
      try {
        await login(c, accounts[2]);
        const p = await c.newPage();
        let fault = "model-groups";
        await p.route("**/api/pricing", async (r) => {
          const response = await r.fetch();
          const data = await response.json();
          data.current_group = "default";
          data.usable_group = { default: "Default" };
          for (const model of data.data) {
            model.enable_groups = ["default"];
            if (fault === "model-groups") delete model.enable_groups;
          }
          if (fault === "empty-usable") data.usable_group = {};
          if (fault === "missing-usable") delete data.usable_group;
          await r.fulfill({ response, json: data });
        });
        for (const [input, label] of [
          ["model-groups", "Access unknown"],
          ["empty-usable", "No access"],
          ["missing-usable", "Available"],
        ]) {
          fault = input;
          await p.goto("/model-list", { waitUntil: "networkidle" });
          await p.getByText(label, { exact: true }).first().waitFor();
        }
      } finally {
        await c.close();
      }
    },
  );
  await check(
    "catalog module switch agrees across sidebar, palette, and onboarding",
    async () => {
      const c = await context();
      try {
        await login(c, accounts[2]);
        const p = await c.newPage();
        await p.route("**/api/status", async (r) => {
          const response = await r.fetch();
          const body = await response.json();
          body.data.SidebarModulesAdmin = JSON.stringify({
            console: { pricing: false },
          });
          await r.fulfill({ response, json: body });
        });
        await p.goto("/dashboard/overview", { waitUntil: "networkidle" });
        await p
          .getByRole("heading", { name: "Get started", exact: true })
          .waitFor();
        assert(
          (await p.locator('a[href="/model-list"]').count()) === 0,
          "Disabled catalog link remained",
        );
        await p.keyboard.press("Control+k");
        await p.getByRole("dialog").waitFor();
        assert(
          (await p
            .getByRole("option", { name: /Model List|Model Catalog/ })
            .count()) === 0,
          "Disabled catalog command remained",
        );
      } finally {
        await c.close();
      }
    },
  );
  await check(
    "super administrator retains system settings command",
    async () => {
      const p = await page(rc, "/dashboard/overview");
      await p.keyboard.press("Control+k");
      await p
        .getByRole("option", { name: /Platform settings|System Settings/ })
        .waitFor();
      await p.close();
    },
  );
  await check(
    "mobile navigation uses local logo fallback and closes by keyboard",
    async () => {
      const c = await context("light", 390);
      try {
        await login(c, accounts[2]);
        const p = await page(c, "/dashboard/overview");
        await p
          .getByRole("button", { name: "Open navigation", exact: true })
          .click();
        const drawer = p.getByRole("dialog", {
          name: "Navigation",
          exact: true,
        });
        await drawer.waitFor();
        const brand = drawer
          .getByRole("link", { name: "QA Gateway", exact: true })
          .first();
        await brand.waitFor();
        await p.waitForFunction(() => {
          const imgs = [
            ...document.querySelectorAll(
              '[role="dialog"] .snowapi-astryx-logo-link img',
            ),
          ];
          return (
            imgs.length > 0 &&
            imgs.every(
              (img) =>
                img.complete &&
                img.naturalWidth > 0 &&
                img.getAttribute("src") === "/logo.png",
            )
          );
        });
        await p.keyboard.press("Escape");
        await drawer.waitFor({ state: "hidden" });
      } finally {
        await c.close();
      }
    },
  );
  // Isolated read fixture: show the upgrade entry without enabling payments.
  async function prepareUpgradePage(c) {
    await login(c, accounts[2]);
    const p = await c.newPage();
    await p.route("**/api/subscription/plans", (r) =>
      r.fulfill({
        json: {
          success: true,
          data: [
            {
              plan: {
                id: 1,
                title: "QA Plan",
                price_amount: 10,
                currency: "USD",
                duration_unit: "month",
                duration_value: 1,
                quota_reset_period: "never",
                enabled: true,
                sort_order: 0,
                max_purchase_per_user: 0,
                total_amount: 1000,
              },
            },
          ],
        },
      }),
    );
    await p.goto("/dashboard/overview", { waitUntil: "networkidle" });
    await p.locator(".snowapi-event-upgrade").waitFor();
    return p;
  }
  await check(
    "subscription chunk loading can close by Escape or button without reopening",
    async () => {
      const c = await context();
      let release;
      const held = new Promise((resolve) => {
        release = resolve;
      });
      try {
        const p = await prepareUpgradePage(c);
        await p.route("**/static/js/async/**", async (r) => {
          await held;
          await r.continue();
        });
        const upgrade = p.locator(".snowapi-event-upgrade");
        await upgrade.click();
        const dialog = p.getByRole("dialog", {
          name: "Subscription Plans",
          exact: true,
        });
        await dialog.getByRole("status").waitFor();
        await p.keyboard.press("Escape");
        await dialog.waitFor({ state: "hidden" });
        await upgrade.click();
        await dialog.getByRole("status").waitFor();
        await dialog
          .getByRole("button", { name: "Close", exact: true })
          .click();
        await dialog.waitFor({ state: "hidden" });
        release();
        await p.waitForLoadState("networkidle");
        assert(
          (await dialog.count()) === 0 || !(await dialog.isVisible()),
          "Late chunk reopened the dialog",
        );
        await upgrade.click();
        await p.locator(".snowapi-upgrade-dialog").waitFor();
      } finally {
        release();
        await c.close();
      }
    },
  );
  await check(
    "subscription chunk failure can close and retry after network recovery",
    async () => {
      const c = await context();
      try {
        const p = await prepareUpgradePage(c);
        await p.route("**/static/js/async/**", (r) => r.abort());
        const upgrade = p.locator(".snowapi-event-upgrade");
        await upgrade.click();
        const dialog = p.getByRole("dialog", {
          name: "Subscription Plans",
          exact: true,
        });
        await dialog
          .getByRole("button", { name: "Retry", exact: true })
          .waitFor();
        await p.keyboard.press("Escape");
        await dialog.waitFor({ state: "hidden" });
        await upgrade.click();
        await dialog
          .getByRole("button", { name: "Close", exact: true })
          .click();
        await dialog.waitFor({ state: "hidden" });
        await p.unroute("**/static/js/async/**");
        await upgrade.click();
        await dialog
          .getByRole("button", { name: "Retry", exact: true })
          .click();
        await p.locator(".snowapi-upgrade-dialog").waitFor();
      } finally {
        await c.close();
      }
    },
  );
  await rc.close();
})()
  .catch((e) => {
    report.issues.push({ name: "harness", error: e.message });
    console.error(e.message);
  })
  .finally(async () => {
    if (browser) await browser.close();
    if (child && child.exitCode === null) {
      child.kill();
      await Promise.race([
        new Promise((r) => child.once("exit", r)),
        delay(4000),
      ]);
    }
    if (log !== undefined) fs.closeSync(log);
    fs.writeFileSync(
      path.join(out, "report.json"),
      JSON.stringify(report, null, 2),
    );
    console.log(
      JSON.stringify({
        passed: report.checks.length,
        failed: report.issues.length,
        pageErrors: report.pageErrors.length,
      }),
    );
    if (report.issues.length || report.pageErrors.length) process.exitCode = 1;
  });
