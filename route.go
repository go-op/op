package fuego

import (
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func NewRoute[T, B, P any](method, path string, handler any, openapi *OpenAPI, options ...func(*BaseRoute)) Route[T, B, P] {
	return Route[T, B, P]{
		BaseRoute: NewBaseRoute(method, path, handler, openapi, options...),
	}
}

// Route is the main struct for a route in Fuego.
// It contains the OpenAPI operation and other metadata.
// It is a wrapper around BaseRoute, with the addition of the response and request body types.
type Route[ResponseBody any, RequestBody any, Params any] struct {
	BaseRoute
}

func NewBaseRoute(method, path string, handler any, openapi *OpenAPI, options ...func(*BaseRoute)) BaseRoute {
	baseRoute := BaseRoute{
		Method:    method,
		Path:      path,
		Params:    make(map[string]OpenAPIParam),
		FullName:  FuncName(handler),
		Operation: openapi3.NewOperation(),
		OpenAPI:   openapi,
	}

	for _, o := range options {
		o(&baseRoute)
	}

	return baseRoute
}

// BaseRoute is the base struct for all routes in Fuego.
// It contains the OpenAPI operation and other metadata.
type BaseRoute struct {
	// OpenAPI operation
	Operation *openapi3.Operation

	// HTTP method (GET, POST, PUT, PATCH, DELETE)
	Method string

	// URL path. Will be prefixed by the base path of the server and the group path if any
	Path string

	// handler executed for this route
	Handler http.Handler

	// namespace and name of the function to execute
	FullName    string
	Params      map[string]OpenAPIParam
	Middlewares []func(http.Handler) http.Handler

	// Content types accepted for the request body. If nil, all content types (*/*) are accepted.
	AcceptedContentTypes []string

	// If true, the route will not be documented in the OpenAPI spec
	Hidden bool

	// Default status code for the response
	DefaultStatusCode int

	// Ref to the whole OpenAPI spec. Be careful when changing directly its value directly.
	OpenAPI *OpenAPI

	// Override the default description
	overrideDescription bool
}

func (r *BaseRoute) GenerateDefaultDescription() {
	if r.overrideDescription {
		return
	}
	r.Operation.Description = DefaultDescription(r.FullName, r.Middlewares) + r.Operation.Description
}

func (r *BaseRoute) GenerateDefaultOperationID() {
	r.Operation.OperationID = r.Method + "_" + strings.ReplaceAll(strings.ReplaceAll(r.Path, "{", ":"), "}", "")
}
