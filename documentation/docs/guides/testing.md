# Testing Fuego Controllers

Fuego provides a `MockContext` type that makes it easy to test your controllers without using httptest, allowing you to focus on your business logic instead of the HTTP layer.

## Using MockContext

The `MockContext` type implements the `ContextWithBody` interface. Here's a simple example:

```go
func TestMyController(t *testing.T) {
    // Create a new mock context with the request body
    ctx := fuego.NewMockContext(MyRequestType{
        Name: "John",
        Age:  30,
    })

    // Add query parameters
    ctx.SetQueryParamInt("page", 1)

    // Call your controller
    response, err := MyController(ctx)

    // Assert the results
    assert.NoError(t, err)
    assert.Equal(t, expectedResponse, response)
}
```

## Complete Example

Here's a more complete example showing how to test a controller that uses request body, query parameters, and validation:

```go
// UserSearchRequest represents the search criteria
type UserSearchRequest struct {
    MinAge    int    `json:"minAge" validate:"gte=0,lte=150"`
    MaxAge    int    `json:"maxAge" validate:"gte=0,lte=150"`
    NameQuery string `json:"nameQuery" validate:"required"`
}

// SearchUsersController is our controller to test
func SearchUsersController(c fuego.ContextWithBody[UserSearchRequest]) (UserSearchResponse, error) {
    body, err := c.Body()
    if err != nil {
        return UserSearchResponse{}, err
    }

    // Get pagination from query params
    page := c.QueryParamInt("page")
    if page < 1 {
        page = 1
    }

    // Business logic validation
    if body.MinAge > body.MaxAge {
        return UserSearchResponse{}, errors.New("minAge cannot be greater than maxAge")
    }

    // ... rest of the controller logic
}

func TestSearchUsersController(t *testing.T) {
    tests := []struct {
        name          string
        body          UserSearchRequest
        setupContext  func(*fuego.MockContext[UserSearchRequest])
        expectedError string
        expected      UserSearchResponse
    }{
        {
            name: "successful search",
            body: UserSearchRequest{
                MinAge:    20,
                MaxAge:    35,
                NameQuery: "John",
            },
            setupContext: func(ctx *fuego.MockContext[UserSearchRequest]) {
                // Add query parameters with OpenAPI validation
                ctx.WithQueryParamInt("page", 1,
                    fuego.ParamDescription("Page number"),
                    fuego.ParamDefault(1))
                ctx.WithQueryParamInt("perPage", 20,
                    fuego.ParamDescription("Items per page"),
                    fuego.ParamDefault(20))
            },
            expected: UserSearchResponse{
                // ... expected response
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create mock context with the test body
            ctx := fuego.NewMockContext[UserSearchRequest](tt.body)

            // Set up context with query parameters
            if tt.setupContext != nil {
                tt.setupContext(ctx)
            }

            // Call the controller
            response, err := SearchUsersController(ctx)

            // Check error cases
            if tt.expectedError != "" {
                assert.EqualError(t, err, tt.expectedError)
                return
            }

            // Check success cases
            assert.NoError(t, err)
            assert.Equal(t, tt.expected, response)
        })
    }
}
```

## Best Practices

1. **Test Edge Cases**: Test both valid and invalid inputs, including validation errors.
2. **Use Table-Driven Tests**: Structure your tests as a slice of test cases for better organization.
3. **Mock using interfaces**: Use interfaces to mock dependencies and make your controllers testable.
4. **Test Business Logic**: Focus on testing your business logic rather than the framework itself.
5. **Fuzz Testing**: Use fuzz testing to automatically find edge cases that you might have missed.
