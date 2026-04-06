package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/magosproject/magos/api/internal/api/handlers"
	"github.com/magosproject/magos/api/internal/generated/clientset/versioned"
	"github.com/magosproject/magos/api/internal/service"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Server represents the HTTP API server.
type Server struct {
	logger             *slog.Logger
	projectHandler     *handlers.ProjectHandler
	workspaceHandler   *handlers.WorkspaceHandler
	rolloutHandler     *handlers.RolloutHandler
	variableSetHandler *handlers.VariableSetHandler
}

// NewServer creates a new API server with the given Kubernetes client.
func NewServer(logger *slog.Logger, vc versioned.Interface) *Server {
	projectSvc := service.NewProjectService(logger, vc)
	workspaceSvc := service.NewWorkspaceService(logger, vc)
	rolloutSvc := service.NewRolloutService(logger, vc)
	variableSetSvc := service.NewVariableSetService(logger, vc)
	return &Server{
		logger:             logger,
		projectHandler:     handlers.NewProjectHandler(logger, projectSvc),
		workspaceHandler:   handlers.NewWorkspaceHandler(logger, workspaceSvc),
		rolloutHandler:     handlers.NewRolloutHandler(logger, rolloutSvc),
		variableSetHandler: handlers.NewVariableSetHandler(logger, variableSetSvc),
	}
}

// NewServerWithDefaults creates a new server using in-cluster or kubeconfig-based client.
// It starts the project informer and waits for the cache to sync before returning.
func NewServerWithDefaults(logger *slog.Logger) (*Server, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := ""
		home, _ := os.LookupEnv("HOME")
		if kcEnv, ok := os.LookupEnv("KUBECONFIG"); ok && kcEnv != "" {
			kubeconfig = kcEnv
		} else if home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
		}
	}

	vc, err := versioned.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create versioned clientset: %w", err)
	}

	return NewServer(logger, vc), nil
}

// Router returns the HTTP handler with all routes configured.
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /healthz", s.healthCheck)
	mux.HandleFunc("GET /readyz", s.healthCheck)

	// Projects
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/projects", s.projectHandler.List)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/projects/events", s.projectHandler.Events)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/projects/{namespace}/{name}", s.projectHandler.Get)

	// Workspaces
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/workspaces", s.workspaceHandler.List)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/workspaces/events", s.workspaceHandler.Events)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}", s.workspaceHandler.Get)

	// Rollouts
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/rollouts", s.rolloutHandler.List)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/rollouts/{namespace}/{name}", s.rolloutHandler.Get)

	// VariableSets
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/variablesets", s.variableSetHandler.List)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/variablesets/{namespace}/{name}", s.variableSetHandler.Get)

	// Wrap with middleware
	var handler http.Handler = mux
	handler = s.loggingMiddleware(handler)
	handler = s.recoveryMiddleware(handler)

	return handler
}

// healthCheck returns a simple health check response.
func (s *Server) healthCheck(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// loggingMiddleware logs HTTP requests.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)
		next.ServeHTTP(w, r)
	})
}

// recoveryMiddleware recovers from panics.
func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				s.logger.Error("panic recovered", "error", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// best-effort; headers already sent
		_ = err
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
