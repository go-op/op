package fuego

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
)

var _ OpenAPIServable = &Server{}

type Server struct {
	// The underlying HTTP server
	*http.Server

	// Will be plugged into the Server field.
	// Not using directly the Server field so
	// [http.ServeMux.Handle] can also be used to register routes.
	Mux *http.ServeMux

	// globalMiddlewares is used to store the options
	// that will be applied on ALL routes.
	globalMiddlewares []func(http.Handler) http.Handler

	*Engine

	listener net.Listener

	template *template.Template // TODO: use preparsed templates

	// Custom serializer that overrides the default one.
	Serialize Sender
	// Used to serialize the error response. Defaults to [SendError].
	SerializeError ErrorSender

	startTime time.Time

	Security Security

	autoAuth AutoAuthConfig
	fs       fs.FS

	// Base path of the group
	basePath string

	loggingConfig LoggingConfig

	// routeOptions is used to store the options
	// that will be applied of the route.
	routeOptions []func(*BaseRoute)

	middlewares []func(http.Handler) http.Handler

	maxBodySize int64
	// If true, the server will return an error if the request body contains unknown fields. Useful for quick debugging in development.
	DisallowUnknownFields  bool
	disableStartupMessages bool
	disableAutoGroupTags   bool
	isTLS                  bool
}

// NewServer creates a new server with the given options.
// Fuego's [Server] is built on top of the standard library's [http.Server].
// The OpenAPI and data flow is handled by the [Engine], a lightweight abstraction available for all kind of routers (net/http, Gin, Echo).
// For example:
//
//	app := fuego.NewServer(
//		fuego.WithAddr(":8080"),
//		fuego.WithoutLogger(),
//	)
//
// Options all begin with `With`.
// Some options are at engine level, and can be set with [WithEngineOptions].
// Some default options are set in the function body.
func NewServer(options ...func(*Server)) *Server {
	s := &Server{
		Server: &http.Server{
			ReadTimeout:       30 * time.Second,
			ReadHeaderTimeout: 30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       30 * time.Second,
		},
		Mux:    http.NewServeMux(),
		Engine: NewEngine(),

		Security: NewSecurity(),

		loggingConfig: defaultLoggingConfig,
	}

	// Default options that can be overridden
	defaultOptions := [...]func(*Server){
		WithAddr("localhost:9999"),
		WithDisallowUnknownFields(true),
		WithSerializer(Send),
		WithErrorSerializer(SendError),
		WithRouteOptions(
			OptionAddResponse(http.StatusBadRequest, "Bad Request _(validation or deserialization error)_", Response{Type: HTTPError{}}),
			OptionAddResponse(http.StatusInternalServerError, "Internal Server Error _(panics)_", Response{Type: HTTPError{}}),
		),
	}
	options = append(defaultOptions[:], options...)
	for _, option := range options {
		option(s)
	}

	s.startTime = time.Now()

	if s.autoAuth.Enabled {
		Post(s, "/auth/login", s.Security.LoginHandler(s.autoAuth.VerifyUserInfo),
			OptionTags("Auth"),
			OptionSummary("Login"),
		)
		PostStd(s, "/auth/logout", s.Security.CookieLogoutHandler,
			OptionTags("Auth"),
			OptionSummary("Logout"),
		)

		s.middlewares = []func(http.Handler) http.Handler{
			s.Security.TokenToContext(TokenFromCookie, TokenFromHeader),
		}

		PostStd(s, "/auth/refresh", s.Security.RefreshHandler,
			OptionTags("Auth"),
			OptionSummary("Refresh"),
		)
	}

	if !s.loggingConfig.Disabled() {
		s.middlewares = append(s.middlewares, newDefaultLogger(s).middleware)
	}

	return s
}

func (s *Server) SpecHandler(_ *Engine) {
	Get(s, s.OpenAPIConfig.SpecURL, s.Engine.SpecHandler(), OptionHide())
	s.printOpenAPIMessage(fmt.Sprintf("JSON spec: %s%s", s.url(), s.OpenAPIConfig.SpecURL))
}

func (s *Server) UIHandler(_ *Engine) {
	GetStd(s, s.OpenAPIConfig.SwaggerURL+"/", s.OpenAPIConfig.UIHandler(s.OpenAPIConfig.SpecURL).ServeHTTP, OptionHide())
	s.printOpenAPIMessage(fmt.Sprintf("OpenAPI UI: %s%s/index.html", s.url(), s.OpenAPIConfig.SwaggerURL))
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

// WithGlobalMiddlewares adds middleware(s) that will be executed on ALL requests,
// even those that don't match any registered routes.
// Global Middlewares are mounted on the [http.Server] Handler, when executing [Server.Run].
// Route Middlewares are mounted directly on [http.ServeMux] added at route registration.
//
// For example, to add CORS middleware:
//
//	import "github.com/rs/cors"
//
//	s := fuego.NewServer(
//		WithGlobalMiddlewares(cors.New(cors.Options{
//			AllowedOrigins:   []string{"*"},
//			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
//			AllowedHeaders:   []string{"*"},
//			AllowCredentials: true,
//		}).Handler),
//	)
func WithGlobalMiddlewares(middlewares ...func(http.Handler) http.Handler) func(*Server) {
	return func(c *Server) {
		c.globalMiddlewares = append(c.globalMiddlewares, middlewares...)
	}
}

// WithCorsMiddleware adds CORS middleware to the server.
//
// Deprecated: Please use [WithGlobalMiddlewares] instead.
func WithCorsMiddleware(corsMiddleware func(http.Handler) http.Handler) func(*Server) {
	return WithGlobalMiddlewares(corsMiddleware)
}

// WithGlobalResponseTypes adds default response types to the server.
// For example:
//
//	app := fuego.NewServer(
//		fuego.WithGlobalResponseTypes(400, "Bad Request _(validation or deserialization error)_", HTTPError{}),
//		fuego.WithGlobalResponseTypes(401, "Unauthorized _(authentication error)_", HTTPError{}),
//		fuego.WithGlobalResponseTypes(500, "Internal Server Error _(panics)_", HTTPError{}),
//		fuego.WithGlobalResponseTypes(204, "No Content", Empty{}),
//	)
//
// Deprecated: Please use [OptionAddResponse] with [WithRouteOptions]
func WithGlobalResponseTypes(code int, description string, response Response) func(*Server) {
	return func(c *Server) {
		WithRouteOptions(
			OptionAddResponse(code, description, response),
		)(c)
	}
}

// WithSecurity configures security schemes in the OpenAPI specification.
// It allows setting up authentication methods like JWT Bearer tokens, API keys, OAuth2, etc.
// For example:
//
//	app := fuego.NewServer(
//		fuego.WithSecurity(map[string]*openapi3.SecuritySchemeRef{
//			"bearerAuth": &openapi3.SecuritySchemeRef{
//				Value: openapi3.NewSecurityScheme().
//					WithType("http").
//					WithScheme("bearer").
//					WithBearerFormat("JWT").
//					WithDescription("Enter your JWT token in the format: Bearer <token>"),
//			},
//		}),
//	)
func WithSecurity(schemes openapi3.SecuritySchemes) func(*Server) {
	return func(s *Server) {
		if s.OpenAPI.Description().Components.SecuritySchemes == nil {
			s.OpenAPI.Description().Components.SecuritySchemes = openapi3.SecuritySchemes{}
		}
		for name, scheme := range schemes {
			s.OpenAPI.Description().Components.SecuritySchemes[name] = scheme
		}
	}
}

// WithoutAutoGroupTags disables the automatic grouping of routes by tags.
// By default, routes are tagged by group.
// For example:
//
//	recipeGroup := fuego.Group(s, "/recipes")
//	fuego.Get(recipeGroup, "/", func(ContextNoBody) (ans, error) {
//		return ans{}, nil
//	})
//
//	RecipeThis route will be tagged with "recipes" by default, but with this option, they will not be tagged.
func WithoutAutoGroupTags() func(*Server) {
	return func(c *Server) { c.disableAutoGroupTags = true }
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
// If the server filesystem is not set, it will use the OS filesystem, at folder "./templates".
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

// WithAddr optionally specifies the TCP address for the server to listen on, in the form "host:port".
// If not specified addr ':9999' will be used.
// If a listener is explicitly set using WithListener, the provided address will be ignored,
func WithAddr(addr string) func(*Server) {
	return func(c *Server) {
		c.Server.Addr = addr
	}
}

// WithXML sets the serializer to XML
//
// Deprecated: fuego supports automatic XML serialization when using the header "Accept: application/xml".
func WithXML() func(*Server) {
	return func(c *Server) {
		c.Serialize = SendXML
		c.SerializeError = SendXMLError
	}
}

// WithLogHandler sets the log handler of the server.
func WithLogHandler(handler slog.Handler) func(*Server) {
	return func(*Server) {
		if handler != nil {
			slog.SetDefault(slog.New(handler))
		}
	}
}

// WithSerializer sets a custom serializer of type Sender that overrides the default one.
// Please send a PR if you think the default serializer should be improved, instead of jumping to this option.
func WithSerializer(serializer Sender) func(*Server) {
	return func(c *Server) { c.Serialize = serializer }
}

// WithErrorSerializer sets a custom serializer of type ErrorSender that overrides the default one.
// Please send a PR if you think the default serializer should be improved, instead of jumping to this option.
func WithErrorSerializer(serializer ErrorSender) func(*Server) {
	return func(c *Server) { c.SerializeError = serializer }
}

// WithoutStartupMessages disables the startup message
func WithoutStartupMessages() func(*Server) {
	return func(c *Server) {
		c.disableStartupMessages = true
		c.OpenAPIConfig.DisableMessages = true
	}
}

// WithoutLogger disables the default logger.
func WithoutLogger() func(*Server) {
	return func(*Server) {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	}
}

// WithListener configures the server to use a custom listener.
// If a listener is provided using this option, any address specified with WithAddr will be ignored.
//
// Example:
//
//	listener, _ := net.Listen("tcp", ":8080")
//	server := NewServer(
//	    WithListener(listener),
//	    WithAddr(":9999"), // This will be ignored because WithListener takes precedence.
//	)
func WithListener(listener net.Listener) func(*Server) {
	return func(s *Server) {
		s.listener = listener
	}
}

// WithEngineOptions allows for setting of Engine options
//
//	app := fuego.NewServer(
//		fuego.WithAddr(":8080"),
//		fuego.WithEngineOptions(
//			WithOpenAPIConfig(
//				OpenAPIConfig{
//					PrettyFormatJSON: true,
//				},
//			),
//		),
//	)
//
// Engine Options all begin with `With`.
func WithEngineOptions(options ...func(*Engine)) func(*Server) {
	return func(s *Server) {
		for _, option := range options {
			option(s.Engine)
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

	return func(*Server) {
		v = newValidator
	}
}

func WithRouteOptions(options ...func(*BaseRoute)) func(*Server) {
	return func(s *Server) {
		s.routeOptions = append(s.routeOptions, options...)
	}
}

// WithLoggingMiddleware configures the default logging middleware for the server.
func WithLoggingMiddleware(loggingConfig LoggingConfig) func(*Server) {
	return func(s *Server) {
		s.loggingConfig.DisableRequest = loggingConfig.DisableRequest
		s.loggingConfig.DisableResponse = loggingConfig.DisableResponse
		if loggingConfig.RequestIDFunc != nil {
			s.loggingConfig.RequestIDFunc = loggingConfig.RequestIDFunc
		}
	}
}

// WithStripTrailingSlash enables stripping of trailing slashes from routes and requests
// Default is false.
//
// Example:
//
//	app := fuego.NewServer(
//		fuego.WithStripTrailingSlash(),
//	)
func WithStripTrailingSlash() func(*Server) {
	return func(s *Server) {
		// Add OptionStripTrailingSlash to the default route options
		s.routeOptions = append(s.routeOptions, OptionStripTrailingSlash())
	}
}
