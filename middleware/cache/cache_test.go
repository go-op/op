package cache

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"
)

type testStruct struct {
	Name string
	Age  int
}

const waitTime = 10 * time.Millisecond

func baseController(c fuego.ContextNoBody) (testStruct, error) {
	time.Sleep(waitTime)
	return testStruct{Name: "test", Age: 10}, nil
}

func TestCache(t *testing.T) {
	t.Run("cache with several configs panics", func(t *testing.T) {
		s := fuego.NewServer()

		require.Panics(t, func() {
			fuego.Use(s, New(Config{}, Config{}))
		})
	})

	t.Run("cache with custom Key", func(t *testing.T) {
		s := fuego.NewServer()
		fuego.Use(s, New(Config{
			Key: func(r *http.Request) string { return "custom_key" },
		}))
		fuego.Get(s, "/with-cache", baseController)
	})

	t.Run("cache with base config", func(t *testing.T) {
		s := fuego.NewServer()

		fuego.Get(s, "/without-cache", baseController)

		fuego.Use(s, New(Config{}))

		fuego.Get(s, "/with-cache", baseController)
		fuego.Post(s, "/cant-be-cached-because-not-get", baseController)

		t.Run("Answer once", func(t *testing.T) {
			r := httptest.NewRequest("GET", "/without-cache", nil)
			w := httptest.NewRecorder()

			s.Mux.ServeHTTP(w, r)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, w.Body.String(), `{"Name":"test","Age":10}`+"\n")
		})

		t.Run("Answer twice without cache", func(t *testing.T) {
			r := httptest.NewRequest("GET", "/without-cache", nil)
			w := httptest.NewRecorder()

			s.Mux.ServeHTTP(w, r)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, w.Body.String(), `{"Name":"test","Age":10}`+"\n")

			r = httptest.NewRequest("GET", "/without-cache", nil)
			w = httptest.NewRecorder()

			start := time.Now()
			s.Mux.ServeHTTP(w, r)
			elapsed := time.Since(start)
			require.True(t, elapsed >= waitTime)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, w.Body.String(), `{"Name":"test","Age":10}`+"\n")
		})

		t.Run("Do not use cache when Cache-Control: no-cache", func(t *testing.T) {
			r := httptest.NewRequest("GET", "/with-cache", nil)
			w := httptest.NewRecorder()

			s.Mux.ServeHTTP(w, r)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, w.Body.String(), `{"Name":"test","Age":10}`+"\n")

			// The response is stored but we will not use it
			r = httptest.NewRequest("GET", "/with-cache", nil)
			r.Header.Set("Cache-Control", "no-cache")
			w = httptest.NewRecorder()

			start := time.Now()
			s.Mux.ServeHTTP(w, r)
			elapsed := time.Since(start)
			require.True(t, elapsed >= waitTime)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, w.Body.String(), `{"Name":"test","Age":10}`+"\n")
		})

		t.Run("Do not store to cache when Cache-Control: no-store", func(t *testing.T) {
			s := fuego.NewServer()
			fuego.Get(s, "/with-cache", baseController, option.Middleware(New(Config{})))

			r := httptest.NewRequest("GET", "/with-cache", nil)
			r.Header.Set("Cache-Control", "no-store") // The response will not be stored
			w := httptest.NewRecorder()

			s.Mux.ServeHTTP(w, r)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, w.Body.String(), `{"Name":"test","Age":10}`+"\n")

			// The response is not stored, so the answer is slow
			r = httptest.NewRequest("GET", "/with-cache", nil)
			r.Header.Set("Cache-Control", "no-store")
			w = httptest.NewRecorder()

			start := time.Now()
			s.Mux.ServeHTTP(w, r)
			elapsed := time.Since(start)
			require.True(t, elapsed >= waitTime)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, w.Body.String(), `{"Name":"test","Age":10}`+"\n")

			// Lets's try again without the no-store header
			r = httptest.NewRequest("GET", "/with-cache", nil)
			w = httptest.NewRecorder()

			start = time.Now()
			s.Mux.ServeHTTP(w, r)
			elapsed = time.Since(start)
			require.True(t, elapsed >= waitTime)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, w.Body.String(), `{"Name":"test","Age":10}`+"\n")

			// The response is stored, so the answer will be fast event with the no-store header
			r = httptest.NewRequest("GET", "/with-cache", nil)
			r.Header.Set("Cache-Control", "no-store")
			w = httptest.NewRecorder()

			start = time.Now()
			s.Mux.ServeHTTP(w, r)
			elapsed = time.Since(start)
			require.True(t, elapsed < waitTime)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, w.Body.String(), `{"Name":"test","Age":10}`+"\n")
		})

		t.Run("Answer twice the same result with cache", func(t *testing.T) {
			r := httptest.NewRequest("GET", "/with-cache", nil)
			w := httptest.NewRecorder()

			s.Mux.ServeHTTP(w, r)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, w.Body.String(), `{"Name":"test","Age":10}`+"\n")

			r = httptest.NewRequest("GET", "/with-cache", nil)
			w = httptest.NewRecorder()

			start := time.Now()
			s.Mux.ServeHTTP(w, r)
			elapsed := time.Since(start)
			require.True(t, elapsed < waitTime)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, w.Body.String(), `{"Name":"test","Age":10}`+"\n")
		})

		t.Run("Cannot cache non GET requests", func(t *testing.T) {
			r := httptest.NewRequest("POST", "/cant-be-cached-because-not-get", nil)
			w := httptest.NewRecorder()

			s.Mux.ServeHTTP(w, r)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, w.Body.String(), `{"Name":"test","Age":10}`+"\n")

			r = httptest.NewRequest("POST", "/cant-be-cached-because-not-get", nil)
			w = httptest.NewRecorder()

			start := time.Now()
			s.Mux.ServeHTTP(w, r)
			elapsed := time.Since(start)
			require.True(t, elapsed >= waitTime)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, w.Body.String(), `{"Name":"test","Age":10}`+"\n")
		})
	})
}

func BenchmarkCache(b *testing.B) {
	s := fuego.NewServer()

	fuego.Get(s, "/without-cache", baseController)

	fuego.Use(s, New(Config{}))

	fuego.Get(s, "/with-cache", baseController)

	b.Run("without cache", func(b *testing.B) {
		for range b.N {
			r := httptest.NewRequest("GET", "/without-cache", nil)
			w := httptest.NewRecorder()

			s.Mux.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				b.Fail()
			}
			require.Equal(b, w.Body.String(), `{"Name":"test","Age":10}`+"\n")
		}
	})

	b.Run("with cache", func(b *testing.B) {
		for range b.N {
			r := httptest.NewRequest("GET", "/with-cache", nil)
			w := httptest.NewRecorder()

			s.Mux.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				b.Fail()
			}
			require.Equal(b, w.Body.String(), `{"Name":"test","Age":10}`+"\n")
		}
	})
}
