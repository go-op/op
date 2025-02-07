package fuego

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// dummyMiddleware sets the X-Test header on the request and the X-Test-Response header on the response.
func dummyMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Test", "test")
		w.Header().Set("X-Test-Response", "response")
		handler.ServeHTTP(w, r)
	})
}

func TestUIHandler(t *testing.T) {
	t.Run("works with DefaultOpenAPIHandler", func(t *testing.T) {
		s := NewServer()

		s.Engine.RegisterOpenAPIRoutes(s)

		require.NotNil(t, s.OpenAPI.Config.UIHandler)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/swagger/index.html", nil)

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, 200, w.Code)
		require.Contains(t, w.Body.String(), "OpenAPI specification")
		require.Equal(t, "", w.Header().Get("X-Test-Response"))
	})

	t.Run("wrap DefaultOpenAPIHandler behind a middleware", func(t *testing.T) {
		s := NewServer(
			WithEngineOptions(
				WithOpenAPIConfig(OpenAPIConfig{
					UIHandler: func(specURL string) http.Handler {
						return dummyMiddleware(DefaultOpenAPIHandler(specURL))
					},
				}),
			),
		)
		s.Engine.RegisterOpenAPIRoutes(s)

		require.NotNil(t, s.OpenAPI.Config.UIHandler)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/swagger/index.html", nil)

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, 200, w.Code)
		require.Contains(t, w.Body.String(), "OpenAPI specification")
		require.Equal(t, "response", w.Header().Get("X-Test-Response"))
	})

	t.Run("disabling UI", func(t *testing.T) {
		s := NewServer(
			WithEngineOptions(
				WithOpenAPIConfig(OpenAPIConfig{
					DisableSwaggerUI: true,
				}),
			),
		)

		s.Engine.RegisterOpenAPIRoutes(s)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/swagger/index.html", nil)

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, 404, w.Code)
		require.Contains(t, w.Body.String(), "404 page not found")
		require.Equal(t, "", w.Header().Get("X-Test-Response"))
	})
}
