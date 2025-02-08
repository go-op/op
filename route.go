package fuego

import (
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func NewRouteWithParams[RequestParams, ResponseBody, RequestBody any](method, path string, handler any, e *Engine, options ...func(*BaseRoute)) RouteWithParams[RequestParams, ResponseBody, RequestBody] {
	return RouteWithParams[RequestParams, ResponseBody, RequestBody]{
		Route: NewRoute[ResponseBody, RequestBody](method, path, handler, e, options...),
	}
}

type RouteWithParams[RequestParams any, ResponseBody any, RequestBody any] struct {
	Route[ResponseBody, RequestBody]
}

func NewRoute[T, B any](method, path string, handler any, e *Engine, options ...func(*BaseRoute)) Route[T, B] {
	return Route[T, B]{
		BaseRoute: NewBaseRoute(method, path, handler, e, options...),
	}
}

// Route is the main struct for a route in Fuego.
// It contains the OpenAPI operation and other metadata.
// It is a wrapper around BaseRoute, with the addition of the response and request body types.
type Route[ResponseBody any, RequestBody any] struct {
	BaseRoute
}

func NewBaseRoute(method, path string, handler any, e *Engine, options ...func(*BaseRoute)) BaseRoute {
	baseRoute := BaseRoute{
		Method:              method,
		Path:                path,
		Params:              make(map[string]OpenAPIParam),
		FullName:            FuncName(handler),
		Operation:           openapi3.NewOperation(),
		OpenAPI:             e.OpenAPI,
		RequestContentTypes: e.requestContentTypes,
	}

	for _, o := range options {
		o(&baseRoute)
	}

	return baseRoute
}

// BaseRoute is the base struct for all routes in Fuego.
// It contains the OpenAPI operation and other metadata.
type BaseRoute struct {
	// handler executed for this route
	Handler http.Handler

	// OpenAPI operation
	Operation *openapi3.Operation

	// Ref to the whole OpenAPI spec. Be careful when changing directly its value directly.
	OpenAPI *OpenAPI

	Params map[string]OpenAPIParam

	// HTTP method (GET, POST, PUT, PATCH, DELETE)
	Method string

	// URL path. Will be prefixed by the base path of the server and the group path if any
	Path string

	// namespace and name of the function to execute
	FullName string

	// Content types accepted for the request body. If nil, all content types (*/*) are accepted.
	RequestContentTypes []string

	Middlewares []func(http.Handler) http.Handler

	// Default status code for the response
	DefaultStatusCode int

	// If true, the route will not be documented in the OpenAPI spec
	Hidden bool

	// Override the default description
	overrideDescription bool

	// Middleware configuration for the route
	middlewareConfig *MiddlewareConfig
}

func (r *BaseRoute) GenerateDefaultDescription() {
	if r.overrideDescription {
		return
	}
	r.Operation.Description = DefaultDescription(r.FullName, r.Middlewares, r.middlewareConfig) + r.Operation.Description
}

func (r *BaseRoute) GenerateDefaultOperationID() {
	r.Operation.OperationID = r.Method + "_" + strings.ReplaceAll(strings.ReplaceAll(r.Path, "{", ":"), "}", "")
}
