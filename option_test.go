package fuego_test

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
	"github.com/thejerf/slogassert"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"
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

func helloWorld(ctx fuego.ContextNoBody) (string, error) {
	return "hello world", nil
}

type ReqBody struct {
	A string
	B int
}

type Resp struct {
	Message string `json:"message"`
}

func dummyController(_ fuego.ContextWithBody[ReqBody]) (Resp, error) {
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

	fuego.Get(s, "/withMiddleware", func(ctx fuego.ContextNoBody) (string, error) {
		return "withmiddleware", nil
	}, fuego.OptionMiddleware(dummyMiddleware))

	fuego.Get(s, "/withoutMiddleware", func(ctx fuego.ContextNoBody) (string, error) {
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
		fuego.Get(s, "/test", func(ctx fuego.ContextNoBody) (string, error) {
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
		fuego.Get(s, "/test", func(ctx fuego.ContextNoBody) (string, error) {
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
		fuego.Get(s, "/test", func(ctx fuego.ContextNoBody) (string, error) {
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
		fuego.Get(s, "/test", func(ctx fuego.ContextNoBody) (string, error) {
			return "test", nil
		},
			fuego.OptionMiddleware(orderMiddleware("Fourth!")),
			fuego.OptionMiddleware(orderMiddleware("Fifth!")),
		)

		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.Header.Set("X-Test-Order", "Start!")
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, []string{"Start!", "First!", "Second!", "Third!", "Fourth!", "Fifth!"}, r.Header["X-Test-Order"])
	})
}

type ans struct{}

func TestOptions(t *testing.T) {
	t.Run("warn if param is not found in openAPI config but called in controller (possibly typo)", func(t *testing.T) {
		handler := slogassert.New(t, slog.LevelWarn, nil)

		s := fuego.NewServer(
			fuego.WithLogHandler(handler),
		)

		fuego.Get(s, "/correct", func(c fuego.ContextNoBody) (ans, error) {
			c.QueryParam("quantity")
			return ans{}, nil
		},
			fuego.OptionQuery("quantity", "some description"),
			fuego.OptionQueryInt("number", "some description", param.Example("3", 3)),
			fuego.OptionQueryBool("is_active", "some description"),
		)

		fuego.Get(s, "/typo", func(c fuego.ContextNoBody) (ans, error) {
			c.QueryParam("quantityy-with-a-typo")
			return ans{}, nil
		},
			fuego.OptionQuery("quantity", "some description"),
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
			fuego.OptionHeader("X-Test", "test header", param.Required(), param.Example("test", "My Header"), param.Default("test")),
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
			fuego.OptionSummary("test summary"),
			fuego.OptionDescription("test description"),
			fuego.OptionTags("first-tag", "second-tag"),
			fuego.OptionDeprecated(),
			fuego.OptionOperationID("test-operation-id"),
		)

		require.Equal(t, "test summary", route.Operation.Summary)
		require.Equal(t, "#### Controller: \n\n`github.com/go-fuego/fuego_test.helloWorld`\n\n---\n\ntest description", route.Operation.Description)
		require.Equal(t, []string{"first-tag", "second-tag"}, route.Operation.Tags)
		require.True(t, route.Operation.Deprecated)
	})
}

func TestGroup(t *testing.T) {
	paramsGroup := fuego.GroupOptions(
		fuego.OptionHeader("X-Test", "test header", param.Required(), param.Example("test", "My Header"), param.Default("test")),
		fuego.OptionQuery("name", "Filter by name", param.Example("cat name", "felix"), param.Nullable()),
		fuego.OptionCookie("session", "Session cookie", param.Example("session", "1234"), param.Nullable()),
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
				fuego.OptionQueryInt("age", "Filter by age (in years)", param.Example("3 years old", "3 but string"), param.Nullable()),
			)
		})

		require.Panics(t, func() {
			fuego.Get(s, "/test", helloWorld,
				fuego.OptionQueryBool("is_active", "Filter by active status", param.Example("true", 3), param.Nullable()),
			)
		})
	})

	t.Run("panics if default value is not the correct type", func(t *testing.T) {
		s := fuego.NewServer()

		require.Panics(t, func() {
			fuego.Get(s, "/test", helloWorld,
				fuego.OptionQuery("name", "Filter by name", param.Default(3), param.Nullable()),
			)
		})
	})
}

func TestPath(t *testing.T) {
	t.Run("Path parameter is automatically declared for the route", func(t *testing.T) {
		s := fuego.NewServer()

		fuego.Get(s, "/test/{id}", helloWorld)

		require.Equal(t, "id", s.OpenAPI.Description().Paths.Find("/test/{id}").Get.Parameters.GetByInAndName("path", "id").Name)
		require.Equal(t, "", s.OpenAPI.Description().Paths.Find("/test/{id}").Get.Parameters.GetByInAndName("path", "id").Description)
	})

	t.Run("Declare explicitly an existing path parameter for the route", func(t *testing.T) {
		s := fuego.NewServer()

		fuego.Get(s, "/test/{id}", helloWorld,
			fuego.OptionPath("id", "some id", param.Example("123", "123"), param.Nullable()),
		)

		require.Equal(t, "id", s.OpenAPI.Description().Paths.Find("/test/{id}").Get.Parameters.GetByInAndName("path", "id").Name)
		require.Equal(t, "some id", s.OpenAPI.Description().Paths.Find("/test/{id}").Get.Parameters.GetByInAndName("path", "id").Description)
		require.Equal(t, true, s.OpenAPI.Description().Paths.Find("/test/{id}").Get.Parameters.GetByInAndName("path", "id").Required, "path parameter is forced to be required")
	})

	t.Run("Declare explicitly a non-existing path parameter for the route panics", func(t *testing.T) {
		s := fuego.NewServer()

		require.Panics(t, func() {
			fuego.Get(s, "/test/{id}", helloWorld,
				fuego.OptionPath("not-existing-in-url", "some id"),
			)
		})
	})
}

func TestRequestContentType(t *testing.T) {
	t.Run("Declare a request content type for the route", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Get(s, "/test", dummyController, fuego.OptionRequestContentType("application/json"))

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
			fuego.OptionRequestContentType("application/json"),
		)

		t.Log("route.Operation", route.Operation)
		content := route.Operation.RequestBody.Value.Content
		require.NotNil(t, content.Get("application/json"))
		require.Nil(t, content.Get("application/xml"))
		require.Equal(t, "#/components/schemas/ReqBody", content.Get("application/json").Schema.Ref)
		_, ok := s.OpenAPI.Description().Components.RequestBodies["ReqBody"]
		require.False(t, ok)
	})

	t.Run("variadic", func(t *testing.T) {
		s := fuego.NewServer()
		route := fuego.Post(s, "/test", dummyController,
			fuego.OptionRequestContentType("application/json", "my/content-type"),
		)

		content := route.Operation.RequestBody.Value.Content
		require.NotNil(t, content.Get("application/json"))
		require.NotNil(t, content.Get("my/content-type"))
		require.Nil(t, content.Get("application/xml"))
		require.Equal(t, "#/components/schemas/ReqBody", content.Get("application/json").Schema.Ref)
		require.Equal(t, "#/components/schemas/ReqBody", content.Get("my/content-type").Schema.Ref)
		_, ok := s.OpenAPI.Description().Components.RequestBodies["ReqBody"]
		require.False(t, ok)
	})

	t.Run("override server", func(t *testing.T) {
		s := fuego.NewServer(fuego.WithRequestContentType("application/json", "application/xml"))
		route := fuego.Post(
			s, "/test", dummyController,
			fuego.OptionRequestContentType("my/content-type"),
		)

		content := route.Operation.RequestBody.Value.Content
		require.Nil(t, content.Get("application/json"))
		require.Nil(t, content.Get("application/xml"))
		require.NotNil(t, content.Get("my/content-type"))
		require.Equal(t, "#/components/schemas/ReqBody", content.Get("my/content-type").Schema.Ref)
		_, ok := s.OpenAPI.Description().Components.RequestBodies["ReqBody"]
		require.False(t, ok)
	})
}

func TestAddError(t *testing.T) {
	t.Run("Declare an error for the route", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Get(s, "/test", helloWorld, fuego.OptionAddError(http.StatusConflict, "Conflict: Pet with the same name already exists"))

		t.Log("route.Operation.Responses", route.Operation.Responses)
		require.Equal(t, 5, route.Operation.Responses.Len()) // 200, 400, 409, 500, default
		resp := route.Operation.Responses.Value("409")
		require.NotNil(t, resp)
		require.Equal(t, "Conflict: Pet with the same name already exists", *route.Operation.Responses.Value("409").Value.Description)
	})

	t.Run("should be fatal", func(t *testing.T) {
		s := fuego.NewServer()

		require.Panics(t, func() {
			fuego.Get(s, "/test", helloWorld, fuego.OptionAddError(http.StatusConflict, "err", Resp{}, Resp{}))
		})
	})
}

func TestAddResponse(t *testing.T) {
	t.Run("base", func(t *testing.T) {
		s := fuego.NewServer()
		route := fuego.Get(s, "/test", helloWorld, fuego.OptionAddResponse(
			http.StatusConflict,
			"Conflict: Pet with the same name already exists",
			fuego.Response{
				ContentTypes: []string{"application/json"},
				Type:         fuego.HTTPError{},
			},
		))
		require.Equal(t, 5, route.Operation.Responses.Len()) // 200, 400, 409, 500, default
		resp := route.Operation.Responses.Value("409")
		require.NotNil(t, resp)
		require.NotNil(t, resp.Value.Content.Get("application/json"))
		require.Nil(t, resp.Value.Content.Get("application/xml"))
		require.Equal(t, "Conflict: Pet with the same name already exists", *route.Operation.Responses.Value("409").Value.Description)
	})

	t.Run("no content types provided", func(t *testing.T) {
		s := fuego.NewServer()
		route := fuego.Get(s, "/test", helloWorld, fuego.OptionAddResponse(
			http.StatusConflict,
			"Conflict: Pet with the same name already exists",
			fuego.Response{
				Type: fuego.HTTPError{},
			},
		))
		require.Equal(t, 5, route.Operation.Responses.Len()) // 200, 400, 409, 500, default
		resp := route.Operation.Responses.Value("409")
		require.NotNil(t, resp)
		require.NotNil(t, resp.Value.Content.Get("application/json"))
		require.NotNil(t, resp.Value.Content.Get("application/xml"))
		require.Equal(t, "Conflict: Pet with the same name already exists", *route.Operation.Responses.Value("409").Value.Description)
	})

	t.Run("should override 200", func(t *testing.T) {
		s := fuego.NewServer()
		route := fuego.Get(s, "/test", helloWorld, fuego.OptionAddResponse(
			http.StatusOK,
			"set 200",
			fuego.Response{
				Type:         fuego.HTTPError{},
				ContentTypes: []string{"application/x-yaml"},
			},
		))
		require.Equal(t, 4, route.Operation.Responses.Len()) // 200, 400, 500, default
		resp := route.Operation.Responses.Value("200")
		require.NotNil(t, resp)
		require.Nil(t, resp.Value.Content.Get("application/json"))
		require.Nil(t, resp.Value.Content.Get("application/xml"))
		require.NotNil(t, resp.Value.Content.Get("application/x-yaml"))
		require.Equal(t, "#/components/schemas/HTTPError", resp.Value.Content.Get("application/x-yaml").Schema.Ref)
		require.Equal(t, "set 200", *route.Operation.Responses.Value("200").Value.Description)
	})

	t.Run("should be fatal", func(t *testing.T) {
		s := fuego.NewServer()

		require.Panics(t, func() {
			fuego.Get(s, "/test", helloWorld, fuego.OptionAddResponse(
				http.StatusConflict,
				"Conflict: Pet with the same name already exists",
				fuego.Response{},
			))
		})
	})
}

func TestHide(t *testing.T) {
	s := fuego.NewServer()

	fuego.Get(s, "/hidden", helloWorld, fuego.OptionHide())
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

func TestOptionResponseHeader(t *testing.T) {
	t.Run("Declare a response header for the route", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Get(s, "/test", helloWorld,
			fuego.OptionResponseHeader("X-Test", "test header", param.Example("test", "My Header"), param.Default("test"), param.Description("test description")),
		)

		require.NotNil(t, route.Operation.Responses.Value("200").Value.Headers["X-Test"])
		require.Equal(t, "My Header", route.Operation.Responses.Value("200").Value.Headers["X-Test"].Value.Examples["test"].Value.Value)
		require.Equal(t, "test description", route.Operation.Responses.Value("200").Value.Headers["X-Test"].Value.Description)
	})

	t.Run("Declare a response header for the route with multiple status codes", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Get(s, "/test", helloWorld,
			fuego.OptionResponseHeader("X-Test", "test header", param.StatusCodes(200, 206)),
		)

		require.NotNil(t, route.Operation.Responses.Value("200").Value.Headers["X-Test"])
		require.NotNil(t, route.Operation.Responses.Value("206").Value.Headers["X-Test"])
		require.Nil(t, route.Operation.Responses.Value("400").Value.Headers["X-Test"])
	})
}

func TestSecurity(t *testing.T) {
	t.Run("single security requirement with defined scheme", func(t *testing.T) {
		s := fuego.NewServer(
			fuego.WithSecurity(openapi3.SecuritySchemes{
				"basic": &openapi3.SecuritySchemeRef{
					Value: openapi3.NewSecurityScheme().
						WithType("http").
						WithScheme("basic"),
				},
			}),
		)

		basic := openapi3.SecurityRequirement{
			"basic": []string{},
		}
		route := fuego.Get(s, "/test", helloWorld,
			fuego.OptionSecurity(basic),
		)

		require.NotNil(t, route.Operation.Security)
		require.Len(t, *route.Operation.Security, 1)
		require.Contains(t, (*route.Operation.Security)[0], "basic")
		require.Empty(t, (*route.Operation.Security)[0]["basic"])

		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		s.Mux.ServeHTTP(w, r)
		require.Equal(t, "hello world", w.Body.String())
	})

	t.Run("security with scopes and defined scheme", func(t *testing.T) {
		s := fuego.NewServer(
			fuego.WithSecurity(openapi3.SecuritySchemes{
				"oauth2": &openapi3.SecuritySchemeRef{
					Value: &openapi3.SecurityScheme{
						Type: "oauth2",
						Flows: &openapi3.OAuthFlows{
							AuthorizationCode: &openapi3.OAuthFlow{
								AuthorizationURL: "https://example.com/oauth/authorize",
								TokenURL:         "https://example.com/oauth/token",
								Scopes: map[string]string{
									"read:users": "Read user information",
								},
							},
						},
					},
				},
			}),
		)

		route := fuego.Get(s, "/test", helloWorld,
			fuego.OptionSecurity(
				openapi3.SecurityRequirement{
					"oauth2": []string{"read:users", "write:users"},
				},
			),
		)

		require.NotNil(t, route.Operation.Security)
		require.Len(t, *route.Operation.Security, 1)
		require.Contains(t, (*route.Operation.Security)[0], "oauth2")
		require.Equal(t,
			[]string{"read:users", "write:users"},
			(*route.Operation.Security)[0]["oauth2"],
		)
	})

	t.Run("AND combination with defined schemes", func(t *testing.T) {
		s := fuego.NewServer(
			fuego.WithSecurity(openapi3.SecuritySchemes{
				"basic": &openapi3.SecuritySchemeRef{
					Value: openapi3.NewSecurityScheme().
						WithType("http").
						WithScheme("basic"),
				},
				"oauth2": &openapi3.SecuritySchemeRef{
					Value: &openapi3.SecurityScheme{
						Type: "oauth2",
						Flows: &openapi3.OAuthFlows{
							AuthorizationCode: &openapi3.OAuthFlow{
								AuthorizationURL: "https://example.com/oauth/authorize",
								TokenURL:         "https://example.com/oauth/token",
								Scopes: map[string]string{
									"read:users": "Read user information",
								},
							},
						},
					},
				},
			}),
		)

		route := fuego.Get(s, "/test", helloWorld,
			fuego.OptionSecurity(
				openapi3.SecurityRequirement{
					"basic":  []string{},
					"oauth2": []string{"read:users"},
				},
			),
		)

		require.NotNil(t, route.Operation.Security)
		require.Len(t, *route.Operation.Security, 1)
		require.Contains(t, (*route.Operation.Security)[0], "basic")
		require.Empty(t, (*route.Operation.Security)[0]["basic"])
		require.Contains(t, (*route.Operation.Security)[0], "oauth2")
		require.Equal(t, []string{"read:users"}, (*route.Operation.Security)[0]["oauth2"])
	})

	t.Run("OR combination with defined schemes", func(t *testing.T) {
		s := fuego.NewServer(
			fuego.WithSecurity(openapi3.SecuritySchemes{
				"basic": &openapi3.SecuritySchemeRef{
					Value: openapi3.NewSecurityScheme().
						WithType("http").
						WithScheme("basic"),
				},
				"oauth2": &openapi3.SecuritySchemeRef{
					Value: &openapi3.SecurityScheme{
						Type: "oauth2",
						Flows: &openapi3.OAuthFlows{
							AuthorizationCode: &openapi3.OAuthFlow{
								AuthorizationURL: "https://example.com/oauth/authorize",
								TokenURL:         "https://example.com/oauth/token",
								Scopes: map[string]string{
									"read:users": "Read user information",
								},
							},
						},
					},
				},
			}),
		)

		route := fuego.Get(s, "/test", helloWorld,
			fuego.OptionSecurity(
				openapi3.SecurityRequirement{
					"basic": []string{},
				},
				openapi3.SecurityRequirement{
					"oauth2": []string{"read:users"},
				},
			),
		)

		require.NotNil(t, route.Operation.Security)
		require.Len(t, *route.Operation.Security, 2)
		require.Contains(t, (*route.Operation.Security)[0], "basic")
		require.Empty(t, (*route.Operation.Security)[0]["basic"])
		require.Contains(t, (*route.Operation.Security)[1], "oauth2")
		require.Equal(t, []string{"read:users"}, (*route.Operation.Security)[1]["oauth2"])
	})

	t.Run("panic on undefined security scheme", func(t *testing.T) {
		s := fuego.NewServer()

		require.Panics(t, func() {
			fuego.Get(s, "/test", helloWorld,
				fuego.OptionSecurity(
					openapi3.SecurityRequirement{
						"undefined": []string{},
					},
				),
			)
		})
	})

	t.Run("panic on partially undefined schemes", func(t *testing.T) {
		s := fuego.NewServer(
			fuego.WithSecurity(openapi3.SecuritySchemes{
				"basic": &openapi3.SecuritySchemeRef{
					Value: openapi3.NewSecurityScheme().
						WithType("http").
						WithScheme("basic"),
				},
			}),
		)

		require.Panics(t, func() {
			fuego.Get(s, "/test", helloWorld,
				fuego.OptionSecurity(
					openapi3.SecurityRequirement{
						"basic":     []string{},
						"undefined": []string{},
					},
				),
			)
		})
	})

	t.Run("empty security options", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Get(s, "/test", helloWorld,
			fuego.OptionSecurity(),
		)

		require.NotNil(t, route.Operation.Security)
		require.Empty(t, (*route.Operation.Security))
	})

	t.Run("multiple security options with different scopes", func(t *testing.T) {
		s := fuego.NewServer(
			fuego.WithSecurity(openapi3.SecuritySchemes{
				"Bearer": &openapi3.SecuritySchemeRef{
					Value: openapi3.NewSecurityScheme().
						WithType("http").
						WithScheme("bearer"),
				},
				"ApiKey": &openapi3.SecuritySchemeRef{
					Value: openapi3.NewSecurityScheme().
						WithType("apiKey").
						WithIn("header").
						WithName("X-API-Key"),
				},
			}),
		)

		route := fuego.Get(s, "/test", helloWorld,
			fuego.OptionSecurity(
				openapi3.SecurityRequirement{
					"Bearer": []string{"read"},
					"ApiKey": []string{"basic"},
				},
			),
		)

		require.NotNil(t, route.Operation.Security)
		require.Len(t, *route.Operation.Security, 1)

		security := (*route.Operation.Security)[0]
		require.Contains(t, security, "Bearer")
		require.Equal(t, []string{"read"}, security["Bearer"])
		require.Contains(t, security, "ApiKey")
		require.Equal(t, []string{"basic"}, security["ApiKey"])
	})
}

func TestOptionDescription(t *testing.T) {
	t.Run("Declare a description for the route", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Get(s, "/test", helloWorld,
			option.Description("test description"),
		)

		require.Equal(t, "#### Controller: \n\n`github.com/go-fuego/fuego_test.helloWorld`\n\n---\n\ntest description", route.Operation.Description)
	})

	t.Run("Override Fuego's description for the route", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Get(s, "/test", helloWorld,
			option.OverrideDescription("another description"),
		)

		require.Equal(t, "another description", route.Operation.Description)
	})

	t.Run("Add description to the route, route middleware is included", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Get(s, "/test", helloWorld,
			option.Middleware(dummyMiddleware),
			option.Description("another description"),
		)

		require.Equal(t, "#### Controller: \n\n`github.com/go-fuego/fuego_test.helloWorld`\n\n#### Middlewares:\n\n- `github.com/go-fuego/fuego_test.dummyMiddleware`\n\n---\n\nanother description", route.Operation.Description)
	})

	t.Run("Add description to the route, route middleware is included", func(t *testing.T) {
		s := fuego.NewServer()

		fuego.Use(s, dummyMiddleware)

		group := fuego.Group(s, "/group", option.Middleware(dummyMiddleware))

		fuego.Use(group, dummyMiddleware)

		route := fuego.Get(s, "/test", helloWorld,
			option.Middleware(dummyMiddleware),
			option.Description("another description"),
			option.Middleware(dummyMiddleware), // After the description
			option.Middleware(dummyMiddleware), // 6th middleware
			option.Middleware(dummyMiddleware), // 7th middleware, should not be included
		)

		require.Equal(t, "#### Controller: \n\n`github.com/go-fuego/fuego_test.helloWorld`\n\n#### Middlewares:\n\n- `github.com/go-fuego/fuego_test.dummyMiddleware`\n- `github.com/go-fuego/fuego_test.dummyMiddleware`\n- `github.com/go-fuego/fuego_test.dummyMiddleware`\n- `github.com/go-fuego/fuego_test.dummyMiddleware`\n- `github.com/go-fuego/fuego_test.dummyMiddleware`\n- more middleware...\n\n---\n\nanother description", route.Operation.Description)
	})
}

func TestDefaultStatusCode(t *testing.T) {
	t.Run("Declare a default status code for the route", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Post(s, "/test", helloWorld,
			option.DefaultStatusCode(201),
		)

		r := httptest.NewRequest(http.MethodPost, "/test", nil)
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, 201, w.Code)
		require.Equal(t, "hello world", w.Body.String())
		require.Equal(t, 201, route.DefaultStatusCode)
		require.NotNil(t, route.Operation.Responses.Value("201").Value)
	})

	t.Run("Declare a default status code for the route but bypass it in the controller", func(t *testing.T) {
		s := fuego.NewServer()

		route := fuego.Post(s, "/test", func(c fuego.ContextNoBody) (string, error) {
			c.SetStatus(200)
			return "hello world", nil
		},
			option.DefaultStatusCode(201),
		)

		r := httptest.NewRequest(http.MethodPost, "/test", nil)
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, 200, w.Code)
		require.Equal(t, "hello world", w.Body.String())
		require.Equal(t, 201, route.DefaultStatusCode, "default status code should not be changed")
		require.NotNil(t, route.Operation.Responses.Value("201").Value, "default status is still in the spec even if code is not used")
	})

	t.Run("can return 204 when no data is being sent", func(t *testing.T) {
		s := fuego.NewServer()

		fuego.Get(s, "/", func(_ fuego.ContextNoBody) (any, error) {
			return nil, nil
		},
			option.DefaultStatusCode(204),
		)

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, 204, w.Code)
	})

	t.Run("must return 500 when an error is being sent, even with no body", func(t *testing.T) {
		s := fuego.NewServer()

		fuego.Get(s, "/", func(_ fuego.ContextNoBody) (any, error) {
			return nil, errors.New("error")
		},
			option.DefaultStatusCode(204),
		)

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		s.Mux.ServeHTTP(w, r)

		require.Equal(t, 500, w.Code)
	})
}
