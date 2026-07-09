package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"payment-gateway/ent"
	configsvc "payment-gateway/internal/domain/configs/service"
	"payment-gateway/internal/platform/httpx"
)

type Handler struct {
	service configsvc.Service
}

func New(service configsvc.Service) Handler {
	return Handler{service: service}
}

type updateGatewayConfigRequest struct {
	SiteName                       string         `json:"site_name"`
	GatewayBaseURL                 string         `json:"gateway_base_url"`
	DefaultCurrency                string         `json:"default_currency"`
	DefaultLocale                  string         `json:"default_locale"`
	RequestIDEnabled               bool           `json:"request_id_enabled"`
	MaintenanceMode                bool           `json:"maintenance_mode"`
	OrderDefaultTTLSeconds         int            `json:"order_default_ttl_seconds"`
	OrderExpireScanIntervalSeconds int            `json:"order_expire_scan_interval_seconds"`
	OrderExpireScanLimit           int            `json:"order_expire_scan_limit"`
	OrderExpireWorkerConcurrency   int            `json:"order_expire_worker_concurrency"`
	OpenAPIRateLimitEnabled        bool           `json:"open_api_rate_limit_enabled"`
	OpenAPIRateLimit               int            `json:"open_api_rate_limit"`
	OpenAPIRateLimitWindowSeconds  int            `json:"open_api_rate_limit_window_seconds"`
	Extra                          map[string]any `json:"extra"`
}

func (h Handler) GetGatewayConfig(ctx *gin.Context) {
	cfg, err := h.service.GetGatewayConfig(ctx.Request.Context())
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "get_gateway_config_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"gateway_config": serializeGatewayConfig(cfg)})
}

func (h Handler) GetPublicSiteConfig(ctx *gin.Context) {
	cfg, err := h.service.GetGatewayConfig(ctx.Request.Context())
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "get_site_config_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"site_config": gin.H{
		"site_name":      cfg.SiteName,
		"default_locale": cfg.DefaultLocale,
	}})
}

func (h Handler) UpdateGatewayConfig(ctx *gin.Context) {
	var req updateGatewayConfigRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	cfg, err := h.service.UpdateGatewayConfig(ctx.Request.Context(), configsvc.UpdateGatewayConfigInput{
		SiteName:                       req.SiteName,
		GatewayBaseURL:                 req.GatewayBaseURL,
		DefaultCurrency:                req.DefaultCurrency,
		DefaultLocale:                  req.DefaultLocale,
		RequestIDEnabled:               req.RequestIDEnabled,
		MaintenanceMode:                req.MaintenanceMode,
		OrderDefaultTTLSeconds:         req.OrderDefaultTTLSeconds,
		OrderExpireScanIntervalSeconds: req.OrderExpireScanIntervalSeconds,
		OrderExpireScanLimit:           req.OrderExpireScanLimit,
		OrderExpireWorkerConcurrency:   req.OrderExpireWorkerConcurrency,
		OpenAPIRateLimitEnabled:        req.OpenAPIRateLimitEnabled,
		OpenAPIRateLimit:               req.OpenAPIRateLimit,
		OpenAPIRateLimitWindowSeconds:  req.OpenAPIRateLimitWindowSeconds,
		Extra:                          req.Extra,
	})
	if err != nil {
		status := http.StatusBadRequest
		if err != configsvc.ErrInvalidGatewayBaseURL {
			status = http.StatusInternalServerError
		}
		httpx.JSONError(ctx, status, "update_gateway_config_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"gateway_config": serializeGatewayConfig(cfg)})
}

func serializeGatewayConfig(cfg *ent.GatewayConfig) gin.H {
	return gin.H{
		"id":                                 cfg.ID,
		"site_name":                          cfg.SiteName,
		"gateway_base_url":                   cfg.GatewayBaseURL,
		"payment_notify_path":                cfg.PaymentNotifyPath,
		"default_currency":                   cfg.DefaultCurrency,
		"default_locale":                     cfg.DefaultLocale,
		"request_id_enabled":                 cfg.RequestIDEnabled,
		"maintenance_mode":                   cfg.MaintenanceMode,
		"order_default_ttl_seconds":          cfg.OrderDefaultTTLSeconds,
		"order_expire_scan_interval_seconds": cfg.OrderExpireScanIntervalSeconds,
		"order_expire_scan_limit":            cfg.OrderExpireScanLimit,
		"order_expire_worker_concurrency":    cfg.OrderExpireWorkerConcurrency,
		"open_api_rate_limit_enabled":        cfg.OpenAPIRateLimitEnabled,
		"open_api_rate_limit":                cfg.OpenAPIRateLimit,
		"open_api_rate_limit_window_seconds": cfg.OpenAPIRateLimitWindowSeconds,
		"extra":                              cfg.Extra,
		"created_at":                         cfg.CreatedAt,
		"updated_at":                         cfg.UpdatedAt,
	}
}
