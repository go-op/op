package fuegoecho

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/internal"
)

type OpenAPIHandler struct {
	Echo *echo.Echo
}

func (o *OpenAPIHandler) SpecHandler(e *fuego.Engine) {
	Get(e, o.Echo, e.OpenAPI.Config.SpecURL, e.SpecHandler(), fuego.OptionHide())
}

func (o *OpenAPIHandler) UIHandler(e *fuego.Engine) {
	GetEcho(
		e,
		o.Echo,
		e.OpenAPI.Config.SwaggerURL+"/",
		echo.WrapHandler(e.OpenAPI.Config.UIHandler(e.OpenAPI.Config.SpecURL)),
		fuego.OptionHide(),
	)
}

type echoIRouter interface {
	Add(method, path string, handler echo.HandlerFunc, middleware ...echo.MiddlewareFunc) *echo.Route
}

func AddEcho(engine *fuego.Engine, echoRouter echoIRouter,
	method, path string, handler echo.HandlerFunc,
	options ...func(*fuego.BaseRoute),
) *fuego.Route[any, any] {
	return handleEcho(engine, echoRouter, method, path, handler, options...)
}

func GetEcho(engine *fuego.Engine, echoRouter echoIRouter,
	path string, handler echo.HandlerFunc,
	options ...func(*fuego.BaseRoute),
) *fuego.Route[any, any] {
	return handleEcho(engine, echoRouter, http.MethodGet, path, handler, options...)
}

func PostEcho(engine *fuego.Engine, echoRouter echoIRouter,
	path string, handler echo.HandlerFunc,
	options ...func(*fuego.BaseRoute),
) *fuego.Route[any, any] {
	return handleEcho(engine, echoRouter, http.MethodPost, path, handler, options...)
}

func PutEcho(engine *fuego.Engine, echoRouter echoIRouter,
	path string, handler echo.HandlerFunc,
	options ...func(*fuego.BaseRoute),
) *fuego.Route[any, any] {
	return handleEcho(engine, echoRouter, http.MethodPut, path, handler, options...)
}

func PatchEcho(engine *fuego.Engine, echoRouter echoIRouter,
	path string, handler echo.HandlerFunc,
	options ...func(*fuego.BaseRoute),
) *fuego.Route[any, any] {
	return handleEcho(engine, echoRouter, http.MethodPatch, path, handler, options...)
}

func DeleteEcho(engine *fuego.Engine, echoRouter echoIRouter,
	path string, handler echo.HandlerFunc,
	options ...func(*fuego.BaseRoute),
) *fuego.Route[any, any] {
	return handleEcho(engine, echoRouter, http.MethodDelete, path, handler, options...)
}

func Add[T, B any](engine *fuego.Engine, echoRouter echoIRouter, method, path string, handler func(c fuego.ContextWithBody[B]) (T, error), options ...func(*fuego.BaseRoute)) *fuego.Route[T, B] {
	return handleFuego(engine, echoRouter, method, path, handler, options...)
}

func Get[T, B any](engine *fuego.Engine, echoRouter echoIRouter, path string, handler func(c fuego.ContextWithBody[B]) (T, error), options ...func(*fuego.BaseRoute)) *fuego.Route[T, B] {
	return handleFuego(engine, echoRouter, http.MethodGet, path, handler, options...)
}

func Post[T, B any](engine *fuego.Engine, echoRouter echoIRouter, path string, handler func(c fuego.ContextWithBody[B]) (T, error), options ...func(*fuego.BaseRoute)) *fuego.Route[T, B] {
	return handleFuego(engine, echoRouter, http.MethodPost, path, handler, options...)
}

func Put[T, B any](engine *fuego.Engine, echoRouter echoIRouter, path string, handler func(c fuego.ContextWithBody[B]) (T, error), options ...func(*fuego.BaseRoute)) *fuego.Route[T, B] {
	return handleFuego(engine, echoRouter, http.MethodPut, path, handler, options...)
}

func Patch[T, B any](engine *fuego.Engine, echoRouter echoIRouter, path string, handler func(c fuego.ContextWithBody[B]) (T, error), options ...func(*fuego.BaseRoute)) *fuego.Route[T, B] {
	return handleFuego(engine, echoRouter, http.MethodPatch, path, handler, options...)
}

func Delete[T, B any](engine *fuego.Engine, echoRouter echoIRouter, path string, handler func(c fuego.ContextWithBody[B]) (T, error), options ...func(*fuego.BaseRoute)) *fuego.Route[T, B] {
	return handleFuego(engine, echoRouter, http.MethodDelete, path, handler, options...)
}

func handleFuego[T, B any](engine *fuego.Engine, echoRouter echoIRouter, method, path string, fuegoHandler func(c fuego.ContextWithBody[B]) (T, error), options ...func(*fuego.BaseRoute)) *fuego.Route[T, B] {
	baseRoute := fuego.NewBaseRoute(method, path, fuegoHandler, engine, options...)
	return fuego.Registers(engine, echoRouteRegisterer[T, B]{
		echoRouter:  echoRouter,
		route:       fuego.Route[T, B]{BaseRoute: baseRoute},
		echoHandler: EchoHandler(engine, fuegoHandler, baseRoute),
	})
}

func handleEcho(engine *fuego.Engine, echoRouter echoIRouter, method, path string, echoHandler echo.HandlerFunc, options ...func(*fuego.BaseRoute)) *fuego.Route[any, any] {
	baseRoute := fuego.NewBaseRoute(method, path, echoHandler, engine, options...)
	return fuego.Registers(engine, echoRouteRegisterer[any, any]{
		echoRouter:  echoRouter,
		route:       fuego.Route[any, any]{BaseRoute: baseRoute},
		echoHandler: echoHandler,
	})
}

type echoRouteRegisterer[T, B any] struct {
	echoRouter  echoIRouter
	echoHandler echo.HandlerFunc
	route       fuego.Route[T, B]
}

func (a echoRouteRegisterer[T, B]) Register() fuego.Route[T, B] {
	route := a.echoRouter.Add(a.route.Method, a.route.Path, a.echoHandler)
	a.route.Path = route.Path
	return a.route
}

// Convert a Fuego handler to a Gin handler.
func EchoHandler[B, T any](engine *fuego.Engine, handler func(c fuego.ContextWithBody[B]) (T, error), route fuego.BaseRoute) echo.HandlerFunc {
	return func(c echo.Context) error {
		context := &echoContext[B]{
			CommonContext: internal.CommonContext[B]{
				CommonCtx:         c.Request().Context(),
				UrlValues:         c.Request().URL.Query(),
				OpenAPIParams:     route.Params,
				DefaultStatusCode: route.DefaultStatusCode,
			},
			echoCtx: c,
		}
		fuego.Flow(engine, context, handler)
		return nil
	}
}
