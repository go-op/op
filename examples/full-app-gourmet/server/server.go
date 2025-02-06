package server

import (
	"net/http"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/handler"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/static"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/templates"
	"github.com/go-fuego/fuego/option"
)

type Resources struct {
	HandlersResources handler.Resource
}

func cache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=600")

		h.ServeHTTP(w, r)
	})
}

func (rs Resources) Setup(
	options ...func(*fuego.Server),
) *fuego.Server {
	serverOptions := []func(*fuego.Server){
		fuego.WithAutoAuth(handler.LoginFunc),
		fuego.WithTemplateFS(templates.FS),
		fuego.WithTemplateGlobs("**/*.html", "**/**/*.html"),
		fuego.WithRouteOptions(
			fuego.OptionAddResponse(http.StatusForbidden, "Forbidden", fuego.Response{Type: fuego.HTTPError{}}),
		),
	}

	options = append(serverOptions, options...)

	// Create server with some options
	app := fuego.NewServer(options...)

	app.OpenAPI.Description().Info.Title = "Gourmet API"

	// Register middlewares (functions that will be executed before AND after the controllers, in the order they are registered)
	// With fuego, you can use any existing middleware that relies on `net/http`, or create your own
	fuego.Use(app, chiMiddleware.Compress(5, "text/html", "text/css", "application/json"))

	fuego.Handle(app, "/static/", http.StripPrefix("/static", static.Handler()), option.Middleware(cache))

	// Register views (controllers that return HTML pages)
	rs.HandlersResources.Routes(app)

	return app
}
