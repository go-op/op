package fuego

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed testdata/*.html
var testdata embed.FS

func TestRender(t *testing.T) {
	s := NewServer(
		WithTemplateFS(testdata),
		WithTemplateGlobs("testdata/*.html"),
	)

	Get(s, "/test", func(ctx ContextNoBody) (CtxRenderer, error) {
		return ctx.Render("testdata/test.html", H{"Name": "test"})
	})

	t.Run("Render once", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, w.Body.String(), "<main>\n  <h1>Test</h1>\n  <p>Your name is: test</p>\n</main>\n")
	})

	t.Run("Render twice", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, w.Body.String(), "<main>\n  <h1>Test</h1>\n  <p>Your name is: test</p>\n</main>\n")
	})

	t.Run("cannot parse unexisting file", func(t *testing.T) {
		Get(s, "/file-not-found", func(ctx ContextNoBody) (CtxRenderer, error) {
			return ctx.Render("testdata/not-found.html", H{"Name": "test"})
		})

		r := httptest.NewRequest(http.MethodGet, "/file-not-found", nil)
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("can execute template with missing variable in map", func(t *testing.T) {
		Get(s, "/impossible", func(ctx ContextNoBody) (CtxRenderer, error) {
			return ctx.Render("testdata/test.html", H{"NotName": "test"})
		})

		r := httptest.NewRequest(http.MethodGet, "/impossible", nil)
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		t.Log(w.Body.String())

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("cannot execute template with missing variable in struct", func(t *testing.T) {
		Get(s, "/impossible-struct", func(ctx ContextNoBody) (CtxRenderer, error) {
			return ctx.Render("testdata/test.html", struct{}{})
		})

		r := httptest.NewRequest(http.MethodGet, "/impossible-struct", nil)
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		t.Log(w.Body.String())

		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, w.Body.String(), "error executing template")
	})
}

func BenchmarkRender(b *testing.B) {
	s := NewServer(
		WithTemplateFS(testdata),
		WithTemplateGlobs("testdata/*.html"),
	)

	Get(s, "/test", func(ctx ContextNoBody) (CtxRenderer, error) {
		return ctx.Render("testdata/test.html", H{"Name": "test"})
	})

	expected := "<main>\n  <h1>Test</h1>\n  <p>Your name is: test</p>\n</main>\n"

	for range b.N {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			b.Fail()
		}
		if w.Body.String() != expected {
			b.Fail()
		}
	}
}

func TestServer_loadTemplates(t *testing.T) {
	s := NewServer(
		WithTemplateFS(testdata),
	)

	t.Run("no template", func(t *testing.T) {
		err := s.loadTemplates()
		require.Error(t, err)
	})

	t.Run("template not found", func(t *testing.T) {
		err := s.loadTemplates("testdata/not-found.html")
		require.Error(t, err)

		err = s.loadTemplates("notfound")
		require.Error(t, err)
	})

	t.Run("template found", func(t *testing.T) {
		err := s.loadTemplates("testdata/test.html")
		require.NoError(t, err)
	})
}
