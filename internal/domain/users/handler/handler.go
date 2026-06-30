package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"payment-gateway/ent"
	usersvc "payment-gateway/internal/domain/users/service"
	"payment-gateway/internal/platform/config"
	"payment-gateway/internal/platform/httpx"
)

type Handler struct {
	service usersvc.Service
	authCfg config.AuthConfig
}

func New(service usersvc.Service, authCfg config.AuthConfig) Handler {
	return Handler{service: service, authCfg: authCfg}
}

type setupRequest struct {
	Username    string `json:"username" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	DisplayName string `json:"display_name"`
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type manageUserRequest struct {
	Username    string `json:"username" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
	RoleIDs     []int  `json:"role_ids"`
}

func (h Handler) Setup(ctx *gin.Context) {
	var req setupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	pair, err := h.service.Setup(ctx.Request.Context(), usersvc.SetupInput{
		Username:    req.Username,
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		UserAgent:   ctx.Request.UserAgent(),
		IPAddress:   ctx.ClientIP(),
	})
	if err != nil {
		if err == usersvc.ErrSetupAlreadyDone {
			httpx.JSONError(ctx, http.StatusConflict, "setup_completed", "setup already completed")
			return
		}
		httpx.JSONError(ctx, http.StatusInternalServerError, "setup_failed", err.Error())
		return
	}
	h.writeTokenResponse(ctx, pair)
}

func (h Handler) Login(ctx *gin.Context) {
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	pair, err := h.service.Login(ctx.Request.Context(), usersvc.LoginInput{
		Username:  req.Username,
		Password:  req.Password,
		UserAgent: ctx.Request.UserAgent(),
		IPAddress: ctx.ClientIP(),
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}
	h.writeTokenResponse(ctx, pair)
}

func (h Handler) Refresh(ctx *gin.Context) {
	refreshToken, err := ctx.Cookie(h.authCfg.RefreshCookieName)
	if err != nil {
		httpx.JSONError(ctx, http.StatusUnauthorized, "invalid_refresh_token", "missing refresh token")
		return
	}
	pair, err := h.service.Refresh(ctx.Request.Context(), refreshToken, ctx.Request.UserAgent(), ctx.ClientIP())
	if err != nil {
		httpx.JSONError(ctx, http.StatusUnauthorized, "invalid_refresh_token", "invalid refresh token")
		return
	}
	h.writeTokenResponse(ctx, pair)
}

func (h Handler) Logout(ctx *gin.Context) {
	refreshToken, _ := ctx.Cookie(h.authCfg.RefreshCookieName)
	if err := h.service.Logout(ctx.Request.Context(), refreshToken); err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "logout_failed", err.Error())
		return
	}
	h.clearRefreshCookie(ctx)
	httpx.JSONNoContent(ctx)
}

func (h Handler) Me(ctx *gin.Context) {
	userID, ok := ctx.Get(httpx.ContextUserID)
	if !ok {
		httpx.JSONError(ctx, http.StatusUnauthorized, "unauthorized", "missing user context")
		return
	}
	user, err := h.service.FindByID(ctx.Request.Context(), userID.(int))
	if err != nil {
		httpx.JSONError(ctx, http.StatusNotFound, "user_not_found", "user not found")
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"user": serializeUser(user)})
}

func (h Handler) ListUsers(ctx *gin.Context) {
	users, err := h.service.ListUsers(ctx.Request.Context())
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "list_users_failed", err.Error())
		return
	}
	items := make([]gin.H, 0, len(users))
	for _, user := range users {
		items = append(items, serializeUser(user))
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"items": items})
}

func (h Handler) CreateUser(ctx *gin.Context) {
	var req manageUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if req.Password == "" {
		httpx.JSONError(ctx, http.StatusBadRequest, "password_required", "password is required")
		return
	}
	user, err := h.service.CreateUser(ctx.Request.Context(), usersvc.ManageUserInput{
		Username:    req.Username,
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Status:      req.Status,
		RoleIDs:     req.RoleIDs,
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "create_user_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusCreated, gin.H{"user": serializeUser(user)})
}

func (h Handler) UpdateUser(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_user_id", "invalid user id")
		return
	}
	var req manageUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	user, err := h.service.UpdateUser(ctx.Request.Context(), id, usersvc.ManageUserInput{
		Username:    req.Username,
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Status:      req.Status,
		RoleIDs:     req.RoleIDs,
	})
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "update_user_failed", err.Error())
		return
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"user": serializeUser(user)})
}

func (h Handler) DeleteUser(ctx *gin.Context) {
	id, err := parseID(ctx)
	if err != nil {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid_user_id", "invalid user id")
		return
	}
	if err := h.service.DeleteUser(ctx.Request.Context(), id); err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "delete_user_failed", err.Error())
		return
	}
	httpx.JSONNoContent(ctx)
}

func (h Handler) ListRoles(ctx *gin.Context) {
	roles, err := h.service.ListRoles(ctx.Request.Context())
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, "list_roles_failed", err.Error())
		return
	}
	items := make([]gin.H, 0, len(roles))
	for _, role := range roles {
		items = append(items, gin.H{
			"id":          role.ID,
			"code":        role.Code,
			"name":        role.Name,
			"description": role.Description,
			"status":      role.Status,
		})
	}
	httpx.JSONOK(ctx, http.StatusOK, gin.H{"items": items})
}

func (h Handler) writeTokenResponse(ctx *gin.Context, pair *usersvc.TokenPair) {
	h.setRefreshCookie(ctx, pair.RefreshToken, pair.RefreshTokenExpiresAt)
	httpx.JSONOK(ctx, http.StatusOK, gin.H{
		"access_token": pair.AccessToken,
		"expires_at":   pair.AccessTokenExpiresAt.Format(time.RFC3339),
		"user":         serializeUser(pair.User),
		"roles":        pair.Roles,
	})
}

func (h Handler) setRefreshCookie(ctx *gin.Context, token string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	ctx.SetCookie(
		h.authCfg.RefreshCookieName,
		token,
		maxAge,
		"/v1/admin/auth",
		"",
		h.authCfg.RefreshCookieSecure,
		true,
	)
}

func (h Handler) clearRefreshCookie(ctx *gin.Context) {
	ctx.SetCookie(h.authCfg.RefreshCookieName, "", -1, "/v1/admin/auth", "", h.authCfg.RefreshCookieSecure, true)
}

func serializeUser(user *ent.User) gin.H {
	roles := make([]string, 0, len(user.Edges.Roles))
	for _, role := range user.Edges.Roles {
		roles = append(roles, role.Code)
	}
	return gin.H{
		"id":           user.ID,
		"username":     user.Username,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"status":       user.Status,
		"roles":        roles,
	}
}

func parseID(ctx *gin.Context) (int, error) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil || id <= 0 {
		return 0, err
	}
	return id, nil
}
