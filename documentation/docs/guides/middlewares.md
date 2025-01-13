# Middlewares

Fuego supports `net/http` middlewares.

It means that all existing middlewares for `net/http`,
including the ones from `chi` and `gorilla` can be used with Fuego! :fire:

You can use them to add functionalities to your routes, such as logging,
authentication, etc.

Middlewares can be registered at 2 levels:

- **Route middlewares**: applies only on registered routes, easily scopable to a specific group, or several routes.
- **Global middlewares**: applies on every request, even non-matching routes (useful for CORS for example).

## Route middlewares

You can add middlewares to a single route.
They are treated as an option to the route handler.
They will be added in the [`http.ServeMux`](https://pkg.go.dev/net/http#ServeMux) in the order they are declared, when registering the route.

```go title="main.go" showLineNumbers {13-14}
package main

import (
	"github.com/go-fuego/fuego"
)

func main() {
	s := fuego.NewServer()

	// Declare the middlewares after the route handler
	fuego.Get(s, "/", myController,
		option.QueryInt("page", "The page number"),
		option.Middleware(middleware1),
		option.Middleware(middleware2, middleware3),
	)

	s.Run()
}
```

### Apply on group or server

To mimic the well-known `Use` method from [`chi`](https://pkg.go.dev/github.com/go-chi/chi/v5#Mux.Use) and [`Gin`](https://pkg.go.dev/github.com/gin-gonic/gin#Engine.Use), Fuego provides a `Use` method to add middlewares to a Group or Server. They are treated as an option to the server or group handler and will be applied to all routes.

But we recommend using the `option.Middleware` method for better readability.

```go title="main.go" showLineNumbers
package main

import (
	"net/http"

	"github.com/go-fuego/fuego"
)

func main() {
	s := fuego.NewServer()

	// Add a middleware to the whole server
	fuego.Use(s, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Do something before the request
			next.ServeHTTP(w, r)
			// Do something after the request
		})
	})

	fuego.Use(s, myMiddleware)

	fuego.Get(s, "/", myController)

	s.Run()
}
```

```go title="main.go" showLineNumbers
package main

import (
	"net/http"

	"github.com/go-fuego/fuego"
)

func main() {
	s := fuego.NewServer()

	// Add a middleware to a group of routes
	api := fuego.Group(s, "/api")

	fuego.Use(api, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Do something before the request
			// Only affects routes in the group
			next.ServeHTTP(w, r)
			// Do something after the request
		})
	})

	// Requests to /api will go through the middleware
	fuego.Get(api, "/", myController)

	// Requests to / will NOT! go through the middleware
	fuego.Get(s, "/", myController)

	s.Run()
}
```

## Global Middlewares

Global middlewares are applied to every request, even if the route does not match.
They are useful for CORS, for example as CORS are using the OPTION method even if not registered.
They are registered in the Server `Handler`, not the `Mux`, and just before
`Run` is called (not at route registration).

```go title="main.go" showLineNumbers
package main

import (
	"net/http"

	"github.com/go-fuego/fuego"
)

func main() {
	s := fuego.NewServer()

	// Add a global middleware
	fuego.WithGlobalMiddlewares(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Hello", "World")
			// Do something before the request
			next.ServeHTTP(w, r)
			// Do something after the request
		})
	})

	fuego.Get(s, "/my-route", myController)

	// Here, the global middleware is applied
	s.Run()
}
```

This will work even if the user requests a route that does not exist:

```bash
curl -v -X GET http://localhost:3000/unknown-route
```

We can see the `X-Hello: World` header in the response.
