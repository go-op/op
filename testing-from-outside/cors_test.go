package testingfromoutside_test

import (
	"net/http/httptest"
	"testing"

	"github.com/rs/cors"
	"github.com/stretchr/testify/require"

	"github.com/go-fuego/fuego"
)

func TestCors(t *testing.T) {
	s := fuego.NewServer(
		fuego.WithoutLogger(),
		fuego.WithGlobalMiddlewares(cors.New(cors.Options{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET"},
		}).Handler),
	)

	fuego.Get(s, "/", func(c fuego.ContextNoBody) (string, error) {
		return "Hello, World!", nil
	})

	t.Run("CORS request INCOMPLETE TEST", func(t *testing.T) {
		r := httptest.NewRequest("GET", "http://example.com/", nil)
		w := httptest.NewRecorder()

		r.Header.Set("Origin", "http://example.com/")
		r.Header.Set("Access-Control-Request-Method", "GET")

		s.Mux.ServeHTTP(w, r)

		t.Log(w.Header())
		body := w.Body.String()
		require.Equal(t, "Hello, World!", body)
		require.Equal(t, 200, w.Code)
	})
}
