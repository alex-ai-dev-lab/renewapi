/* Check actual status markup against the built CSS, without backend or production data. */
const fs = require("node:fs");
const path = require("node:path");
const http = require("node:http");
const { createRequire } = require("node:module");
const { spawnSync } = require("node:child_process");
const root = path.resolve(__dirname, "../..");
const frontend = path.join(root, "web/default");
const req = createRequire(path.join(frontend, "package.json"));
const { chromium } = req("playwright");
const out = path.resolve(
  process.env.CONTRAST_QA_OUT || path.join(root, ".cache/color-contrast"),
);
fs.mkdirSync(out, { recursive: true });
const fixture = path.join(out, "fixture.json");
const bun =
  process.env.BUN_EXECUTABLE ||
  (process.platform === "win32"
    ? path.join(process.env.APPDATA, "npm/node_modules/bun/bin/bun.exe")
    : "bun");
const render = spawnSync(bun, ["scripts/color-contrast-fixture.tsx", fixture], {
  cwd: frontend,
  encoding: "utf8",
});
if (render.status !== 0)
  throw Error(render.stderr || "Could not render contrast fixture");
const { markup, presets } = JSON.parse(fs.readFileSync(fixture, "utf8"));
const dist = path.join(frontend, "dist");
const entry = fs.readFileSync(path.join(dist, "index.html"), "utf8");
const styles = [...entry.matchAll(/<link[^>]+href="([^"]+\.css)"[^>]*>/g)].map(
  (m) => m[1],
);
if (!styles.length)
  throw Error("Build the default frontend before running contrast QA");
const html = `<!doctype html><html><head>${styles.map((href) => `<link rel="stylesheet" href="${href}">`).join("")}<style>*,*::before,*::after{transition:none!important;animation:none!important} [data-samples]{background:var(--card);padding:16px;display:flex;flex-wrap:wrap;gap:8px} [data-contrast]{font-size:14px} [data-slot=dialog-content]{position:static!important}</style></head><body><main class="snowapi-astryx-content"><section data-samples="main">${markup}</section></main><div data-slot="dialog-content"><section data-samples="portal">${markup}</section></div></body></html>`;
const server = http.createServer((request, response) => {
  const pathname = new URL(request.url, "http://localhost").pathname;
  if (pathname === "/") {
    response.setHeader("Content-Type", "text/html");
    return response.end(html);
  }
  const file = path.resolve(dist, "." + pathname);
  if (
    !file.startsWith(dist + path.sep) ||
    !fs.existsSync(file) ||
    !fs.statSync(file).isFile()
  ) {
    response.statusCode = 404;
    return response.end();
  }
  response.setHeader(
    "Content-Type",
    file.endsWith(".css") ? "text/css" : "application/octet-stream",
  );
  fs.createReadStream(file).pipe(response);
});
(async () => {
  let browser;
  try {
    await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
    const executablePath =
      process.env.CHROME_PATH ||
      (process.platform === "win32"
        ? "C:/Program Files/Google/Chrome/Application/chrome.exe"
        : undefined);
    browser = await chromium.launch({ executablePath, headless: true });
    const page = await browser.newPage();
    await page.goto(`http://127.0.0.1:${server.address().port}`, {
      waitUntil: "networkidle",
    });
    const report = await page.evaluate((presets) => {
      const canvas = document.createElement("canvas");
      canvas.width = canvas.height = 1;
      const ctx = canvas.getContext("2d", { willReadFrequently: true });
      const rgba = (color) => {
        ctx.clearRect(0, 0, 1, 1);
        ctx.fillStyle = color;
        ctx.fillRect(0, 0, 1, 1);
        return [...ctx.getImageData(0, 0, 1, 1).data].map((v, i) =>
          i === 3 ? v / 255 : v,
        );
      };
      const mix = (a, b) =>
        a
          .slice(0, 3)
          .map((v, i) => v * a[3] + b[i] * (1 - a[3]))
          .concat(1);
      const luminance = (c) =>
        c
          .slice(0, 3)
          .map((v) => v / 255)
          .map((v) => (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4))
          .reduce((s, v, i) => s + v * [0.2126, 0.7152, 0.0722][i], 0);
      const ratio = (a, b) =>
        (Math.max(luminance(a), luminance(b)) + 0.05) /
        (Math.min(luminance(a), luminance(b)) + 0.05);
      const background = (e) => {
        const chain = [];
        for (let n = e; n; n = n.parentElement) chain.push(n);
        return chain
          .reverse()
          .reduce(
            (c, n) => mix(rgba(getComputedStyle(n).backgroundColor), c),
            [255, 255, 255, 1],
          );
      };
      const failures = [];
      let textChecks = 0;
      let chartChecks = 0;
      let scopeChecks = 0;
      let minimumText = Infinity;
      let minimumChart = Infinity;
      let weakestText, weakestChart;
      const sections = [...document.querySelectorAll("[data-samples]")];
      for (const theme of ["light", "dark"])
        for (const preset of presets)
          for (const design of ["snowflake", "poolside"])
            for (const scope of ["public", "console"]) {
              document.documentElement.classList.toggle(
                "dark",
                theme === "dark",
              );
              document.documentElement.dataset.siteDesign = design;
              document.body.dataset.themePreset = preset;
              if (scope === "console")
                document.body.dataset.snowapiConsole = "true";
              else delete document.body.dataset.snowapiConsole;
              for (const surface of [
                "background",
                "card",
                "popover",
                "muted",
                "accent",
              ]) {
                for (const section of sections)
                  section.style.background = `var(--${surface})`;
                for (const el of document.querySelectorAll("[data-contrast]")) {
                  const c = getComputedStyle(el);
                  const bg = background(el);
                  const value = ratio(mix(rgba(c.color), bg), bg);
                  textChecks++;
                  if (value < minimumText) {
                    minimumText = value;
                    weakestText = {
                      theme,
                      preset,
                      design,
                      scope,
                      surface,
                      sample: el.dataset.contrast,
                    };
                  }
                  if (value < 4.5)
                    failures.push({
                      theme,
                      preset,
                      design,
                      scope,
                      surface,
                      location: el.closest("[data-samples]").dataset.samples,
                      sample: el.dataset.contrast,
                      ratio: +value.toFixed(3),
                      color: c.color,
                    });
                }
              }
              const mainSamples = [
                ...sections[0].querySelectorAll("[data-contrast]"),
              ];
              const portalSamples = [
                ...sections[1].querySelectorAll("[data-contrast]"),
              ];
              mainSamples.forEach((el, index) => {
                scopeChecks++;
                if (
                  getComputedStyle(el).color !==
                  getComputedStyle(portalSamples[index]).color
                )
                  failures.push({
                    theme,
                    preset,
                    design,
                    scope,
                    sample: el.dataset.contrast,
                    reason: "Content and portal text inks differ",
                  });
              });
              // Chart strokes/markers have a separate non-text target on plotting surfaces.
              const vars = getComputedStyle(document.body);
              for (const surface of ["background", "card", "popover"])
                for (const slot of [3, 4, 5]) {
                  const value = ratio(
                    rgba(vars.getPropertyValue(`--chart-${slot}`)),
                    rgba(vars.getPropertyValue(`--${surface}`)),
                  );
                  chartChecks++;
                  if (value < minimumChart) {
                    minimumChart = value;
                    weakestChart = {
                      theme,
                      preset,
                      design,
                      scope,
                      surface,
                      slot,
                    };
                  }
                  if (value < 3)
                    failures.push({
                      theme,
                      preset,
                      design,
                      scope,
                      surface,
                      sample: `chart-${slot}`,
                      ratio: +value.toFixed(3),
                    });
                }
            }
      return {
        textChecks,
        chartChecks,
        scopeChecks,
        minimumText,
        minimumChart,
        weakestText,
        weakestChart,
        failures,
      };
    }, presets);
    fs.writeFileSync(
      path.join(out, "report.json"),
      JSON.stringify(report, null, 2),
    );
    if (process.env.CONTRAST_QA_SCREENSHOTS === "true") {
      for (const theme of ["light", "dark"]) {
        await page.evaluate((theme) => {
          document.documentElement.classList.toggle("dark", theme === "dark");
          document.documentElement.dataset.siteDesign = "snowflake";
          document.body.dataset.snowapiConsole = "true";
          document.body.dataset.themePreset = "default";
          document.querySelectorAll("[data-samples]").forEach((el) => {
            el.style.background = "var(--card)";
          });
        }, theme);
        await page.screenshot({
          path: path.join(out, `${theme}.png`),
          fullPage: true,
        });
      }
    }
    console.log(
      JSON.stringify(
        {
          ...report,
          failures: report.failures.length,
          examples: report.failures.slice(0, 6),
        },
        null,
        2,
      ),
    );
    if (report.failures.length) process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    await new Promise((resolve) => server.close(resolve));
  }
})().catch((error) => {
  console.error(error.message);
  process.exitCode = 1;
});
