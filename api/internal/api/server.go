package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/magosproject/magos/api/internal/api/handlers"
	"github.com/magosproject/magos/api/internal/generated/clientset/versioned"
	"github.com/magosproject/magos/api/internal/generated/informers/externalversions"
	"github.com/magosproject/magos/api/internal/service"
	"github.com/magosproject/magos/internal/logstore"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

//go:embed docs/swagger.json
var openapiSpec []byte

//go:embed docs/swagger-ui.html
var swaggerUI []byte

// Server represents the HTTP API server.
type Server struct {
	logger             *slog.Logger
	projectHandler     *handlers.ProjectHandler
	workspaceHandler   *handlers.WorkspaceHandler
	rolloutHandler     *handlers.RolloutHandler
	variableSetHandler *handlers.VariableSetHandler
}

// NewServer creates a new API server with the given Kubernetes client.
func NewServer(logger *slog.Logger, vc versioned.Interface, kube kubernetes.Interface, logs logstore.Store) *Server {
	factory := externalversions.NewSharedInformerFactory(vc, 5*time.Minute)

	projectSvc := service.NewProjectService(logger, factory)
	workspaceSvc := service.NewWorkspaceService(logger, factory, vc, kube, logs)
	rolloutSvc := service.NewRolloutService(logger, factory)
	variableSetSvc := service.NewVariableSetService(logger, factory)

	factory.Start(context.Background().Done())

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
	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	logs, err := logstore.NewStore(context.Background(), logstore.LoadConfigFromEnv())
	if err != nil {
		return nil, fmt.Errorf("failed to create log store: %w", err)
	}

	return NewServer(logger, vc, kube, logs), nil
}

// Router returns the HTTP handler with all routes configured.
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /healthz", s.healthCheck)
	mux.HandleFunc("GET /readyz", s.healthCheck)

	// OpenAPI spec and docs UI
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(openapiSpec)
	})
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(swaggerUI)
	})

	// Projects
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/projects", s.projectHandler.List)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/projects/events", s.projectHandler.Events)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/projects/{namespace}/{name}", s.projectHandler.Get)

	// Workspaces
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/workspaces", s.workspaceHandler.List)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/workspaces/events", s.workspaceHandler.Events)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}", s.workspaceHandler.Get)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}/runs", s.workspaceHandler.ListRuns)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}/runs/{runID}/log", s.workspaceHandler.GetRunLog)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}/runs/current/log/stream", s.workspaceHandler.StreamCurrentRunLog)
	mux.HandleFunc("POST /apis/magosproject.io/v1alpha1/workspaces/{namespace}/{name}/reconcile", s.workspaceHandler.RequestReconcile)

	// Rollouts
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/rollouts", s.rolloutHandler.List)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/rollouts/events", s.rolloutHandler.Events)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/rollouts/{namespace}/{name}", s.rolloutHandler.Get)

	// VariableSets
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/variablesets", s.variableSetHandler.List)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/variablesets/events", s.variableSetHandler.Events)
	mux.HandleFunc("GET /apis/magosproject.io/v1alpha1/variablesets/{namespace}/{name}", s.variableSetHandler.Get)

	// Wrap with middleware
	var handler http.Handler = mux
	handler = s.loggingMiddleware(handler)
	handler = s.recoveryMiddleware(handler)
	handler = s.corsMiddleware(handler)

	return handler
}

// healthCheck godoc
//
//	@Summary	Health check
//	@Tags		Health
//	@Produce	json
//	@Success	200	{object}	map[string]string
//	@Router		/healthz [get]
//	@Router		/readyz [get]
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

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO(anyone): figure out if it is a security concern to allow all origins
		// my first intuition says it's fine for now especially since we're not storing any cookies/sessions in
		// the browser and this will never run on a public domain anyways?
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		_ = err
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
