package fuego_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"
	"github.com/go-fuego/fuego/param"
)

// UserSearchRequest represents the search criteria for users
type UserSearchRequest struct {
	MinAge    int    `json:"minAge" validate:"gte=0,lte=150"`
	MaxAge    int    `json:"maxAge" validate:"gte=0,lte=150"`
	NameQuery string `json:"nameQuery" validate:"required"`
}

// UserSearchResponse represents the search results
type UserSearchResponse struct {
	Users       []UserProfile `json:"users"`
	TotalCount  int           `json:"totalCount"`
	CurrentPage int           `json:"currentPage"`
}

// UserProfile represents a user in the system
type UserProfile struct {
	ID    string `json:"id"`
	Name  string `json:"name" validate:"required"`
	Age   int    `json:"age" validate:"gte=0,lte=150"`
	Email string `json:"email" validate:"required,email"`
}

// SearchUsersController is an example of a real controller that would be used in a Fuego app
func SearchUsersController(c fuego.ContextWithBody[UserSearchRequest]) (UserSearchResponse, error) {
	// Get and validate the request body
	body, err := c.Body()
	if err != nil {
		return UserSearchResponse{}, err
	}

	// Get pagination parameters from query
	page := c.QueryParamInt("page")
	if page < 1 {
		page = 1
	}
	perPage := c.QueryParamInt("perPage")
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	// Example validation beyond struct tags
	if body.MinAge > body.MaxAge {
		return UserSearchResponse{}, errors.New("minAge cannot be greater than maxAge")
	}

	// In a real app, this would query a database
	// Here we just return mock data that matches the criteria
	users := []UserProfile{
		{ID: "user_1", Name: "John Doe", Age: 25, Email: "john@example.com"},
		{ID: "user_2", Name: "Jane Smith", Age: 30, Email: "jane@example.com"},
	}

	// Filter users based on criteria (simplified example)
	var filteredUsers []UserProfile
	for _, user := range users {
		if user.Age >= body.MinAge && user.Age <= body.MaxAge {
			filteredUsers = append(filteredUsers, user)
		}
	}

	return UserSearchResponse{
		Users:       filteredUsers,
		TotalCount:  len(filteredUsers),
		CurrentPage: page,
	}, nil
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
			name: "successful search with age range",
			body: UserSearchRequest{
				MinAge:    20,
				MaxAge:    35,
				NameQuery: "John",
			},
			setupContext: func(ctx *fuego.MockContext[UserSearchRequest]) {
				ctx.SetQueryParamInt("page", 1)
				ctx.SetQueryParamInt("perPage", 20)
			},
			expected: UserSearchResponse{
				Users: []UserProfile{
					{ID: "user_1", Name: "John Doe", Age: 25, Email: "john@example.com"},
					{ID: "user_2", Name: "Jane Smith", Age: 30, Email: "jane@example.com"},
				},
				TotalCount:  2,
				CurrentPage: 1,
			},
		},
		{
			name: "invalid age range",
			body: UserSearchRequest{
				MinAge:    40,
				MaxAge:    20,
				NameQuery: "John",
			},
			expectedError: "minAge cannot be greater than maxAge",
		},
		{
			name: "default pagination values",
			body: UserSearchRequest{
				MinAge:    20,
				MaxAge:    35,
				NameQuery: "John",
			},
			expected: UserSearchResponse{
				Users: []UserProfile{
					{ID: "user_1", Name: "John Doe", Age: 25, Email: "john@example.com"},
					{ID: "user_2", Name: "Jane Smith", Age: 30, Email: "jane@example.com"},
				},
				TotalCount:  2,
				CurrentPage: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock context with the test body
			ctx := fuego.NewMockContext(tt.body)

			// Set up context with query parameters if provided
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

func TestMockContextNoBody(t *testing.T) {
	myController := func(c fuego.ContextNoBody) (string, error) {
		return "Hello, " + c.QueryParam("name"), nil
	}

	// Just check that `myController` is indeed an acceptable Fuego controller
	s := fuego.NewServer()
	fuego.Get(s, "/route", myController,
		option.Query("name", "Name given to be greeted", param.Default("World")),
	)

	t.Run("TestMockContextNoBody", func(t *testing.T) {
		ctx := fuego.NewMockContextNoBody()
		assert.NotNil(t, ctx)

		ctx.SetQueryParam("name", "You")

		// Call the controller
		response, err := myController(ctx)

		require.NoError(t, err)
		require.Equal(t, "Hello, You", response)
	})

	t.Run("Does not use the default params from the route declaration", func(t *testing.T) {
		ctx := fuego.NewMockContextNoBody()
		assert.NotNil(t, ctx)

		// Call the controller
		response, err := myController(ctx)

		require.NoError(t, err)
		require.Equal(t, "Hello, ", response)
	})
}
