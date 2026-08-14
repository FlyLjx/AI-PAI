package httpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"aipi-go/internal/apiaccess"
	"aipi-go/internal/auth"
	"aipi-go/internal/build"
	"aipi-go/internal/cleanupstatus"
	"aipi-go/internal/config"
	"aipi-go/internal/database"
	"aipi-go/internal/generation"
	"aipi-go/internal/requestmonitor"
	"golang.org/x/sync/singleflight"
)

type Router struct {
	cfg            config.Config
	db             *database.DB
	logger         *slog.Logger
	mux            *http.ServeMux
	tokens         auth.TokenManager
	queue          *generation.Queue
	notifications  *serviceNotificationManager
	requestMonitor *requestmonitor.Recorder
	cleanupTracker *cleanupstatus.Tracker

	updateMu      sync.Mutex
	updateCache   systemUpdateVersion
	updateCacheAt time.Time

	dynamicConcurrencyMu      sync.RWMutex
	dynamicConcurrencyCache   apiaccess.DynamicConcurrencyConfig
	dynamicConcurrencyCacheAt time.Time

	taskTimeoutMu      sync.RWMutex
	taskTimeoutCache   time.Duration
	taskTimeoutCacheAt time.Time

	// A subscription entitlement includes the current period's generation
	// usage. Coalesce concurrent requests for the same user so a burst does
	// not repeat the full usage scan for every request.
	subscriptionEntitlementGroup singleflight.Group
}

func NewRouter(cfg config.Config, db *database.DB, logger *slog.Logger, trackers ...*cleanupstatus.Tracker) http.Handler {
	var cleanupTracker *cleanupstatus.Tracker
	if len(trackers) > 0 {
		cleanupTracker = trackers[0]
	}
	router := &Router{
		cfg:            cfg,
		db:             db,
		logger:         logger,
		mux:            http.NewServeMux(),
		tokens:         auth.NewTokenManager(cfg.Database),
		notifications:  newServiceNotificationManager(db, logger),
		cleanupTracker: cleanupTracker,
	}
	router.queue = generation.NewQueue(db, logger, 0)
	router.initializeUpstreamMaintenancePause()
	router.requestMonitor = requestmonitor.NewRecorder(db, logger)
	router.queue.Start()
	router.routes()
	return router.withMiddleware(router.mux)
}

func (r *Router) routes() {
	r.mux.HandleFunc("/api/health", r.health)
	r.mux.HandleFunc("/api/upstream/stability", r.upstreamStability)
	r.mux.HandleFunc("/api/upstream/openai-status", r.openAIStatus)
	r.mux.HandleFunc("/api/dashboard", r.dashboard)
	r.mux.HandleFunc("/api/admin/login", r.adminLogin)
	r.mux.HandleFunc("/api/admin/session", r.adminSession)
	r.mux.HandleFunc("/api/admin/users/consumption-ranking", r.adminConsumptionRanking)
	r.mux.HandleFunc("/api/admin/users", r.adminUsers)
	r.mux.HandleFunc("/api/admin/user-model-prices", r.adminUserModelPrices)
	r.mux.HandleFunc("/api/admin/user-model-prices/", r.adminUserModelPriceByID)
	r.mux.HandleFunc("/api/users/register", r.registerUser)
	r.mux.HandleFunc("/api/users/register/challenge", r.registrationChallenge)
	r.mux.HandleFunc("/api/users/login", r.userLogin)
	r.mux.HandleFunc("/api/users/verify-email", r.verifyEmail)
	r.mux.HandleFunc("/api/users/verify-email-change", r.verifyEmailChange)
	r.mux.HandleFunc("/api/users/password/forgot", r.forgotPassword)
	r.mux.HandleFunc("/api/users/password/reset", r.resetPassword)
	r.mux.HandleFunc("/api/users", r.listUsers)
	r.mux.HandleFunc("/api/users/options", r.listUserOptions)
	r.mux.HandleFunc("/api/users/", r.userProfile)
	r.mux.HandleFunc("/api/api-providers", r.listProviders)
	r.mux.HandleFunc("/api/api-providers/", r.providerByID)
	r.mux.HandleFunc("/api/models", r.listModels)
	r.mux.HandleFunc("/api/models/pricing", r.publicModelPrices)
	r.mux.HandleFunc("/api/models/", r.modelByID)
	r.mux.HandleFunc("/api/api-access/keys", r.userAPIAccessKeys)
	r.mux.HandleFunc("/api/api-access/keys/", r.userAPIAccessKeyByID)
	r.mux.HandleFunc("/api/api-access/logs", r.userAPIAccessLogs)
	r.mux.HandleFunc("/api/api-access/logs/trend", r.userAPIAccessTrend)
	r.mux.HandleFunc("/api/api-access/logs/analytics", r.userAPIAccessAnalytics)
	r.mux.HandleFunc("/api/admin/api-access/keys", r.adminAPIAccessKeys)
	r.mux.HandleFunc("/api/admin/api-access/keys/", r.adminAPIAccessKeyByID)
	r.mux.HandleFunc("/api/admin/api-access/logs", r.adminAPIAccessLogs)
	r.mux.HandleFunc("/api/admin/api-access/operations", r.adminAPIAccessOperations)
	r.mux.HandleFunc("/api/admin/api-access/operations/live", r.adminAPIAccessOperationsLive)
	r.mux.HandleFunc("/api/admin/api-access/operations/ranking", r.adminAPIAccessOperationsRanking)
	r.mux.HandleFunc("/api/admin/api-access/operations/trend", r.adminAPIAccessOperationsTrend)
	r.mux.HandleFunc("/api/admin/mail-broadcast", r.mailBroadcast)
	r.mux.HandleFunc("/api/admin/mail-preview", r.mailPreview)
	r.mux.HandleFunc("/api/admin/mail-logs", r.adminMailLogs)
	r.mux.HandleFunc("/api/admin/request-monitor", r.adminRequestMonitor)
	r.mux.HandleFunc("/api/admin/upstream-maintenance", r.upstreamMaintenance)
	r.mux.HandleFunc("/api/admin/system-update", r.systemUpdate)
	r.mux.HandleFunc("/api/admin/data-export", r.adminDataExport)
	r.mux.HandleFunc("/api/admin/image-cleanup/status", r.imageCleanupStatus)
	r.mux.HandleFunc("/api/announcements/public", r.publicAnnouncements)
	r.mux.HandleFunc("/api/announcements", r.announcements)
	r.mux.HandleFunc("/api/announcements/", r.announcementByID)
	r.mux.HandleFunc("/api/subscriptions/public/plans", r.plans)
	r.mux.HandleFunc("/api/subscriptions/public/current", r.currentSubscription)
	r.mux.HandleFunc("/api/subscriptions/plans", r.adminPlans)
	r.mux.HandleFunc("/api/subscriptions/plans/", r.planByID)
	r.mux.HandleFunc("/api/recharge/qr-code", r.rechargeQRCode)
	r.mux.HandleFunc("/api/recharge/alipay/notify", r.alipayNotify)
	r.mux.HandleFunc("/api/recharge/history", r.rechargeHistory)
	r.mux.HandleFunc("/api/recharge/orders", r.rechargeOrders)
	r.mux.HandleFunc("/api/recharge", r.recharge)
	r.mux.HandleFunc("/api/recharge/", r.rechargeByID)
	r.mux.HandleFunc("/api/tasks", r.listTasks)
	r.mux.HandleFunc("/api/tasks/", r.taskByID)
	r.mux.HandleFunc("/api/system-logs", r.listSystemLogs)
	r.mux.HandleFunc("/api/system-logs/detail", r.systemLogDetail)
	r.mux.HandleFunc("/api/system-logs/", r.deleteSystemLog)
	r.mux.HandleFunc("/api/settings/public", r.publicSettings)
	r.mux.HandleFunc("/api/settings/test-bark", r.testSettingEndpoint)
	r.mux.HandleFunc("/api/settings", r.settings)
	r.mux.HandleFunc("/api/invites/summary", r.inviteSummary)
	r.mux.HandleFunc("/api/invites/", r.inviteByID)
	r.mux.HandleFunc("/api/invites", r.invites)
	r.mux.HandleFunc("/v1/models", r.compatModels)
	r.mux.HandleFunc("/v1/balance", r.compatBalance)
	r.mux.HandleFunc("/v1/chat/completions", r.compatChatCompletions)
	r.mux.HandleFunc("/v1/responses", r.compatResponses)
	r.mux.HandleFunc("/v1/images/generations", r.compatImageGenerations)
	r.mux.HandleFunc("/v1/images/edits", r.compatImageEdits)
}

func (r *Router) health(w http.ResponseWriter, _ *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := r.db.PingContext(ctx)
	status := "ok"
	if err != nil {
		status = "degraded"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"status": status,
			"build":  build.Info(),
			"mysql":  errString(err),
		},
	})
}

func (r *Router) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		startedAt := time.Now()
		if r.cfg.RequestBodyLimit > 0 {
			req.Body = http.MaxBytesReader(w, req.Body, r.cfg.RequestBodyLimit)
		}
		shouldRecord := r.requestMonitor != nil && requestmonitor.ShouldRecord(req)
		var captured requestmonitor.CapturedRequest
		responseWriter := &trackedResponseWriter{ResponseWriter: w}
		if shouldRecord {
			captured = requestmonitor.Capture(req)
			w = responseWriter
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				r.logger.Error("http panic",
					"path", req.URL.Path,
					"method", req.Method,
					"remoteAddr", req.RemoteAddr,
					"panic", recovered,
					"stack", string(debug.Stack()),
				)
				writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "服务器内部错误"})
			}
			duration := time.Since(startedAt)
			if shouldRecord {
				statusCode := responseWriter.statusCode
				if statusCode == 0 {
					statusCode = http.StatusOK
				}
				r.requestMonitor.Submit(requestmonitor.Record{
					Method: req.Method, Path: req.URL.Path,
					QueryParams: captured.QueryParams, BodyParams: captured.BodyParams,
					SourceIP: captured.SourceIP, SourceHost: captured.SourceHost,
					Origin: captured.Origin, Referer: captured.Referer, UserAgent: captured.UserAgent,
					StatusCode: statusCode, DurationMS: duration.Milliseconds(), ResponseBytes: responseWriter.responseBytes,
					CreatedAt: startedAt,
				})
			}
		}()
		r.applyCORS(w, req)
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, req)
		r.logger.Info("http request",
			"method", req.Method,
			"path", req.URL.Path,
			"durationMs", time.Since(startedAt).Milliseconds(),
			"remoteAddr", req.RemoteAddr,
		)
	})
}

type trackedResponseWriter struct {
	http.ResponseWriter
	statusCode    int
	responseBytes int64
}

func (writer *trackedResponseWriter) WriteHeader(statusCode int) {
	if writer.statusCode != 0 {
		return
	}
	writer.statusCode = statusCode
	writer.ResponseWriter.WriteHeader(statusCode)
}

func (writer *trackedResponseWriter) Write(value []byte) (int, error) {
	if writer.statusCode == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	written, err := writer.ResponseWriter.Write(value)
	writer.responseBytes += int64(written)
	return written, err
}

func (writer *trackedResponseWriter) ReadFrom(reader io.Reader) (int64, error) {
	if writer.statusCode == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	written, err := io.Copy(writer.ResponseWriter, reader)
	writer.responseBytes += written
	return written, err
}

func (writer *trackedResponseWriter) Flush() {
	if writer.statusCode == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(writer.ResponseWriter).Flush()
}

func (writer *trackedResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(writer.ResponseWriter).Hijack()
}

func (writer *trackedResponseWriter) Push(target string, options *http.PushOptions) error {
	if pusher, ok := writer.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, options)
	}
	return http.ErrNotSupported
}

func (writer *trackedResponseWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func (r *Router) applyCORS(w http.ResponseWriter, req *http.Request) {
	origin := req.Header.Get("Origin")
	if origin == "" {
		return
	}
	if !r.isAllowedOrigin(origin) {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
}

func (r *Router) isAllowedOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	for _, item := range r.cfg.CorsOrigins {
		if item == "*" || item == origin {
			return true
		}
	}
	return strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "http://[::1]:")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
