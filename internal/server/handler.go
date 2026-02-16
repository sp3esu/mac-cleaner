package server

import (
	"context"
	"fmt"
)

// Handler dispatches NDJSON requests to method-specific handlers.
type Handler struct {
	server *Server
}

// NewHandler creates a new handler bound to the given server.
func NewHandler(s *Server) *Handler {
	return &Handler{server: s}
}

// Dispatch routes a request to the appropriate handler method.
func (h *Handler) Dispatch(ctx context.Context, req Request, w *NDJSONWriter) {
	switch req.Method {
	case MethodPing:
		h.handlePing(req, w)
	case MethodScan:
		h.handleScan(ctx, req, w)
	case MethodCleanup:
		h.handleCleanup(ctx, req, w)
	case MethodCategories:
		h.handleCategories(req, w)
	default:
		_ = w.WriteErrorMsg(req.ID, fmt.Sprintf("unknown method: %s", req.Method))
	}
}

// handlePing responds with the server version.
func (h *Handler) handlePing(req Request, w *NDJSONWriter) {
	_ = w.WriteResult(req.ID, PingResult{
		Status:  "ok",
		Version: h.server.version,
	})
}
