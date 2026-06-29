package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCORSMiddlewarePreflightAllowsRegisteredMethods(t *testing.T) {
	server := &Server{}
	handler := server.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/apis/magosproject.io/v1alpha1/workspaces/default/demo", nil)
	req.Header.Set("Origin", "https://example.test")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNoContent, resp.Code)
	methods := strings.Split(resp.Header().Get("Access-Control-Allow-Methods"), ", ")
	require.Contains(t, methods, http.MethodHead)
	require.Contains(t, methods, http.MethodPatch)
}
