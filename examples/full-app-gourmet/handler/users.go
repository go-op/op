package handler

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/go-fuego/fuego"
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
