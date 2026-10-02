# Obsidian Control integration — 2026-10-01

> Archived implementation stage, completed and shipped in v1.0.0. The initial branch and no-push limits below were superseded by the user's release authorization; see the [release record](../2026-10-release-v1.0.0.md).

## Scope and baseline
- Fresh clone at D:/Code/renewapi/ui-20261001/renewapi, baseline d35afe9528fb630199b8b40e1a528f33afe3239d, approved isolated branch feature/obsidian-ui. Do not switch branches, commit, push, deploy, modify backend, touch classic or previous source/templates.
- User-approved current task and branch take precedence over historical repository main-only workflow notes. Do not read previous design/task documents or memories. Only use current source for behavior and this session's 01-obsidian-control visual specification.
- Baseline: Bun1.3.14 frozen install, bun test and bun run build:check passed before implementation.
- Final result must use real existing React/TypeScript components, hooks and API; no template mock data, fake stats or invented teams/projects.

## Shared visual contract
Main agent owns shared styles/index.css/theme.css/theme-presets.css, context theme defaults, components/ui, page-primitives, data-table styles, shared aurora component presentation, locales and final tests.
- Semantic variables remain --background/foreground/card/popover/primary/primary-foreground/secondary/muted/muted-foreground/accent/border/input/ring/sidebar/chart-* etc. Use existing Tailwind semantic classes.
- Dark bg #101213; card #171a1c; secondary #1d2225; text #e4e9e8; muted #9aa7a6; border #30383b; primary #8cd6b5; primary-foreground #10271d. Matching neutral light mode. Existing saved preset/font/scale/color preferences preserved.
- Default radius 4px (--radius), compact spacing 4/8/12/16/24/32, no gradient/glass/shadow, no exaggerated titles. Body 13–14px, h1 26px/600, labels11–12px, tabular metrics29px. Sidebar224/topbar56.
- Main removes active aurora-bento import and supplies concise compatibility for functional .aurora-* slots where shared components remain. Do not import old reference CSS in new code.
- Each owner may add a scoped CSS file in src/styles/obsidian-<area>.css, import in owned entry, use semantic variables, no global !important overrides. Responsive 1440/1280/1024/768/390, all grid children min-width0, tables/code own scrolling, no clipping main.
- All new strings t('English source key') with useTranslation in own components. Do NOT edit locales concurrently. Write exact key→zh/en additions into your own task fragment tasks/active/obsidian-i18n-<area>.json, main merges locales. Reuse existing keys whenever reasonable.
- Preserve copyrights. New files should use matching AGPL header if repo checks require.
- Run bun run typecheck after TS changes; do not format unrelated files. Main coordinates full build/format/test. No screenshot calls.

## Ownership
1. Home/auth agent: src/features/home/**, src/features/auth/** presentation only; obsidian-home.css, obsidian-auth.css; do not edit shared PublicLayout (request if needed).
2. Shell agent: src/components/layout/components/** (AuthenticatedLayout, AppSidebar/Header, PublicLayout, SectionPageLayout, Nav*) and obsidian-shell.css. Preserve all Provider/Auth, menu data from useSidebarView; no new hardcoded nav array. May edit corresponding sidebar hook only if absolutely necessary, preserve filtering.
3. Dashboard agent: src/features/dashboard/**, obsidian-dashboard.css. Real user/admin metrics and models/channels/users tabs retained. Do not edit shared chart theme libs; report needs to main.
4. User surfaces agent: src/features/keys, usage-logs, wallet, pricing, profile, chat, playground, rankings, about, legal, errors, setup, and obsidian-user.css. Preserve hooks/API/forms/permissions; focus presentation, cover actual subpanels.
5. Admin surfaces agent: src/features/channels, users, models, subscriptions, redemption-codes, performance-metrics, system-settings, channel-test-prompts, and obsidian-admin.css. Preserve hooks/API/form-schema/mutations and all advanced editors.
Main: shared styling/primitives/data-table/chart theme if needed, regression QA, i18n merge, documentation and overall integration.

## Safety and QA
- Native modern routes /sign-in and /dashboard, not classic /login /console.
- Site theme.frontend already defaults to default after setting initialization; don't alter backend defaults. HomePageContent blank uses new home; HTTP URL iframe and markdown custom content preserved.
- api success:false resolves promise: retain explicit feature success checks, token read endpoints, query names, pagination and cancellation.
- No live payments, emails, OAuth or paid model requests during checks. No credentials in source.
- Current environment Go1.22.3 vs required1.25.1: backend validation is not assumed until tested.

## Progress
- [x] Clean clone and baseline frontend install/tests/build.
- [x] Shared tokens and components.
- [x] Single-screen home/auth.
- [x] Sidebar/topbar shell.
- [x] Dashboard real metrics.
- [x] User and admin surfaces.
- [x] Localization and browser regression.
- [x] Final type/lint/test/build and delivery notes.

## Verification — 2026-10-02
- Final Bun suite: 96 passed, 0 failed, 253 assertions across 23 files.
- `bun run build:check` and Go 1.25.1 build (both frontend assets embedded): passed.
- Real Go + fresh SQLite + Chrome: 447 checks passed across public/Root/user routes, five widths, dark/light, default-dark and system preference, command dialog and access control. Zero recorded issues, page errors, console errors or network failures. No screenshots.
- New UI accessibility fixes include labels for analytics pagination/selects and profile switches, visible About link underlines, and readable light tabs/dark outline buttons.
- Full ESLint retains repository baseline errors/warnings; changed-file rule/severity comparison found no introduced diagnostics. Full format check flags 67 unchanged baseline files; modified files were formatted separately. No blanket lint fixes or state-logic refactors.
- Backend and classic source changes: none. No commits/pushes/deployment. Original source and template library untouched.
- See `docs/OBSIDIAN_UI.md` for run instructions and limitations; raw reports under ignored `qa-artifacts/obsidian/`.
