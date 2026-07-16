package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"aipi-go/internal/auth"
	"aipi-go/internal/build"
	"aipi-go/internal/config"
	"aipi-go/internal/database"
	"aipi-go/internal/generation"
	"aipi-go/internal/tasks"
	"aipi-go/internal/users"
)

type Router struct {
	cfg     config.Config
	db      *database.DB
	logger  *slog.Logger
	mux     *http.ServeMux
	tokens  auth.TokenManager
	queue   *generation.Queue
	taskHub *tasks.Hub
	userHub *users.Hub

	updateMu      sync.Mutex
	updateCache   systemUpdateVersion
	updateCacheAt time.Time
}

func NewRouter(cfg config.Config, db *database.DB, logger *slog.Logger) http.Handler {
	router := &Router{
		cfg:    cfg,
		db:     db,
		logger: logger,
		mux:    http.NewServeMux(),
		tokens: auth.NewTokenManager(cfg.Database),
	}
	router.taskHub = tasks.NewHub()
	router.userHub = users.NewHub()
	router.queue = generation.NewQueue(db, logger, 0, router.taskHub, router.userHub)
	router.queue.Start()
	router.routes()
	return router.withMiddleware(router.mux)
}

func (r *Router) routes() {
	r.mux.HandleFunc("/api/health", r.health)
	r.mux.HandleFunc("/api/upstream/stability", r.upstreamStability)
	r.mux.HandleFunc("/api/go/migration", r.migrationStatus)
	r.mux.HandleFunc("/api/dashboard", r.dashboard)
	r.mux.HandleFunc("/api/admin/login", r.adminLogin)
	r.mux.HandleFunc("/api/admin/session", r.adminSession)
	r.mux.HandleFunc("/api/users/register", r.registerUser)
	r.mux.HandleFunc("/api/users/login", r.userLogin)
	r.mux.HandleFunc("/api/users/verify-email", r.verifyEmail)
	r.mux.HandleFunc("/api/users/verify-email-change", r.verifyEmailChange)
	r.mux.HandleFunc("/api/users/password/forgot", r.forgotPassword)
	r.mux.HandleFunc("/api/users/password/reset", r.resetPassword)
	r.mux.HandleFunc("/api/users", r.listUsers)
	r.mux.HandleFunc("/api/users/", r.userProfile)
	r.mux.HandleFunc("/api/api-providers/model-details", r.providerModelDetails)
	r.mux.HandleFunc("/api/api-providers/models", r.providerModelDetails)
	r.mux.HandleFunc("/api/api-providers", r.listProviders)
	r.mux.HandleFunc("/api/api-providers/", r.providerByID)
	r.mux.HandleFunc("/api/models", r.listModels)
	r.mux.HandleFunc("/api/models/", r.modelByID)
	r.mux.HandleFunc("/api/api-access/keys", r.userAPIAccessKeys)
	r.mux.HandleFunc("/api/api-access/keys/", r.userAPIAccessKeyByID)
	r.mux.HandleFunc("/api/api-access/logs", r.userAPIAccessLogs)
	r.mux.HandleFunc("/api/api-access/logs/trend", r.userAPIAccessTrend)
	r.mux.HandleFunc("/api/admin/api-access/keys", r.adminAPIAccessKeys)
	r.mux.HandleFunc("/api/admin/api-access/keys/", r.adminAPIAccessKeyByID)
	r.mux.HandleFunc("/api/admin/api-access/logs", r.adminAPIAccessLogs)
	r.mux.HandleFunc("/api/admin/system-update", r.systemUpdate)
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
	r.mux.HandleFunc("/api/tasks/stats", r.taskStats)
	r.mux.HandleFunc("/api/tasks", r.listTasks)
	r.mux.HandleFunc("/api/tasks/", r.taskByID)
	r.mux.HandleFunc("/api/system-logs", r.listSystemLogs)
	r.mux.HandleFunc("/api/system-logs/detail", r.systemLogDetail)
	r.mux.HandleFunc("/api/system-logs/stream", r.systemLogStream)
	r.mux.HandleFunc("/api/system-logs/", r.deleteSystemLog)
	r.mux.HandleFunc("/api/settings/public", r.publicSettings)
	r.mux.HandleFunc("/api/settings/account-pool", r.accountPoolSettings)
	r.mux.HandleFunc("/api/settings/test-email", r.testSettingEndpoint)
	r.mux.HandleFunc("/api/settings", r.settings)
	r.mux.HandleFunc("/api/account-pool/accounts", r.accountPoolAccounts)
	r.mux.HandleFunc("/v1/models", r.compatModels)
	r.mux.HandleFunc("/v1/images/generations", r.compatImageGenerations)
	r.mux.HandleFunc("/v1/images/edits", r.compatImageEdits)

	if r.cfg.ServeStatic {
		r.mux.HandleFunc("/", r.staticFallback)
	}
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

func (r *Router) migrationStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"phase": "go-primary",
			"modules": []map[string]string{
				{"name": "config/database/health", "status": "ready"},
				{"name": "users/admin/auth/api-keys", "status": "ready"},
				{"name": "models/providers", "status": "ready"},
				{"name": "generation/tasks/queue", "status": "ready"},
				{"name": "openai-compatible-image-api", "status": "ready"},
				{"name": "api-access-management", "status": "ready"},
				{"name": "billing/subscriptions/recharge", "status": "ready"},
			},
		},
	})
}

var adminPathPattern = regexp.MustCompile(`^/admin(?:/.*)?$`)

func (r *Router) staticFallback(w http.ResponseWriter, req *http.Request) {
	if strings.HasPrefix(req.URL.Path, "/api/") ||
		strings.HasPrefix(req.URL.Path, "/oauth/") ||
		strings.HasPrefix(req.URL.Path, "/v1/") ||
		strings.HasPrefix(req.URL.Path, "/ws/") {
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "接口尚未迁移到 Go 服务"})
		return
	}

	publicDir := filepath.Clean(r.cfg.PublicDir)
	requestPath := strings.TrimPrefix(req.URL.Path, "/")
	if requestPath != "" {
		candidate := filepath.Join(publicDir, requestPath)
		if isSafePublicPath(publicDir, candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				setStaticNoStoreHeaders(w, req.URL.Path)
				http.ServeFile(w, req, candidate)
				return
			}
		}
	}

	indexPath := filepath.Join(publicDir, "web", "index.html")
	if adminPathPattern.MatchString(req.URL.Path) {
		indexPath = filepath.Join(publicDir, "admin", "index.html")
	}
	setStaticNoStoreHeaders(w, req.URL.Path)
	http.ServeFile(w, req, indexPath)
}

func setStaticNoStoreHeaders(w http.ResponseWriter, requestPath string) {
	if requestPath == "/" ||
		requestPath == "/admin" ||
		requestPath == "/web" ||
		requestPath == "/web/" ||
		requestPath == "/admin/" ||
		strings.HasSuffix(requestPath, ".html") {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		return
	}
	if strings.HasPrefix(requestPath, "/web/") || strings.HasPrefix(requestPath, "/admin/") {
		if strings.HasSuffix(requestPath, ".js") ||
			strings.HasSuffix(requestPath, ".css") ||
			strings.HasSuffix(requestPath, ".json") ||
			strings.HasSuffix(requestPath, ".svg") {
			w.Header().Set("Cache-Control", "no-cache, must-revalidate, max-age=0")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
}

func (r *Router) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		startedAt := time.Now()
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
		}()
		if r.cfg.RequestBodyLimit > 0 {
			req.Body = http.MaxBytesReader(w, req.Body, r.cfg.RequestBodyLimit)
		}
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

func isSafePublicPath(root string, candidate string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	return absCandidate == absRoot || strings.HasPrefix(absCandidate, absRoot+string(os.PathSeparator))
}
