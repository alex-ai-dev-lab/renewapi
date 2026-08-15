package main

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/pkg/compat"
	"github.com/QuantumNous/new-api/pkg/compat/errornorm"
	"github.com/QuantumNous/new-api/pkg/compat/pricesync"
	"github.com/QuantumNous/new-api/pkg/compat/scheduler"
	"github.com/QuantumNous/new-api/pkg/compat/toolschema"
	"github.com/QuantumNous/new-api/pkg/compat/ua"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/router"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/requestguard"
	_ "github.com/QuantumNous/new-api/setting/performance_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	_ "net/http/pprof"
)

//go:embed web/default/dist
var buildFS embed.FS

//go:embed web/default/dist/index.html
var indexPage []byte

//go:embed web/classic/dist
var classicBuildFS embed.FS

//go:embed web/classic/dist/index.html
var classicIndexPage []byte

func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		os.Exit(runMigrationCommand(os.Args[2:]))
	}
	startTime := time.Now()
	signalCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	workers := newBackgroundWorkers()
	startWorker := func(name string, run func(context.Context)) {
		workers.Go(name, func(ctx context.Context) {
			common.SysLog("worker started: " + name)
			run(ctx)
			common.SysLog("worker stopped: " + name)
		})
	}

	err := InitResources()
	if err != nil {
		common.FatalLog("failed to initialize resources: " + err.Error())
		return
	}

	common.SysLog("New API " + common.Version + " started")
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	if common.DebugEnabled {
		common.SysLog("running in debug mode")
	}

	resourcesClosed := false
	closeResources := func() {
		if resourcesClosed {
			return
		}
		resourcesClosed = true
		if err := common.CloseRedis(); err != nil {
			common.SysError("failed to close Redis: " + err.Error())
		}
		if err := model.CloseDB(); err != nil {
			common.SysError("failed to close database: " + err.Error())
		}
	}
	defer func() {
		workers.cancel()
		closeResources()
	}()
	startWorker("requestguard-observe", requestguard.RunObserveWorkers)
	startWorker("system-monitor", common.RunSystemMonitor)

	if common.RedisEnabled {
		// for compatibility with old versions
		common.MemoryCacheEnabled = true
	}
	if common.MemoryCacheEnabled {
		common.SysLog("memory cache enabled")
		common.SysLog(fmt.Sprintf("sync frequency: %d seconds", common.SyncFrequency))

		// Add panic recovery and retry for InitChannelCache
		func() {
			defer func() {
				if r := recover(); r != nil {
					common.SysLog(fmt.Sprintf("InitChannelCache panic: %v, retrying once", r))
					// Retry once
					_, _, fixErr := model.FixAbility()
					if fixErr != nil {
						common.FatalLog(fmt.Sprintf("InitChannelCache failed: %s", fixErr.Error()))
					}
				}
			}()
			model.InitChannelCache()
		}()

		startWorker("channel-cache-sync", func(ctx context.Context) { model.RunChannelCacheSync(ctx, common.SyncFrequency) })
	}

	// User-Agent cache for relay hot path (custom UA list feature)
	ua.InitCache()
	startWorker("user-agent-cache", ua.Run)
	startWorker("model-endpoint-cache", model.RunModelEndpointCacheSync)
	startWorker("channel-failure-cleanup", service.RunChannelFailureTrackerCleanup)

	// 热更新配置
	startWorker("option-sync", func(ctx context.Context) { model.RunOptionSync(ctx, common.SyncFrequency) })

	// 数据看板
	startWorker("quota-data", model.RunQuotaDataUpdater)

	if os.Getenv("CHANNEL_UPDATE_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_UPDATE_FREQUENCY"))
		if err != nil {
			common.FatalLog("failed to parse CHANNEL_UPDATE_FREQUENCY: " + err.Error())
		}
		startWorker("channel-balance", func(ctx context.Context) { controller.RunChannelBalanceUpdater(ctx, frequency) })
	}

	startWorker("channel-tests", controller.RunAutomaticChannelTests)

	// Codex credential auto-refresh check every 10 minutes, refresh when expires within 1 day
	startWorker("codex-credential-refresh", service.RunCodexCredentialAutoRefreshTask)

	// Subscription quota reset task (daily/weekly/monthly/custom)
	startWorker("subscription-reset", service.RunSubscriptionQuotaResetTask)
	startWorker("billing-reconciler", service.RunBillingReconciler)

	// Client identity rotation worker
	startWorker("client-identity-rotation", service.RunClientIdentityRotationWorker)

	// Official model price sync task (daily from models.dev)
	startWorker("official-price-sync", pricesync.Run)

	// Wire task polling adaptor factory (breaks service -> relay import cycle)
	service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor {
		a := relay.GetTaskAdaptor(platform)
		if a == nil {
			return nil
		}
		return a
	}

	// Channel upstream model update check task
	startWorker("channel-upstream-model-sync", scheduler.Run)

	if common.IsMasterNode && constant.UpdateTask {
		startWorker("midjourney-task-polling", controller.RunMidjourneyTaskBulk)
		startWorker("task-polling", controller.RunTaskBulk)
	}
	if os.Getenv("BATCH_UPDATE_ENABLED") == "true" {
		common.BatchUpdateEnabled = true
		common.SysLog("batch update enabled with interval " + strconv.Itoa(common.BatchUpdateInterval) + "s")
		startWorker("batch-updater", model.RunBatchUpdater)
	}

	if os.Getenv("ENABLE_PPROF") == "true" {
		pprofServer := &http.Server{Addr: "127.0.0.1:8005", Handler: http.DefaultServeMux}
		startWorker("pprof-server", func(ctx context.Context) {
			errCh := make(chan error, 1)
			go func() { errCh <- pprofServer.ListenAndServe() }()
			select {
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if err := pprofServer.Shutdown(shutdownCtx); err != nil {
					log.Printf("pprof shutdown: %v", err)
				}
			case err := <-errCh:
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Printf("pprof server: %v", err)
				}
			}
		})
		startWorker("pprof-monitor", common.RunPprofMonitor)
		common.SysLog("pprof enabled")
	}

	err = common.StartPyroScope()
	if err != nil {
		common.SysError(fmt.Sprintf("start pyroscope error : %v", err))
	}

	// Initialize HTTP server
	server := gin.New()
	server.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		common.SysLog(fmt.Sprintf("panic detected: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("Panic detected, error: %v. Please report at https://github.com/alex-ai-dev-lab/renewapi/issues", err),
				"type":    "new_api_panic",
			},
		})
	}))
	// This will cause SSE not to work!!!
	//server.Use(gzip.Gzip(gzip.DefaultCompression))
	server.Use(middleware.RequestId())
	server.Use(middleware.PoweredBy())
	server.Use(middleware.I18n())
	middleware.SetUpLogger(server)
	// Initialize session store
	// Refuse to boot with a missing or well-known session secret.
	if s := strings.TrimSpace(common.SessionSecret); s == "" || s == "change-me" {
		common.FatalLog("SESSION_SECRET is unset or uses the insecure default 'change-me'; " +
			"set a strong random value (e.g. `openssl rand -hex 32`) before starting")
	}
	store := cookie.NewStore([]byte(common.SessionSecret))
	// 依据对外地址协议决定是否仅 HTTPS 下发 Cookie；可用 COOKIE_SECURE 显式覆盖
	secureCookie := strings.HasPrefix(strings.ToLower(system_setting.ServerAddress), "https")
	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		secureCookie = v == "true"
	}
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   2592000, // 30 days
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
	server.Use(sessions.Sessions("session", store))

	InjectUmamiAnalytics()
	InjectGoogleAnalytics()

	// 设置路由
	router.SetRouter(server, router.ThemeAssets{
		DefaultBuildFS:   buildFS,
		DefaultIndexPage: indexPage,
		ClassicBuildFS:   classicBuildFS,
		ClassicIndexPage: classicIndexPage,
	})
	var port = os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(*common.Port)
	}

	// Log startup success message
	common.LogStartupSuccess(startTime, port)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: server,
	}
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case <-signalCtx.Done():
		common.SysLog("received shutdown signal")
	case listenErr := <-serverErr:
		if listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			common.SysError("HTTP server stopped unexpectedly: " + listenErr.Error())
		}
	}

	shutdownSeconds := common.GetEnvOrDefault("SHUTDOWN_TIMEOUT_SECONDS", 5)
	if shutdownSeconds <= 0 {
		shutdownSeconds = 5
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(shutdownSeconds)*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		common.SysError(fmt.Sprintf("graceful shutdown timed out: %v", err))
		if closeErr := srv.Close(); closeErr != nil {
			common.SysError(fmt.Sprintf("forced server close failed: %v", closeErr))
		}
	}
	if err := workers.Stop(shutdownCtx); err != nil {
		common.SysError("background worker shutdown failed: " + err.Error())
	}

	if err := model.ShutdownSpendLogBatch(shutdownCtx); err != nil {
		common.SysError("failed to flush spend log batch: " + err.Error())
	}
	if processed, err := service.FlushBillingOutbox(shutdownCtx, 100); err != nil {
		common.SysError("failed to flush billing outbox: " + err.Error())
	} else if processed > 0 {
		common.SysLog(fmt.Sprintf("flushed %d billing outbox events", processed))
	}

	if common.BatchUpdateEnabled {
		model.FlushBatchUpdates()
	}
	if common.IsDataExportEnabled() {
		model.SaveQuotaDataCache()
	}
	closeResources()
	common.SysLog("server exited")
}

func runMigrationCommand(args []string) int {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	check := flags.Bool("check", false, "verify that all schema migrations are applied")
	up := flags.Bool("up", false, "apply all pending schema migrations")
	status := flags.Bool("status", false, "print schema migration status")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	selected := 0
	for _, enabled := range []bool{*check, *up, *status} {
		if enabled {
			selected++
		}
	}
	if selected != 1 || flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: new-api migrate --check|--up|--status")
		return 2
	}

	_ = godotenv.Load(".env")
	originalArgs := os.Args
	os.Args = []string{originalArgs[0]}
	common.InitEnv()
	os.Args = originalArgs
	logger.SetupLogger()

	if err := model.InitDBForMigration(); err != nil {
		fmt.Fprintln(os.Stderr, "initialize main database:", err)
		return 1
	}
	if err := model.InitLogDBForMigration(); err != nil {
		fmt.Fprintln(os.Stderr, "initialize log database:", err)
		return 1
	}
	defer func() {
		if err := model.CloseDB(); err != nil {
			fmt.Fprintln(os.Stderr, "close database:", err)
		}
	}()

	if *up {
		if err := model.ApplySchemaMigrations(); err != nil {
			fmt.Fprintln(os.Stderr, "apply main schema migrations:", err)
			return 1
		}
		if err := model.ApplyLogSchemaMigrations(); err != nil {
			fmt.Fprintln(os.Stderr, "apply log schema migrations:", err)
			return 1
		}
	}

	statuses, err := model.GetSchemaMigrationStatus()
	if err != nil {
		fmt.Fprintln(os.Stderr, "read migration status:", err)
		return 1
	}
	for _, item := range statuses {
		state := "pending"
		if item.Applied {
			state = "applied"
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%dms\n", state, item.Key, item.Checksum, item.AppVersion, item.DurationMS)
	}
	if *status {
		return 0
	}
	if err := model.CheckSchemaMigrations(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := model.CheckLogSchema(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func InjectUmamiAnalytics() {
	analyticsInjectBuilder := &strings.Builder{}
	if os.Getenv("UMAMI_WEBSITE_ID") != "" {
		umamiSiteID := os.Getenv("UMAMI_WEBSITE_ID")
		umamiScriptURL := os.Getenv("UMAMI_SCRIPT_URL")
		if umamiScriptURL == "" {
			umamiScriptURL = "https://analytics.umami.is/script.js"
		}
		analyticsInjectBuilder.WriteString("<script defer src=\"")
		analyticsInjectBuilder.WriteString(umamiScriptURL)
		analyticsInjectBuilder.WriteString("\" data-website-id=\"")
		analyticsInjectBuilder.WriteString(umamiSiteID)
		analyticsInjectBuilder.WriteString("\"></script>")
	}
	analyticsInjectBuilder.WriteString("<!--Umami QuantumNous-->\n")
	analyticsInject := []byte(analyticsInjectBuilder.String())
	placeholder := []byte("<!--umami-->\n")
	indexPage = bytes.ReplaceAll(indexPage, placeholder, analyticsInject)
	classicIndexPage = bytes.ReplaceAll(classicIndexPage, placeholder, analyticsInject)
}

func InjectGoogleAnalytics() {
	analyticsInjectBuilder := &strings.Builder{}
	if os.Getenv("GOOGLE_ANALYTICS_ID") != "" {
		gaID := os.Getenv("GOOGLE_ANALYTICS_ID")
		// Google Analytics 4 (gtag.js)
		analyticsInjectBuilder.WriteString("<script async src=\"https://www.googletagmanager.com/gtag/js?id=")
		analyticsInjectBuilder.WriteString(gaID)
		analyticsInjectBuilder.WriteString("\"></script>")
		analyticsInjectBuilder.WriteString("<script>")
		analyticsInjectBuilder.WriteString("window.dataLayer = window.dataLayer || [];")
		analyticsInjectBuilder.WriteString("function gtag(){dataLayer.push(arguments);}")
		analyticsInjectBuilder.WriteString("gtag('js', new Date());")
		analyticsInjectBuilder.WriteString("gtag('config', '")
		analyticsInjectBuilder.WriteString(gaID)
		analyticsInjectBuilder.WriteString("');")
		analyticsInjectBuilder.WriteString("</script>")
	}
	analyticsInjectBuilder.WriteString("<!--Google Analytics QuantumNous-->\n")
	analyticsInject := []byte(analyticsInjectBuilder.String())
	placeholder := []byte("<!--Google Analytics-->\n")
	indexPage = bytes.ReplaceAll(indexPage, placeholder, analyticsInject)
	classicIndexPage = bytes.ReplaceAll(classicIndexPage, placeholder, analyticsInject)
}

func InitResources() error {
	// Initialize resources here if needed
	// This is a placeholder function for future resource initialization
	err := godotenv.Load(".env")
	if err != nil {
		if common.DebugEnabled {
			common.SysLog("No .env file found, using default environment variables. If needed, please create a .env file and set the relevant variables.")
		}
	}

	// 加载环境变量
	common.InitEnv()

	logger.SetupLogger()

	// Initialize model settings
	ratio_setting.InitRatioSettings()

	service.InitHttpClient()

	service.InitTokenEncoders()

	// Initialize SQL Database
	err = model.InitDB()
	if err != nil {
		common.FatalLog("failed to initialize database: " + err.Error())
		return err
	}

	model.CheckSetup()

	// Initialize options, should after model.InitDB()
	model.InitOptionMap()

	// 清理旧的磁盘缓存文件
	common.CleanupOldCacheFiles()

	// 初始化模型
	model.GetPricing()

	// Initialize SQL Database
	err = model.InitLogDB()
	if err != nil {
		return err
	}

	// Initialize Redis
	err = common.InitRedisClient()
	if err != nil {
		return err
	}

	perfmetrics.Init()

	// Register compat hooks (Step 3: overlay layer)
	compat.Register(errornorm.New())
	compat.Register(toolschema.New())
	compat.Register(compat.NewRequestGuardHook())

	// Initialize errornorm DB-backed rules store (Step 4)
	if err := errornorm.EnsureSchema(model.DB); err != nil {
		common.SysError("errornorm: failed to ensure schema: " + err.Error())
	} else {
		store := errornorm.NewStore(model.DB)
		if err := store.Reload(context.Background()); err != nil {
			common.SysError("errornorm: initial reload failed: " + err.Error())
		}
		errornorm.SetGlobalStore(store)
	}

	// Initialize i18n
	err = i18n.Init()
	if err != nil {
		common.SysError("failed to initialize i18n: " + err.Error())
		// Don't return error, i18n is not critical
	} else {
		common.SysLog("i18n initialized with languages: " + strings.Join(i18n.SupportedLanguages(), ", "))
	}
	// Register user language loader for lazy loading
	i18n.SetUserLangLoader(model.GetUserLanguage)

	// Load custom OAuth providers from database
	err = oauth.LoadCustomProviders()
	if err != nil {
		common.SysError("failed to load custom OAuth providers: " + err.Error())
		// Don't return error, custom OAuth is not critical
	}

	// Initialize user-agent cache
	model.InitUserAgentCache()

	return nil
}
