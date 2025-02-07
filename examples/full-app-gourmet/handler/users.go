package handler

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"modernc.org/sqlite"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/store"
)

// MyCustomToken is a custom token that contains the standard claims and some custom claims.
type MyCustomToken struct {
	jwt.RegisteredClaims // Required, this struct contains the standard claims
	Username             string
	UserID               string
	Roles                []string
}

var _ jwt.Claims = &MyCustomToken{}

type LoginPayload struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type TokenResponse struct {
	Token string `json:"token"`
}

// Custom login controller
func (rs Resource) login(c fuego.ContextWithBody[LoginPayload]) (TokenResponse, error) {
	body, err := c.Body()
	if err != nil {
		return TokenResponse{}, err
	}

	// Check credentials.
	// In a real application, you should check the credentials against a database.
	if body.Username != "admin" || body.Password != "adminadmin" {
		return TokenResponse{}, fuego.ErrUnauthorized
	}

	myToken := MyCustomToken{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    body.Username,
			Subject:   body.Username,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        "1234567890",
		},
		Roles:    []string{"admin", "cook"},
		Username: "myUsername",
	}

	jwtString, err := rs.Security.GenerateTokenToCookies(myToken, c.Response())

	return TokenResponse{
		Token: jwtString,
	}, err
}

func LoginFunc(user, password string) (jwt.Claims, error) {
	// Check credentials.
	// In a real application, you should check the credentials against a database.
	if user != "admin" || password != "adminadmin" {
		return MyCustomToken{}, fuego.ErrUnauthorized
	}

	return MyCustomToken{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    user,
			Subject:   user,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        "1234567890",
		},
		Roles:    []string{"admin", "cook"},
		Username: "myUsername",
	}, nil
}

type CreateUserPayload struct {
	Username string `json:"username" validate:"required"`
	FullName string `json:"full_name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

func (rs Resource) createUser(c fuego.ContextWithBody[CreateUserPayload]) (*store.User, error) {
	user, err := c.Body()
	if err != nil {
		return nil, err
	}

	createdUser, err := rs.UsersQueries.CreateUser(c.Context(), store.CreateUserParams{
		Username:          user.Username,
		FullName:          user.FullName,
		Email:             user.Email,
		EncryptedPassword: user.Password + "-encrypted",
	})
	if err != nil {
		var sqliteError *sqlite.Error
		if errors.As(err, &sqliteError) {
			if sqliteError.Code() == 2067 || sqliteError.Code() == 1555 {
				return nil, fuego.ConflictError{Title: "Duplicate", Detail: sqliteError.Error(), Err: errors.New(sqliteError.Error())}
			}
		}
		return nil, err
	}

	return &createdUser, nil
}

func (rs Resource) getUserByUsername(c fuego.ContextNoBody) (store.User, error) {
	username := c.PathParam("username")

	return rs.UsersQueries.GetUserByUsername(c.Context(), username)
}

type UserRepository interface {
	CreateUser(ctx context.Context, arg store.CreateUserParams) (store.User, error)
	GetUserByUsername(ctx context.Context, username string) (store.User, error)
}
