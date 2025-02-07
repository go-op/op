package fuego

import (
	"log/slog"
	"net/http"
	"reflect"
	"runtime"
	"strings"
)

// Group allows grouping routes under a common path.
// Middlewares are scoped to the group.
// For example:
//
//	s := fuego.NewServer()
//	viewsRoutes := fuego.Group(s, "")
//	apiRoutes := fuego.Group(s, "/api")
//	// Registering a middlewares scoped to /api only
//	fuego.Use(apiRoutes, myMiddleware)
//	// Registering a route under /api/users
//	fuego.Get(apiRoutes, "/users", func(c fuego.ContextNoBody) (ans, error) {
//		return ans{Ans: "users"}, nil
//	})
//	s.Run()
func Group(s *Server, path string, routeOptions ...func(*BaseRoute)) *Server {
	if path == "/" {
		path = ""
	} else if path != "" && path[len(path)-1] == '/' {
		slog.Warn("Group path should not end with a slash.", "path", path+"/", "new", path)
	}

	ss := *s
	newServer := &ss
	newServer.basePath += path

	if autoTag := strings.TrimLeft(path, "/"); !s.disableAutoGroupTags && autoTag != "" {
		newServer.routeOptions = append(s.routeOptions, OptionTags(autoTag))
	}

	newServer.routeOptions = append(newServer.routeOptions, routeOptions...)

	return newServer
}

// All captures all methods (GET, POST, PUT, PATCH, DELETE) and register a controller.
func All[T, B any](s *Server, path string, controller func(ContextWithBody[B]) (T, error), options ...func(*BaseRoute)) *Route[T, B] {
	return registerFuegoController(s, "", path, controller, options...)
}

func Get[T, B any](s *Server, path string, controller func(ContextWithBody[B]) (T, error), options ...func(*BaseRoute)) *Route[T, B] {
	return registerFuegoController(s, http.MethodGet, path, controller, options...)
}

func Post[T, B any](s *Server, path string, controller func(ContextWithBody[B]) (T, error), options ...func(*BaseRoute)) *Route[T, B] {
	return registerFuegoController(s, http.MethodPost, path, controller, options...)
}

func Delete[T, B any](s *Server, path string, controller func(ContextWithBody[B]) (T, error), options ...func(*BaseRoute)) *Route[T, B] {
	return registerFuegoController(s, http.MethodDelete, path, controller, options...)
}

func Put[T, B any](s *Server, path string, controller func(ContextWithBody[B]) (T, error), options ...func(*BaseRoute)) *Route[T, B] {
	return registerFuegoController(s, http.MethodPut, path, controller, options...)
}

func Patch[T, B any](s *Server, path string, controller func(ContextWithBody[B]) (T, error), options ...func(*BaseRoute)) *Route[T, B] {
	return registerFuegoController(s, http.MethodPatch, path, controller, options...)
}

// Register registers a controller into the default net/http mux.
//
// Deprecated: Used internally. Please satisfy the [Registerer] interface instead and pass to [Registers].
func Register[T, B any](s *Server, route Route[T, B], controller http.Handler, options ...func(*BaseRoute)) *Route[T, B] {
	for _, o := range options {
		o(&route.BaseRoute)
	}

	route.Path = s.basePath + route.Path

	fullPath := route.Path
	if route.Method != "" {
		fullPath = route.Method + " " + fullPath
	}
	slog.Debug("registering controller " + fullPath)

	route.Middlewares = append(s.middlewares, route.Middlewares...)
	s.Mux.Handle(fullPath, withMiddlewares(controller, route.Middlewares...))

	return &route
}

func UseStd(s *Server, middlewares ...func(http.Handler) http.Handler) {
	Use(s, middlewares...)
}

func Use(s *Server, middlewares ...func(http.Handler) http.Handler) {
	s.middlewares = append(s.middlewares, middlewares...)
}

// Handle registers a standard HTTP handler into the default mux.
// Use this function if you want to use a standard HTTP handler instead of a Fuego controller.
func Handle(s *Server, path string, controller http.Handler, options ...func(*BaseRoute)) *Route[any, any] {
	return Register(s, Route[any, any]{
		BaseRoute: BaseRoute{
			Path:     path,
			FullName: FuncName(controller),
		},
	}, controller)
}

func AllStd(s *Server, path string, controller func(http.ResponseWriter, *http.Request), options ...func(*BaseRoute)) *Route[any, any] {
	return registerStdController(s, "", path, controller, options...)
}

func GetStd(s *Server, path string, controller func(http.ResponseWriter, *http.Request), options ...func(*BaseRoute)) *Route[any, any] {
	return registerStdController(s, http.MethodGet, path, controller, options...)
}

func PostStd(s *Server, path string, controller func(http.ResponseWriter, *http.Request), options ...func(*BaseRoute)) *Route[any, any] {
	return registerStdController(s, http.MethodPost, path, controller, options...)
}

func DeleteStd(s *Server, path string, controller func(http.ResponseWriter, *http.Request), options ...func(*BaseRoute)) *Route[any, any] {
	return registerStdController(s, http.MethodDelete, path, controller, options...)
}

func PutStd(s *Server, path string, controller func(http.ResponseWriter, *http.Request), options ...func(*BaseRoute)) *Route[any, any] {
	return registerStdController(s, http.MethodPut, path, controller, options...)
}

func PatchStd(s *Server, path string, controller func(http.ResponseWriter, *http.Request), options ...func(*BaseRoute)) *Route[any, any] {
	return registerStdController(s, http.MethodPatch, path, controller, options...)
}

func registerFuegoController[T, B any](s *Server, method, path string, controller func(ContextWithBody[B]) (T, error), options ...func(*BaseRoute)) *Route[T, B] {
	options = append(options, OptionHeader("Accept", ""))
	route := NewRoute[T, B](method, path, controller, s.Engine, append(s.routeOptions, options...)...)

	return Registers(s.Engine, netHttpRouteRegisterer[T, B]{
		s:          s,
		route:      route,
		controller: HTTPHandler(s, controller, route.BaseRoute),
	})
}

func registerStdController(s *Server, method, path string, controller func(http.ResponseWriter, *http.Request), options ...func(*BaseRoute)) *Route[any, any] {
	route := NewRoute[any, any](method, path, controller, s.Engine, append(s.routeOptions, options...)...)

	return Registers(s.Engine, netHttpRouteRegisterer[any, any]{
		s:          s,
		route:      route,
		controller: http.HandlerFunc(controller),
	})
}

func withMiddlewares(controller http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		controller = middlewares[i](controller)
	}
	return controller
}

// FuncName returns the name of a function and the name with package path
func FuncName(f interface{}) string {
	return strings.TrimSuffix(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name(), "-fm")
}

// NameFromNamespace returns the Route's FullName final string
// delimited by `.`. Essentially getting the name of the function
// and leaving the package path
//
// The output can be further modified with a list of optional
// string manipulation funcs (i.e func(string) string)
func (route Route[T, B]) NameFromNamespace(opts ...func(string) string) string {
	ss := strings.Split(route.FullName, ".")
	name := ss[len(ss)-1]
	for _, o := range opts {
		name = o(name)
	}
	return name
}

// transform camelCase to human-readable string
func camelToHuman(s string) string {
	result := strings.Builder{}
	for i, r := range s {
		if 'A' <= r && r <= 'Z' {
			if i > 0 {
				result.WriteRune(' ')
			}
			result.WriteRune(r + 'a' - 'A') // 'A' -> 'a
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// DefaultDescription returns a default .md description for a controller
func DefaultDescription[T any](handler string, middlewares []T, middlewareDisplayLimit int) string {
	description := "#### Controller: \n\n`" +
		handler + "`"

	if len(middlewares) > 0 {
		description += "\n\n#### Middlewares:\n"

		for i, fn := range middlewares {
			description += "\n- `" + FuncName(fn) + "`"

			if i == middlewareDisplayLimit-1 {
				description += "\n- more middleware…"
				break
			}
		}
	}

	return description + "\n\n---\n\n"
}

type netHttpRouteRegisterer[T, B any] struct {
	s          *Server
	controller http.Handler
	route      Route[T, B]
}

var _ Registerer[string, any] = netHttpRouteRegisterer[string, any]{}

func (a netHttpRouteRegisterer[T, B]) Register() Route[T, B] {
	return *Register(a.s, a.route, a.controller)
}
