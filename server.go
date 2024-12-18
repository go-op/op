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
	DisableSwagger   bool                              // If true, the server will not serve the Swagger UI nor the OpenAPI JSON spec
	DisableSwaggerUI bool                              // If true, the server will not serve the Swagger UI
	DisableLocalSave bool                              // If true, the server will not save the OpenAPI JSON spec locally
	SwaggerUrl       string                            // URL to serve the swagger UI
	UIHandler        func(specURL string) http.Handler // Handler to serve the OpenAPI UI from spec URL
	JsonUrl          string                            // URL to serve the OpenAPI JSON spec
	JsonFilePath     string                            // Local path to save the OpenAPI JSON spec
	PrettyFormatJson bool                              // Pretty prints the OpenAPI spec with proper JSON indentation
}

var defaultOpenAPIConfig = OpenAPIConfig{
	SwaggerUrl:   "/swagger",
	JsonUrl:      "/swagger/openapi.json",
	JsonFilePath: "doc/openapi.json",
	UIHandler:    DefaultOpenAPIHandler,
}

type Server struct {
	// The underlying HTTP server
	*http.Server

	// Will be plugged into the Server field.
	// Not using directly the Server field so
	// [http.ServeMux.Handle] can also be used to register routes.
	Mux *http.ServeMux

	// Not stored with the other middlewares because it is a special case :
	// it applies on routes that are not registered.
	// For example, it allows OPTIONS /foo even if it is not declared (only GET /foo is declared).
	corsMiddleware func(http.Handler) http.Handler

	// routeOptions is used to store the options
	// that will be applied of the route.
	routeOptions []func(*BaseRoute)

	middlewares []func(http.Handler) http.Handler

	disableStartupMessages bool
	disableAutoGroupTags   bool
	basePath               string // Base path of the group

	// Points to the server OpenAPI struct.
	OpenAPI *OpenAPI

	Security Security

	autoAuth AutoAuthConfig
	fs       fs.FS
	template *template.Template // TODO: use preparsed templates

	acceptedContentTypes []string

	DisallowUnknownFields bool // If true, the server will return an error if the request body contains unknown fields. Useful for quick debugging in development.
	DisableOpenapi        bool // If true, the routes within the server will not generate an OpenAPI spec.
	maxBodySize           int64

	Serialize      Sender                // Custom serializer that overrides the default one.
	SerializeError ErrorSender           // Used to serialize the error response. Defaults to [SendError].
	ErrorHandler   func(err error) error // Used to transform any error into a unified error type structure with status code. Defaults to [ErrorHandler]
	startTime      time.Time

	OpenAPIConfig OpenAPIConfig

	isTLS bool
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
		Mux:     http.NewServeMux(),
		OpenAPI: NewOpenAPI(),

		OpenAPIConfig: defaultOpenAPIConfig,

		Security: NewSecurity(),
	}

	// Default options that can be overridden
	defaultOptions := [...]func(*Server){
		WithAddr("localhost:9999"),
		WithDisallowUnknownFields(true),
		WithSerializer(Send),
		WithErrorSerializer(SendError),
		WithErrorHandler(ErrorHandler),
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
//		}).Handler)
//	)
func WithCorsMiddleware(corsMiddleware func(http.Handler) http.Handler) func(*Server) {
	return func(c *Server) { c.corsMiddleware = corsMiddleware }
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

// WithPort sets the port of the server. For example, 8080.
// If not specified, the default port is 9999.
// If you want to use a different address, use [WithAddr] instead.
//
// Deprecated: Please use [WithAddr]
func WithPort(port int) func(*Server) {
	return func(s *Server) { s.Server.Addr = fmt.Sprintf("localhost:%d", port) }
}

// WithAddr optionally specifies the TCP address for the server to listen on, in the form "host:port".
// If not specified addr ':9999' will be used.
func WithAddr(addr string) func(*Server) {
	return func(c *Server) { c.Server.Addr = addr }
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
	return func(c *Server) {
		if handler != nil {
			slog.SetDefault(slog.New(handler))
		}
	}
}

// WithRequestContentType sets the accepted content types for the server.
// By default, the accepted content types is */*.
func WithRequestContentType(consumes ...string) func(*Server) {
	return func(s *Server) { s.acceptedContentTypes = consumes }
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

// WithErrorHandler sets a customer error handler for the server
func WithErrorHandler(errorHandler func(err error) error) func(*Server) {
	return func(c *Server) { c.ErrorHandler = errorHandler }
}

// WithoutStartupMessages disables the startup message
func WithoutStartupMessages() func(*Server) {
	return func(c *Server) { c.disableStartupMessages = true }
}

// WithoutLogger disables the default logger.
func WithoutLogger() func(*Server) {
	return func(c *Server) {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	}
}

func WithOpenAPIConfig(openapiConfig OpenAPIConfig) func(*Server) {
	return func(s *Server) {
		if openapiConfig.JsonUrl != "" {
			s.OpenAPIConfig.JsonUrl = openapiConfig.JsonUrl
		}

		if openapiConfig.SwaggerUrl != "" {
			s.OpenAPIConfig.SwaggerUrl = openapiConfig.SwaggerUrl
		}

		if openapiConfig.JsonFilePath != "" {
			s.OpenAPIConfig.JsonFilePath = openapiConfig.JsonFilePath
		}

		if openapiConfig.UIHandler != nil {
			s.OpenAPIConfig.UIHandler = openapiConfig.UIHandler
		}

		s.OpenAPIConfig.DisableSwagger = openapiConfig.DisableSwagger
		s.OpenAPIConfig.DisableSwaggerUI = openapiConfig.DisableSwaggerUI
		s.OpenAPIConfig.DisableLocalSave = openapiConfig.DisableLocalSave
		s.OpenAPIConfig.PrettyFormatJson = openapiConfig.PrettyFormatJson

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

func WithRouteOptions(options ...func(*BaseRoute)) func(*Server) {
	return func(s *Server) {
		s.routeOptions = append(s.routeOptions, options...)
	}
}
