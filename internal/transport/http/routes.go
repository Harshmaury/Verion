package http

import (
	"encoding/json"
	"net/http"

	"github.com/Harshmaury/verion/internal/identity"
)

// registerRoutes registers all REST routes.
// Protected routes are wrapped with RequireAuth middleware.
// Public routes are left unwrapped.
func (g *Gateway) registerRoutes(mux *http.ServeMux) {
	auth := RequireAuth(g.tokenSvc)

	// ── Public routes ─────────────────────────────────────────────────────────
	mux.HandleFunc("GET /healthz", g.handleHealthz)

	// WebAuthn — always public
	mux.HandleFunc("POST /v1/auth/register", g.wauthn.RegisterBegin)
	mux.HandleFunc("POST /v1/auth/register/complete", g.wauthn.RegisterComplete)
	mux.HandleFunc("POST /v1/auth/login", g.login.LoginBegin)
	mux.HandleFunc("POST /v1/auth/login/complete", g.login.LoginComplete)

	// Create tenant — public (admin auth added in Phase 4)
	mux.HandleFunc("POST /v1/tenants", g.handleCreateTenant)

	// ── Protected routes ──────────────────────────────────────────────────────
	mux.Handle("GET /v1/tenants/{id}", auth(http.HandlerFunc(g.handleGetTenant)))
	mux.Handle("POST /v1/tenants/{id}/suspend", auth(http.HandlerFunc(g.handleSuspendTenant)))
	mux.Handle("POST /v1/tenants/{id}/activate", auth(http.HandlerFunc(g.handleActivateTenant)))

	mux.Handle("POST /v1/identities", auth(http.HandlerFunc(g.handleCreateIdentity)))
	mux.Handle("GET /v1/identities/{id}", auth(http.HandlerFunc(g.handleGetIdentity)))
	mux.Handle("GET /v1/identities/handle/{handle}", auth(http.HandlerFunc(g.handleGetByHandle)))
	mux.Handle("GET /v1/identities", auth(http.HandlerFunc(g.handleListIdentities)))
	mux.Handle("PUT /v1/identities/{id}", auth(http.HandlerFunc(g.handleUpdateIdentity)))
	mux.Handle("POST /v1/identities/{id}/suspend", auth(http.HandlerFunc(g.handleSuspendIdentity)))
	mux.Handle("POST /v1/identities/{id}/reactivate", auth(http.HandlerFunc(g.handleReactivateIdentity)))
	mux.Handle("DELETE /v1/identities/{id}", auth(http.HandlerFunc(g.handleDeactivateIdentity)))

	mux.Handle("POST /v1/keys", auth(http.HandlerFunc(g.handleGenerateKey)))
	mux.Handle("GET /v1/keys/{key_id}", auth(http.HandlerFunc(g.handleGetKey)))
	mux.Handle("POST /v1/keys/{key_id}/rotate", auth(http.HandlerFunc(g.handleRotateKey)))
	mux.Handle("DELETE /v1/keys/{key_id}", auth(http.HandlerFunc(g.handleRevokeKey)))
}

// ── Health ────────────────────────────────────────────────────────────────────

func (g *Gateway) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": "0.2.0",
	})
}

// ── Tenant handlers ───────────────────────────────────────────────────────────

func (g *Gateway) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		Slug       string `json:"slug"`
		Tier       string `json:"tier"`
		DataRegion string `json:"data_region"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := g.tenantSvc.CreateTenant(r.Context(), identity.CreateTenantInput{
		Name:       req.Name,
		Slug:       req.Slug,
		Tier:       identity.TenantTier(req.Tier),
		DataRegion: req.DataRegion,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (g *Gateway) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	result, err := g.tenantSvc.GetTenant(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (g *Gateway) handleSuspendTenant(w http.ResponseWriter, r *http.Request) {
	if err := g.tenantSvc.SuspendTenant(r.Context(), r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "suspended"})
}

func (g *Gateway) handleActivateTenant(w http.ResponseWriter, r *http.Request) {
	if err := g.tenantSvc.ActivateTenant(r.Context(), r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "active"})
}

// ── Identity handlers ─────────────────────────────────────────────────────────

func (g *Gateway) handleCreateIdentity(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID    string         `json:"tenant_id"`
		Type        string         `json:"type"`
		DisplayName string         `json:"display_name"`
		Handle      string         `json:"handle"`
		CreatedBy   string         `json:"created_by"`
		Attributes  map[string]any `json:"attributes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	input := identity.CreateIdentityInput{
		TenantID:    req.TenantID,
		Type:        identity.IdentityType(req.Type),
		DisplayName: req.DisplayName,
		Handle:      req.Handle,
		Attributes:  req.Attributes,
	}
	if req.CreatedBy != "" {
		input.CreatedBy = &req.CreatedBy
	}
	result, err := g.identitySvc.CreateIdentity(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (g *Gateway) handleGetIdentity(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id query parameter required")
		return
	}
	result, err := g.identitySvc.GetIdentity(r.Context(), tenantID, r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (g *Gateway) handleGetByHandle(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id query parameter required")
		return
	}
	result, err := g.identitySvc.GetIdentityByHandle(r.Context(), tenantID, r.PathValue("handle"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (g *Gateway) handleListIdentities(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id query parameter required")
		return
	}
	results, err := g.identitySvc.ListIdentities(r.Context(), tenantID, identity.IdentityFilter{Limit: 50})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"identities": results, "total": len(results)})
}

func (g *Gateway) handleUpdateIdentity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		TenantID    string         `json:"tenant_id"`
		DisplayName string         `json:"display_name"`
		Attributes  map[string]any `json:"attributes"`
		Version     int64          `json:"version"`
		ActorID     string         `json:"actor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	input := identity.UpdateIdentityInput{
		TenantID:    req.TenantID,
		ID:          id,
		DisplayName: req.DisplayName,
		Attributes:  req.Attributes,
		Version:     req.Version,
	}
	if req.ActorID != "" {
		input.ActorID = &req.ActorID
	}
	result, err := g.identitySvc.UpdateIdentity(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (g *Gateway) handleSuspendIdentity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		TenantID string `json:"tenant_id"`
		ActorID  string `json:"actor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := g.identitySvc.SuspendIdentity(r.Context(), req.TenantID, id, req.ActorID); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "suspended"})
}

func (g *Gateway) handleReactivateIdentity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		TenantID string `json:"tenant_id"`
		ActorID  string `json:"actor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := g.identitySvc.ReactivateIdentity(r.Context(), req.TenantID, id, req.ActorID); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "active"})
}

func (g *Gateway) handleDeactivateIdentity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		TenantID string `json:"tenant_id"`
		ActorID  string `json:"actor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := g.identitySvc.DeactivateIdentity(r.Context(), req.TenantID, id, req.ActorID); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deactivated"})
}

// ── Key handlers ──────────────────────────────────────────────────────────────

func (g *Gateway) handleGenerateKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TenantID   string `json:"tenant_id"`
		IdentityID string `json:"identity_id"`
		KeyType    string `json:"key_type"`
		Purpose    string `json:"purpose"`
		ActorID    string `json:"actor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	input := identity.GenerateKeyInput{
		TenantID:   req.TenantID,
		IdentityID: req.IdentityID,
		KeyType:    identity.KeyType(req.KeyType),
		Purpose:    identity.KeyPurpose(req.Purpose),
	}
	if req.ActorID != "" {
		input.ActorID = &req.ActorID
	}
	result, err := g.keySvc.GenerateKey(r.Context(), input)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (g *Gateway) handleGetKey(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id query parameter required")
		return
	}
	result, err := g.keySvc.GetKey(r.Context(), tenantID, r.PathValue("key_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (g *Gateway) handleRotateKey(w http.ResponseWriter, r *http.Request) {
	keyID := r.PathValue("key_id")
	var req struct {
		TenantID   string `json:"tenant_id"`
		IdentityID string `json:"identity_id"`
		ActorID    string `json:"actor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := g.keySvc.RotateKey(r.Context(), req.TenantID, req.IdentityID, keyID, req.ActorID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (g *Gateway) handleRevokeKey(w http.ResponseWriter, r *http.Request) {
	keyID := r.PathValue("key_id")
	var req struct {
		TenantID string `json:"tenant_id"`
		ActorID  string `json:"actor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := g.keySvc.RevokeKey(r.Context(), req.TenantID, keyID, req.ActorID); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}
