package fuego

import (
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func controller(c *ContextNoBody) (testStruct, error) {
	return testStruct{"Ewen", 23}, nil
}

func controllerWithError(c *ContextNoBody) (testStruct, error) {
	return testStruct{}, errors.New("error")
}

func TestNewServer(t *testing.T) {
	s := NewServer()

	t.Run("can register controller", func(t *testing.T) {
		Get(s, "/", controller)

		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)

		s.Mux.ServeHTTP(recorder, req)

		require.Equal(t, 200, recorder.Code)
	})
}

func TestWithXML(t *testing.T) {
	s := NewServer(
		WithXML(),
	)
	Get(s, "/", controller)
	Get(s, "/error", controllerWithError)

	t.Run("response is XML", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)

		s.Mux.ServeHTTP(recorder, req)

		require.Equal(t, 200, recorder.Code)
		require.Equal(t, "application/xml", recorder.Header().Get("Content-Type"))
		require.Equal(t, "<testStruct><Name>Ewen</Name><Age>23</Age></testStruct>", recorder.Body.String())
	})

	t.Run("error response is XML", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/error", nil)

		s.Mux.ServeHTTP(recorder, req)

		require.Equal(t, 500, recorder.Code)
		require.Equal(t, "application/xml", recorder.Header().Get("Content-Type"))
		require.Equal(t, "<HTTPError><Error>error</Error></HTTPError>", recorder.Body.String())
	})
}

func TestWithOpenAPIConfig(t *testing.T) {
	t.Run("with default values", func(t *testing.T) {
		s := NewServer(
			WithOpenapiConfig(OpenapiConfig{}),
		)

		require.Equal(t, "/swagger", s.OpenapiConfig.SwaggerUrl)
		require.Equal(t, "/swagger/openapi.json", s.OpenapiConfig.JsonUrl)
		require.Equal(t, "doc/openapi.json", s.OpenapiConfig.JsonFilePath)
	})

	t.Run("with custom values", func(t *testing.T) {
		s := NewServer(
			WithOpenapiConfig(OpenapiConfig{
				SwaggerUrl:       "/api",
				JsonUrl:          "/api/openapi.json",
				JsonFilePath:     "openapi.json",
				DisableSwagger:   true,
				DisableLocalSave: true,
			}),
		)

		require.Equal(t, "/api", s.OpenapiConfig.SwaggerUrl)
		require.Equal(t, "/api/openapi.json", s.OpenapiConfig.JsonUrl)
		require.Equal(t, "openapi.json", s.OpenapiConfig.JsonFilePath)
		require.True(t, s.OpenapiConfig.DisableSwagger)
		require.True(t, s.OpenapiConfig.DisableLocalSave)
	})

	t.Run("with invalid local path values", func(t *testing.T) {
		t.Run("with invalid path", func(t *testing.T) {
			NewServer(
				WithOpenapiConfig(OpenapiConfig{
					JsonFilePath: "path/to/jsonSpec",
					SwaggerUrl:   "p   i",
					JsonUrl:      "pi/op  enapi.json",
				}),
			)
		})
		t.Run("with invalid url", func(t *testing.T) {
			NewServer(
				WithOpenapiConfig(OpenapiConfig{
					JsonFilePath: "path/to/jsonSpec.json",
					JsonUrl:      "pi/op  enapi.json",
					SwaggerUrl:   "p   i",
				}),
			)
		})

		t.Run("with invalid url", func(t *testing.T) {
			NewServer(
				WithOpenapiConfig(OpenapiConfig{
					JsonFilePath: "path/to/jsonSpec.json",
					JsonUrl:      "/api/openapi.json",
					SwaggerUrl:   "invalid path",
				}),
			)
		})
	})
}

func TestWithBasePath(t *testing.T) {
	s := NewServer(
		WithBasePath("/api"),
	)

	require.Equal(t, "/api", s.basePath)
}

func TestWithMaxBodySize(t *testing.T) {
	s := NewServer(
		WithMaxBodySize(1024),
	)

	require.Equal(t, int64(1024), s.maxBodySize)
}

func TestWithAutoAuth(t *testing.T) {
	s := NewServer(
		WithAutoAuth(nil),
	)

	require.NotNil(t, s.autoAuth)
	require.True(t, s.autoAuth.Enabled)
	// The authoauth is tested in security_test.go,
	// this is just an option to enable it.
}

func TestWithTemplates(t *testing.T) {
	t.Run("with template FS", func(t *testing.T) {
		template := template.New("test")
		s := NewServer(
			WithTemplateFS(testdata),
			WithTemplates(template),
		)

		require.NotNil(t, s.template)
	})

	t.Run("without template FS", func(t *testing.T) {
		template := template.New("test")
		s := NewServer(
			WithTemplates(template),
		)

		require.NotNil(t, s.template)
	})
}

func TestWithLogHandler(t *testing.T) {
	handler := slog.NewTextHandler(io.Discard, nil)
	NewServer(
		WithLogHandler(handler),
	)
}

func TestWithValidator(t *testing.T) {
	type args struct {
		newValidator *validator.Validate
	}
	tests := []struct {
		name      string
		args      args
		wantPanic bool
	}{
		{
			name: "with custom validator",
			args: args{
				newValidator: validator.New(),
			},
		},
		{
			name: "no validator provided",
			args: args{
				newValidator: nil,
			},
			wantPanic: true,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				if tt.wantPanic {
					assert.Panics(
						t, func() { WithValidator(tt.args.newValidator) },
					)
				} else {
					NewServer(
						WithValidator(tt.args.newValidator),
					)
					assert.Equal(t, tt.args.newValidator, v)
				}
			},
		)
	}
}
