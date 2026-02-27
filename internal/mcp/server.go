package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/remote-file-manager/server/internal/devices"
	"github.com/remote-file-manager/server/internal/objects"
	"github.com/remote-file-manager/server/internal/rpc"
	"github.com/remote-file-manager/server/pkg/config"
	"go.uber.org/zap"
)

// ToolHandler is a function that handles a tool call
type ToolHandler func(ctx context.Context, session *Session, params json.RawMessage) (interface{}, error)

// Server represents the MCP server
type Server struct {
	registry       *devices.Registry
	rpcManager     *rpc.Manager
	storage        *objects.Storage
	tokenManager   *objects.TokenManager
	sessionManager *SessionManager
	config         config.MCPConfig
	logger         *zap.Logger
	tools          map[string]ToolHandler
	serverAddr     string
}

// NewServer creates a new MCP server
func NewServer(
	registry *devices.Registry,
	rpcManager *rpc.Manager,
	storage *objects.Storage,
	tokenManager *objects.TokenManager,
	cfg config.MCPConfig,
	logger *zap.Logger,
	serverAddr string,
) *Server {
	s := &Server{
		registry:       registry,
		rpcManager:     rpcManager,
		storage:        storage,
		tokenManager:   tokenManager,
		sessionManager: NewSessionManager(cfg.SessionTimeout()),
		config:         cfg,
		logger:         logger,
		tools:          make(map[string]ToolHandler),
		serverAddr:     serverAddr,
	}

	// Register tools
	s.registerTools()

	return s
}

// RegisterRoutes registers MCP HTTP routes
func (s *Server) RegisterRoutes(router *mux.Router) {
	// Apply auth middleware to all MCP routes
	mcpRouter := router.PathPrefix(s.config.Endpoint).Subrouter()
	mcpRouter.Use(s.authMiddleware)

	// POST /mcp/messages - receive JSON-RPC requests
	mcpRouter.HandleFunc("/messages", s.handleMessages).Methods("POST")

	// GET /mcp/sse - establish SSE connection for responses
	mcpRouter.HandleFunc("/sse", s.handleSSE).Methods("GET")
}

// authMiddleware validates Bearer token
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.writeError(w, nil, ErrUnauthorized())
			return
		}

		// Check Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			s.writeError(w, nil, ErrUnauthorized())
			return
		}

		token := parts[1]
		if token != s.config.AuthToken {
			s.writeError(w, nil, ErrUnauthorized())
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleMessages handles JSON-RPC 2.0 requests
func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	// Get or create session
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	session := s.sessionManager.GetOrCreate(sessionID)

	// Parse request
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, nil, NewMCPError(ErrorCodeParseError, "Failed to parse request", nil))
		return
	}

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		s.writeError(w, req.ID, NewMCPError(ErrorCodeInvalidRequest, "Invalid JSON-RPC version", nil))
		return
	}

	// Find tool handler
	handler, exists := s.tools[req.Method]
	if !exists {
		s.writeError(w, req.ID, NewMCPError(ErrorCodeMethodNotFound, fmt.Sprintf("Method not found: %s", req.Method), nil))
		return
	}

	// Execute tool
	result, err := handler(r.Context(), session, req.Params)
	if err != nil {
		if mcpErr, ok := err.(*MCPError); ok {
			s.writeError(w, req.ID, mcpErr)
		} else {
			s.writeError(w, req.ID, NewMCPError(ErrorCodeInternalError, err.Error(), nil))
		}
		return
	}

	// Send response
	resp := Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Session-ID", sessionID)
	json.NewEncoder(w).Encode(resp)
}

// handleSSE handles Server-Sent Events connection
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Get session ID
	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		http.Error(w, "Missing X-Session-ID header", http.StatusBadRequest)
		return
	}

	session, exists := s.sessionManager.Get(sessionID)
	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send events
	for {
		select {
		case msg, ok := <-session.ResponseChan:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// writeError writes a JSON-RPC error response
func (s *Server) writeError(w http.ResponseWriter, id interface{}, err *MCPError) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   err,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors use 200 OK
	json.NewEncoder(w).Encode(resp)
}

// RegisterTool registers a tool handler
func (s *Server) RegisterTool(name string, handler ToolHandler) {
	s.tools[name] = handler
}
