package fuego

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
)

type OpenAPIConfig struct {
	DisableSwagger   bool   // If true, the server will not serve the swagger ui nor the openapi json spec
	DisableLocalSave bool   // If true, the server will not save the openapi json spec locally
	SwaggerUrl       string // URL to serve the swagger ui
	JsonUrl          string // URL to serve the openapi json spec
	JsonFilePath     string // Local path to save the openapi json spec
}

var defaultOpenAPIConfig = OpenAPIConfig{
	SwaggerUrl:   "/swagger",
	JsonUrl:      "/swagger/openapi.json",
	JsonFilePath: "doc/openapi.json",
}

type Server struct {
	// The underlying http server
	*http.Server

	// Will be plugged into the Server field.
	// Not using directly the Server field so
	// [http.ServeMux.Handle] can also be used to register routes.
	Mux *http.ServeMux

	// Not stored with the oter middlewares because it is a special case :
	// it applies on routes that are not registered.
	// For example, it allows OPTIONS /foo even if it is not declared (only GET /foo is declared).
	corsMiddleware func(http.Handler) http.Handler

	middlewares []func(http.Handler) http.Handler

	basePath string

	OpenApiSpec openapi3.T                        // OpenAPI spec generated by the server
	UIHandler   func(specURL string) http.Handler // Handler to serve the openapi ui from spec url

	Security Security

	autoAuth AutoAuthConfig
	fs       fs.FS
	template *template.Template // TODO: use preparsed templates

	DisallowUnknownFields bool // If true, the server will return an error if the request body contains unknown fields. Useful for quick debugging in development.
	DisableOpenapi        bool // If true, the the routes within the server will not generate an openapi spec.
	maxBodySize           int64
	Serialize             func(w http.ResponseWriter, ans any)   // Used to serialize the response. Defaults to [SendJSON].
	SerializeError        func(w http.ResponseWriter, err error) // Used to serialize the error response. Defaults to [SendJSONError].
	ErrorHandler          func(err error) error                  // Used to transform any error into a unified error type structure with status code. Defaults to [ErrorHandler]
	startTime             time.Time

	OpenAPIConfig OpenAPIConfig
}

// NewServer creates a new server with the given options.
// For example:
//
//	app := fuego.NewServer(
//		fuego.WithAddr(":8080"),
//		fuego.WithoutLogger(),
//	)
//
// Option all begin with `With`.
// Some default options are set in the function body.
func NewServer(options ...func(*Server)) *Server {
	s := &Server{
		Server: &http.Server{
			ReadTimeout:       30 * time.Second,
			ReadHeaderTimeout: 30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       30 * time.Second,
		},
		Mux:         http.NewServeMux(),
		OpenApiSpec: NewOpenApiSpec(),

		OpenAPIConfig: defaultOpenAPIConfig,
		UIHandler:     defaultOpenAPIHandler,

		Security: NewSecurity(),
	}

	defaultOptions := [...]func(*Server){
		WithPort(9999),
		WithDisallowUnknownFields(true),
		WithSerializer(SendJSON),
		WithErrorSerializer(SendJSONError),
		WithErrorHandler(ErrorHandler),
	}

	for _, option := range append(defaultOptions[:], options...) {
		option(s)
	}

	s.startTime = time.Now()

	if s.autoAuth.Enabled {
		Post(s, "/auth/login", s.Security.LoginHandler(s.autoAuth.VerifyUserInfo)).Tags("Auth").Summary("Login")
		PostStd(s, "/auth/logout", s.Security.CookieLogoutHandler).Tags("Auth").Summary("Logout")

		s.middlewares = []func(http.Handler) http.Handler{
			s.Security.TokenToContext(TokenFromCookie, TokenFromHeader),
		}

		PostStd(s, "/auth/refresh", s.Security.RefreshHandler).Tags("Auth").Summary("Refresh token")
	}

	return s
}

// WithTemplateFS sets the filesystem used to load templates.
// To be used with [WithTemplateGlobs] or [WithTemplates].
// For example:
//
//	WithTemplateFS(os.DirFS("./templates"))
//
// or with embedded templates:
//
//	//go:embed templates
//	var templates embed.FS
//	...
//	WithTemplateFS(templates)
func WithTemplateFS(fs fs.FS) func(*Server) {
	return func(c *Server) { c.fs = fs }
}

// WithCorsMiddleware registers a middleware to handle CORS.
// It is not handled like other middlewares with [Use] because it applies routes that are not registered.
// For example:
//
//	import "github.com/rs/cors"
//
//	s := fuego.NewServer(
//		WithCorsMiddleware(cors.New(cors.Options{
//			AllowedOrigins:   []string{"*"},
//			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
//			AllowedHeaders:   []string{"*"},
//			AllowCredentials: true,
//		}))
//	)
func WithCorsMiddleware(corsMiddleware func(http.Handler) http.Handler) func(*Server) {
	return func(c *Server) { c.corsMiddleware = corsMiddleware }
}

// WithTemplates loads the templates used to render HTML.
// To be used with [WithTemplateFS]. If not set, it will use the os filesystem, at folder "./templates".
func WithTemplates(templates *template.Template) func(*Server) {
	return func(s *Server) {
		if s.fs == nil {
			s.fs = os.DirFS("./templates")
			slog.Warn("No template filesystem set. Using os filesystem at './templates'.")
		}
		s.template = templates

		slog.Debug("Loaded templates", "templates", s.template.DefinedTemplates())
	}
}

// WithTemplateGlobs loads templates matching the given patterns from the server filesystem.
// If the server filesystem is not set, it will use the os filesystem, at folder "./templates".
// For example:
//
//	WithTemplateGlobs("*.html, */*.html", "*/*/*.html")
//	WithTemplateGlobs("pages/*.html", "pages/admin/*.html")
//
// for reference about the glob patterns in Go (no ** support for example): https://pkg.go.dev/path/filepath?utm_source=godoc#Match
func WithTemplateGlobs(patterns ...string) func(*Server) {
	return func(s *Server) {
		if s.fs == nil {
			s.fs = os.DirFS("./templates")
			slog.Warn("No template filesystem set. Using os filesystem at './templates'.")
		}
		err := s.loadTemplates(patterns...)
		if err != nil {
			slog.Error("Error loading templates", "error", err)
			panic(err)
		}

		slog.Debug("Loaded templates", "templates", s.template.DefinedTemplates())
	}
}

func WithBasePath(basePath string) func(*Server) {
	return func(c *Server) { c.basePath = basePath }
}

func WithMaxBodySize(maxBodySize int64) func(*Server) {
	return func(c *Server) { c.maxBodySize = maxBodySize }
}

func WithAutoAuth(verifyUserInfo func(user, password string) (jwt.Claims, error)) func(*Server) {
	return func(c *Server) {
		c.autoAuth.Enabled = true
		c.autoAuth.VerifyUserInfo = verifyUserInfo
	}
}

// WithDisallowUnknownFields sets the DisallowUnknownFields option.
// If true, the server will return an error if the request body contains unknown fields.
// Useful for quick debugging in development.
// Defaults to true.
func WithDisallowUnknownFields(b bool) func(*Server) {
	return func(c *Server) { c.DisallowUnknownFields = b }
}

// WithPort sets the port of the server. For example, 8080.
// If not specified, the default port is 9999.
// If you want to use a different address, use [WithAddr] instead.
func WithPort(port int) func(*Server) {
	return func(s *Server) { s.Server.Addr = fmt.Sprintf("localhost:%d", port) }
}

// WithAddr optionally specifies the TCP address for the server to listen on, in the form "host:port".
// If not specified addr ':9999' will be used.
func WithAddr(addr string) func(*Server) {
	return func(c *Server) { c.Server.Addr = addr }
}

func WithXML() func(*Server) {
	return func(c *Server) {
		c.Serialize = SendXML
		c.SerializeError = SendXMLError
	}
}

// WithLogHandler sets the log handler of the server.
func WithLogHandler(handler slog.Handler) func(*Server) {
	return func(c *Server) {
		if handler != nil {
			slog.SetDefault(slog.New(handler))
		}
	}
}

func WithSerializer(serializer func(w http.ResponseWriter, ans any)) func(*Server) {
	return func(c *Server) { c.Serialize = serializer }
}

func WithErrorSerializer(serializer func(w http.ResponseWriter, err error)) func(*Server) {
	return func(c *Server) { c.SerializeError = serializer }
}

func WithErrorHandler(errorHandler func(err error) error) func(*Server) {
	return func(c *Server) { c.ErrorHandler = errorHandler }
}

// WithoutLogger disables the default logger.
func WithoutLogger() func(*Server) {
	return func(c *Server) {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	}
}

func WithOpenAPIConfig(openapiConfig OpenAPIConfig) func(*Server) {
	return func(s *Server) {
		s.OpenAPIConfig = openapiConfig

		if s.OpenAPIConfig.JsonUrl == "" {
			s.OpenAPIConfig.JsonUrl = defaultOpenAPIConfig.JsonUrl
		}

		if s.OpenAPIConfig.SwaggerUrl == "" {
			s.OpenAPIConfig.SwaggerUrl = defaultOpenAPIConfig.SwaggerUrl
		}

		if s.OpenAPIConfig.JsonFilePath == "" {
			s.OpenAPIConfig.JsonFilePath = defaultOpenAPIConfig.JsonFilePath
		}

		if !validateJsonSpecLocalPath(s.OpenAPIConfig.JsonFilePath) {
			slog.Error("Error writing json spec. Value of 'jsonSpecLocalPath' option is not valid", "file", s.OpenAPIConfig.JsonFilePath)
			return
		}

		if !validateJsonSpecUrl(s.OpenAPIConfig.JsonUrl) {
			slog.Error("Error serving openapi json spec. Value of 's.OpenAPIConfig.JsonSpecUrl' option is not valid", "url", s.OpenAPIConfig.JsonUrl)
			return
		}

		if !validateSwaggerUrl(s.OpenAPIConfig.SwaggerUrl) {
			slog.Error("Error serving swagger ui. Value of 's.OpenAPIConfig.SwaggerUrl' option is not valid", "url", s.OpenAPIConfig.SwaggerUrl)
			return
		}
	}
}

// WithValidator sets the validator to be used by the fuego server.
// If no validator is provided, a default validator will be used.
//
// Note: If you are using the default validator, you can add tags to your structs using the `validate` tag.
// For example:
//
//	type MyStruct struct {
//		Field1 string `validate:"required"`
//		Field2 int    `validate:"min=10,max=20"`
//	}
//
// The above struct will be validated using the default validator, and if any errors occur, they will be returned as part of the response.
func WithValidator(newValidator *validator.Validate) func(*Server) {
	if newValidator == nil {
		panic("new validator not provided")
	}

	return func(s *Server) {
		v = newValidator
	}
}
