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
	"reflect"
	"slices"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
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

	// OpenAPI documentation tags used for logical groupings of operations
	// These tags will be inherited by child Routes/Groups
	tags []string

	// OpenAPI documentation parameters used for all server routes
	params map[string]OpenAPIParam

	middlewares []func(http.Handler) http.Handler

	disableStartupMessages bool
	disableAutoGroupTags   bool
	groupTag               string
	mainRouter             *Server // Ref to the main router (used for groups)
	basePath               string  // Base path of the group

	globalOpenAPIResponses []openAPIError // Global error responses

	OpenApiSpec openapi3.T // OpenAPI spec generated by the server

	listener net.Listener

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

	openAPIGenerator *openapi3gen.Generator

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
		Mux:         http.NewServeMux(),
		OpenApiSpec: NewOpenApiSpec(),

		OpenAPIConfig: defaultOpenAPIConfig,

		openAPIGenerator: openapi3gen.NewGenerator(
			openapi3gen.UseAllExportedFields(),
		),

		params: make(map[string]OpenAPIParam),

		Security: NewSecurity(),
	}

	defaultOptions := [...]func(*Server){
		WithDisallowUnknownFields(true),
		WithSerializer(Send),
		WithErrorSerializer(SendError),
		WithErrorHandler(ErrorHandler),
		WithGlobalResponseTypes(http.StatusBadRequest, "Bad Request _(validation or deserialization error)_", HTTPError{}),
		WithGlobalResponseTypes(http.StatusInternalServerError, "Internal Server Error _(panics)_", HTTPError{}),
	}

	for _, option := range append(defaultOptions[:], options...) {
		option(s)
	}

	if s.Server.Addr == "" {
		WithAddr("localhost:9999")(s)
	}

	s.OpenApiSpec.Servers = append(s.OpenApiSpec.Servers, &openapi3.Server{
		URL:         fmt.Sprintf("%s://%s", s.proto(), s.Server.Addr),
		Description: "local server",
	})

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
//		}).Handler)
//	)
func WithCorsMiddleware(corsMiddleware func(http.Handler) http.Handler) func(*Server) {
	return func(c *Server) { c.corsMiddleware = corsMiddleware }
}

// WithGlobalResponseTypes adds default response types to the server.
// useful for adding global error types.
// For example:
//
//	app := fuego.NewServer(
//		fuego.WithGlobalResponseTypes(400, "Bad Request _(validation or deserialization error)_", HTTPError{}),
//		fuego.WithGlobalResponseTypes(401, "Unauthorized _(authentication error)_", HTTPError{}),
//		fuego.WithGlobalResponseTypes(500, "Internal Server Error _(panics)_", HTTPError{}),
//	)
func WithGlobalResponseTypes(code int, description string, errorType ...any) func(*Server) {
	errorType = append(errorType, HTTPError{})
	return func(c *Server) {
		c.globalOpenAPIResponses = append(c.globalOpenAPIResponses, openAPIError{code, description, errorType[0]})
	}
}

// WithoutAutoGroupTags disables the automatic grouping of routes by tags.
// By default, routes are tagged by group.
// For example:
//
//	recipeGroup := fuego.Group(s, "/recipes")
//	fuego.Get(recipeGroup, "/", func(*ContextNoBody) (ans, error) {
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
// Deprecated: Please use fuego.WithAddr(addr string)
func WithPort(port int) func(*Server) {
	return func(s *Server) { s.Server.Addr = fmt.Sprintf("localhost:%d", port) }
}

// WithAddr optionally specifies the TCP address for the server to listen on, in the form "host:port".
// If not specified addr ':9999' will be used.
func WithAddr(addr string) func(*Server) {
	return func(c *Server) {
		if c.listener != nil {
			panic("cannot set addr when a listener is already configured")
		}
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

// WithListener configures the server to use a custom listener.
func WithListener(listener net.Listener) func(*Server) {
	return func(s *Server) {
		if s.listener != nil {
			panic("a listener is already configured; cannot overwrite it")
		}
		s.isTLS = isTLSListener(listener)
		WithAddr(listener.Addr().String())(s)
		s.listener = listener
	}
}

func isTLSListener(listener net.Listener) bool {
	listenerType := reflect.TypeOf(listener)
	if listenerType != nil && listenerType.String() == "*tls.listener" {
		return true
	}
	return false
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
		baseRoute := &BaseRoute{
			Params:    make(map[string]OpenAPIParam),
			Operation: openapi3.NewOperation(),
		}
		for _, option := range options {
			option(baseRoute)
		}
		s.params = baseRoute.Params
	}
}

// Replaces Tags for the Server (i.e Group)
// By default, the tag is the type of the response body.
func (s *Server) Tags(tags ...string) *Server {
	s.tags = tags
	return s
}

// AddTags adds tags from the Server (i.e Group)
// Tags from the parent Groups will be respected
func (s *Server) AddTags(tags ...string) *Server {
	s.tags = append(s.tags, tags...)
	return s
}

// RemoveTags removes tags from the Server (i.e Group)
// if the parent Group(s) has matching tags they will be removed
func (s *Server) RemoveTags(tags ...string) *Server {
	for _, tag := range tags {
		for i, t := range s.tags {
			if t == tag {
				s.tags = slices.Delete(s.tags, i, i+1)
				break
			}
		}
	}
	return s
}

func (s *Server) Close() {
	if s.listener != nil {
		s.listener.Close()
	}
	s.Server.Close()
}

// Registers a param for all server routes.
func (s *Server) Param(name, description string, params ...OpenAPIParamOption) *Server {
	param := OpenAPIParam{Name: name, Description: description}

	for _, p := range params {
		if p.Required {
			param.Required = p.Required
		}
		if p.Example != "" {
			param.Example = p.Example
		}
		if p.Type != "" {
			param.Type = p.Type
		}
	}

	if s.params == nil {
		s.params = make(map[string]OpenAPIParam)
	}
	s.params[name] = param

	return s
}

// Registers a header param for all server routes.
func (s *Server) Header(name, description string, params ...OpenAPIParamOption) *Server {
	s.Param(name, description, append(params, OpenAPIParamOption{Type: HeaderParamType})...)
	return s
}

// Registers a cookie param for all server routes.
func (s *Server) Cookie(name, description string, params ...OpenAPIParamOption) *Server {
	s.Param(name, description, append(params, OpenAPIParamOption{Type: CookieParamType})...)
	return s
}

// Registers a query param for all server routes.
func (s *Server) Query(name, description string, params ...OpenAPIParamOption) *Server {
	s.Param(name, description, append(params, OpenAPIParamOption{Type: QueryParamType})...)
	return s
}
