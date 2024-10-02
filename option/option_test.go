package option

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thejerf/slogassert"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/param"
)

// dummyMiddleware sets the X-Test header on the request and the X-Test-Response header on the response.
func dummyMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("X-Test", "test")
		w.Header().Set("X-Test-Response", "response")
		handler.ServeHTTP(w, r)
	})
}

func helloWorld(ctx *fuego.ContextNoBody) (string, error) {
	return "hello world", nil
}

type ReqBody struct {
	A string
	B int
}

type Resp struct {
	Message string `json:"message"`
}

func dummyController(_ *fuego.ContextWithBody[ReqBody]) (Resp, error) {
	return Resp{Message: "hello world"}, nil
}

// orderMiddleware sets the X-Test-Order Header on the request and
// X-Test-Response header on the response. It is
// used to test the order execution of our middleware
func orderMiddleware(s string) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Add("X-Test-Order", s)
			w.Header().Set("X-Test-Response", "response")
			handler.ServeHTTP(w, r)
		})
	}
}

func TestPerRouteMiddleware(t *testing.T) {
	s := fuego.NewServer()

	fuego.Get(s, "/withMiddleware", func(ctx *fuego.ContextNoBody) (string, error) {
		return "withmiddleware", nil
	}, Middleware(dummyMiddleware))

	fuego.Get(s, "/withoutMiddleware", func(ctx *fuego.ContextNoBody) (string, error) {
		return "withoutmiddleware", nil
	})

	t.Run("withMiddleware", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/withMiddleware", nil)

		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, "withmiddleware", w.Body.String())
		require.Equal(t, "response", w.Header().Get("X-Test-Response"))
	})

	t.Run("withoutMiddleware", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/withoutMiddleware", nil)

		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, "withoutmiddleware", w.Body.String())
		require.Equal(t, "", w.Header().Get("X-Test-Response"))
	})
}

func TestUse(t *testing.T) {
	t.Run("base", func(t *testing.T) {
		s := fuego.NewServer()
		fuego.Use(s, orderMiddleware("First!"))
		fuego.Get(s, "/test", func(ctx *fuego.ContextNoBody) (string, error) {
			return "test", nil
		})

		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.Header.Set("X-Test-Order", "Start!")
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, []string{"Start!", "First!"}, r.Header["X-Test-Order"])
	})

	t.Run("multiple uses of Use", func(t *testing.T) {
		s := fuego.NewServer()
		fuego.Use(s, orderMiddleware("First!"))
		fuego.Use(s, orderMiddleware("Second!"))
		fuego.Get(s, "/test", func(ctx *fuego.ContextNoBody) (string, error) {
			return "test", nil
		})

		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.Header.Set("X-Test-Order", "Start!")
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, []string{"Start!", "First!", "Second!"}, r.Header["X-Test-Order"])
	})

	t.Run("variadic use of Use", func(t *testing.T) {
		s := fuego.NewServer()
		fuego.Use(s, orderMiddleware("First!"))
		fuego.Use(s, orderMiddleware("Second!"), orderMiddleware("Third!"))
		fuego.Get(s, "/test", func(ctx *fuego.ContextNoBody) (string, error) {
			return "test", nil
		})

		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.Header.Set("X-Test-Order", "Start!")
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, []string{"Start!", "First!", "Second!", "Third!"}, r.Header["X-Test-Order"])
	})

	t.Run("variadic use of Route Get", func(t *testing.T) {
		s := fuego.NewServer()
		fuego.Use(s, orderMiddleware("First!"))
		fuego.Use(s, orderMiddleware("Second!"), orderMiddleware("Third!"))
		fuego.Get(s, "/test", func(ctx *fuego.ContextNoBody) (string, error) {
			return "test", nil
		},
			Middleware(orderMiddleware("Fourth!")),
			Middleware(orderMiddleware("Fifth!")),
		)

		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.Header.Set("X-Test-Order", "Start!")
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, []string{"Start!", "First!", "Second!", "Third!", "Fourth!", "Fifth!"}, r.Header["X-Test-Order"])
	})
}

type ans struct{}

func TestParam(t *testing.T) {
	t.Run("warn if param is not found in openAPI config but called in controller (possibly typo)", func(t *testing.T) {
		handler := slogassert.New(t, slog.LevelWarn, nil)

		s := fuego.NewServer(
			fuego.WithLogHandler(handler),
		)

		fuego.Get(s, "/correct", func(c fuego.ContextNoBody) (ans, error) {
			c.QueryParam("quantity")
			return ans{}, nil
		},
			Query("quantity", "some description"),
			QueryInt("number", "some description", param.Example("3", 3)),
			QueryBool("is_active", "some description"),
		)

		fuego.Get(s, "/typo", func(c fuego.ContextNoBody) (ans, error) {
			c.QueryParam("quantityy-with-a-typo")
			return ans{}, nil
		},
			Query("quantity", "some description"),
		)

		t.Run("correct param", func(t *testing.T) {
			r := httptest.NewRequest("GET", "/correct", nil)
			w := httptest.NewRecorder()

			s.Mux.ServeHTTP(w, r)

			// all log messages have been accounted for
			handler.AssertEmpty()
		})

		t.Run("typo param", func(t *testing.T) {
			r := httptest.NewRequest("GET", "/typo", nil)
			w := httptest.NewRecorder()

			s.Mux.ServeHTTP(w, r)

			handler.AssertMessage("query parameter not expected in OpenAPI spec")

			// all log messages have been accounted for
			handler.AssertEmpty()
		})
	})
}

func TestHeader(t *testing.T) {
	t.Run("Declare a header parameter for the route", func(t *testing.T) {
		s := fuego.NewServer()

		fuego.Get(s, "/test", helloWorld,
			Header("X-Test", "test header", param.Required(), param.Example("test", "My Header"), param.Default("test")),
		)

		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, "hello world", w.Body.String())
	})
}

func TestOpenAPI(t *testing.T) {
	t.Run("Declare a openapi parameters for the route", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Get(s, "/test", helloWorld,
			Summary("test summary"),
			Description("test description"),
			Tags("first-tag", "second-tag"),
			Deprecated(),
			OperationID("test-operation-id"),
		)

		require.Equal(t, "test summary", route.Operation.Summary)
		require.Equal(t, "controller: `github.com/go-fuego/fuego/option.helloWorld`\n\n---\n\ntest description", route.Operation.Description)
		require.Equal(t, []string{"first-tag", "second-tag"}, route.Operation.Tags)
		require.True(t, route.Operation.Deprecated)
	})
}

func TestGroup(t *testing.T) {
	paramsGroup := Group(
		Header("X-Test", "test header", param.Required(), param.Example("test", "My Header"), param.Default("test")),
		Query("name", "Filter by name", param.Example("cat name", "felix"), param.Nullable()),
		Cookie("session", "Session cookie", param.Example("session", "1234"), param.Nullable()),
	)

	t.Run("Declare a group parameter for the route", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Get(s, "/test", helloWorld, paramsGroup)

		require.NotNil(t, route)
		require.NotNil(t, route.Params)
		require.Len(t, route.Params, 3)
		require.Equal(t, "test header", route.Params["X-Test"].Description)
		require.Equal(t, "My Header", route.Operation.Parameters.GetByInAndName("header", "X-Test").Examples["test"].Value.Value)
	})
}

func TestQuery(t *testing.T) {
	t.Run("panics if example is not the correct type", func(t *testing.T) {
		s := fuego.NewServer()

		require.Panics(t, func() {
			fuego.Get(s, "/test", helloWorld,
				QueryInt("age", "Filter by age (in years)", param.Example("3 years old", "3 but string"), param.Nullable()),
			)
		})

		require.Panics(t, func() {
			fuego.Get(s, "/test", helloWorld,
				QueryBool("is_active", "Filter by active status", param.Example("true", 3), param.Nullable()),
			)
		})
	})

	t.Run("panics if default value is not the correct type", func(t *testing.T) {
		s := fuego.NewServer()

		require.Panics(t, func() {
			fuego.Get(s, "/test", helloWorld,
				Query("name", "Filter by name", param.Default(3), param.Nullable()),
			)
		})
	})
}

func TestRequestContentType(t *testing.T) {
	t.Run("Declare a request content type for the route", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Get(s, "/test", dummyController, RequestContentType("application/json"))

		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, "{\"message\":\"hello world\"}\n", w.Body.String())
		require.Len(t, route.AcceptedContentTypes, 1)
		require.Equal(t, "application/json", route.AcceptedContentTypes[0])
	})

	t.Run("base", func(t *testing.T) {
		s := fuego.NewServer()
		route := fuego.Post(s, "/base", dummyController,
			RequestContentType("application/json"),
		)

		t.Log("route.Operation", route.Operation)
		content := route.Operation.RequestBody.Value.Content
		require.NotNil(t, content.Get("application/json"))
		require.Nil(t, content.Get("application/xml"))
		require.Equal(t, "#/components/schemas/ReqBody", content.Get("application/json").Schema.Ref)
		_, ok := s.OpenApiSpec.Components.RequestBodies["ReqBody"]
		require.False(t, ok)
	})

	t.Run("variadic", func(t *testing.T) {
		s := fuego.NewServer()
		route := fuego.Post(s, "/test", dummyController,
			RequestContentType("application/json", "my/content-type"),
		)

		content := route.Operation.RequestBody.Value.Content
		require.NotNil(t, content.Get("application/json"))
		require.NotNil(t, content.Get("my/content-type"))
		require.Nil(t, content.Get("application/xml"))
		require.Equal(t, "#/components/schemas/ReqBody", content.Get("application/json").Schema.Ref)
		require.Equal(t, "#/components/schemas/ReqBody", content.Get("my/content-type").Schema.Ref)
		_, ok := s.OpenApiSpec.Components.RequestBodies["ReqBody"]
		require.False(t, ok)
	})
}

func TestAddError(t *testing.T) {
	t.Run("Declare an error for the route", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Get(s, "/test", helloWorld, AddError(409, "Conflict: Pet with the same name already exists"))

		t.Log("route.Operation.Responses", route.Operation.Responses)
		require.Equal(t, 5, route.Operation.Responses.Len()) // 200, 400, 409, 500, default
		resp := route.Operation.Responses.Value("409")
		require.NotNil(t, resp)
		require.Equal(t, "Conflict: Pet with the same name already exists", *route.Operation.Responses.Value("409").Value.Description)
	})
}

func TestHide(t *testing.T) {
	s := fuego.NewServer()

	fuego.Get(s, "/hidden", helloWorld, Hide())
	fuego.Get(s, "/visible", helloWorld)

	spec := s.OutputOpenAPISpec()
	pathItemVisible := spec.Paths.Find("/visible")
	require.NotNil(t, pathItemVisible)
	pathItemHidden := spec.Paths.Find("/hidden")
	require.Nil(t, pathItemHidden)

	t.Run("visible route works normally", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/visible", nil)
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, 200, w.Code)
		require.Equal(t, "hello world", w.Body.String())
	})

	t.Run("hidden route still accessible even if not in openAPI spec", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/hidden", nil)
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, 200, w.Code)
		require.Equal(t, "hello world", w.Body.String())
	})
}
