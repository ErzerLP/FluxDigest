package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"rss-platform/internal/app/api/handlers"
	"rss-platform/internal/app/api/middleware"
	"rss-platform/internal/telemetry"
)

type routerConfig struct {
	apiKey        string
	articleReader handlers.ArticleReader
	dossierReader handlers.DossierReader
	digestReader  handlers.DigestReader
	profileReader handlers.ProfileReader
	jobTrigger    handlers.JobTrigger
	admin         handlers.AdminDeps
	metrics       *telemetry.Metrics
	staticDir     string
}

// Option 定义 router 组装选项。
type Option func(*routerConfig)

// WithAPIKey 配置 job 接口使用的 API key。
func WithAPIKey(apiKey string) Option {
	return func(cfg *routerConfig) {
		cfg.apiKey = apiKey
	}
}

// WithArticleReader 注入文章读取依赖。
func WithArticleReader(reader handlers.ArticleReader) Option {
	return func(cfg *routerConfig) {
		cfg.articleReader = reader
	}
}

// WithDigestReader 注入日报读取依赖。
func WithDigestReader(reader handlers.DigestReader) Option {
	return func(cfg *routerConfig) {
		cfg.digestReader = reader
	}
}

// WithDossierReader 注入 dossier 读取依赖。
func WithDossierReader(reader handlers.DossierReader) Option {
	return func(cfg *routerConfig) {
		cfg.dossierReader = reader
	}
}

// WithProfileReader 注入配置读取依赖。
func WithProfileReader(reader handlers.ProfileReader) Option {
	return func(cfg *routerConfig) {
		cfg.profileReader = reader
	}
}

// WithJobTrigger 注入日报任务触发依赖。
func WithJobTrigger(trigger handlers.JobTrigger) Option {
	return func(cfg *routerConfig) {
		cfg.jobTrigger = trigger
	}
}

// WithAdminDeps 注入 admin 路由依赖。
func WithAdminDeps(deps handlers.AdminDeps) Option {
	return func(cfg *routerConfig) {
		cfg.admin = deps
	}
}

// WithMetrics 注入 metrics 导出器。
func WithMetrics(metrics *telemetry.Metrics) Option {
	return func(cfg *routerConfig) {
		cfg.metrics = metrics
	}
}

// WithStaticDir 配置 SPA 静态资源目录。
func WithStaticDir(staticDir string) Option {
	return func(cfg *routerConfig) {
		cfg.staticDir = staticDir
	}
}

// NewRouter 创建可注入依赖的最小 API router。
func NewRouter(options ...Option) *gin.Engine {
	cfg := defaultRouterConfig()
	for _, option := range options {
		option(&cfg)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/metrics", gin.WrapH(cfg.metrics.Handler()))

	apiV1 := router.Group("/api/v1")
	handlers.RegisterArticleRoutes(apiV1, cfg.articleReader)
	handlers.RegisterDossierRoutes(apiV1, cfg.dossierReader)
	handlers.RegisterDigestRoutes(apiV1, cfg.digestReader)
	handlers.RegisterProfileRoutes(apiV1, cfg.profileReader)
	handlers.RegisterAdminRoutes(apiV1, cfg.admin)

	jobs := apiV1.Group("")
	if cfg.apiKey != "" {
		jobs.Use(middleware.RequireAPIKey(cfg.apiKey))
	}
	handlers.RegisterJobRoutes(jobs, cfg.jobTrigger)
	registerStaticRoutes(router, cfg.staticDir)

	return router
}

func defaultRouterConfig() routerConfig {
	return routerConfig{
		metrics:   telemetry.NewMetrics(),
		staticDir: os.Getenv("APP_STATIC_DIR"),
	}
}

func registerStaticRoutes(router *gin.Engine, staticDir string) {
	if staticDir == "" {
		return
	}

	indexFile := filepath.Join(staticDir, "index.html")
	if _, err := os.Stat(indexFile); err != nil {
		return
	}

	router.Static("/assets", filepath.Join(staticDir, "assets"))
	router.NoRoute(func(c *gin.Context) {
		if !shouldServeSPAIndex(c.Request) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.File(indexFile)
	})
}

func shouldServeSPAIndex(request *http.Request) bool {
	if request == nil {
		return false
	}

	switch request.Method {
	case http.MethodGet, http.MethodHead:
	default:
		return false
	}

	if isReservedRoute(request.URL.Path) {
		return false
	}

	return acceptsHTML(request.Header.Get("Accept"))
}

func isReservedRoute(path string) bool {
	normalized := strings.TrimRight(path, "/")
	if normalized == "" {
		normalized = "/"
	}

	return normalized == "/api" ||
		strings.HasPrefix(normalized, "/api/") ||
		normalized == "/healthz" ||
		strings.HasPrefix(normalized, "/healthz/") ||
		normalized == "/metrics" ||
		strings.HasPrefix(normalized, "/metrics/")
}

func acceptsHTML(accept string) bool {
	accept = strings.TrimSpace(accept)
	if accept == "" {
		return true
	}

	for _, value := range strings.Split(accept, ",") {
		mediaType := strings.TrimSpace(strings.SplitN(value, ";", 2)[0])
		if mediaType == "text/html" || mediaType == "*/*" {
			return true
		}
	}

	return false
}
