package fuego

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrUnauthorized     = errors.New("unauthorized")
	ErrTokenNotFound    = errors.New("token not found")
	ErrInvalidTokenType = errors.New("invalid token type")
	ErrInvalidRolesType = errors.New("invalid role type. Must be []string")
	ErrExpired          = errors.New("token is expired")
)

// Security holds the key to sign the JWT tokens, and configuration information.
// The key isn't accessible once created to avoid leaking it.
// To use it, please use the methods provided.
type Security struct {
	key             *ecdsa.PrivateKey
	Now             func() time.Time
	ExpiresInterval time.Duration
}

func NewSecurity() Security {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	return Security{
		key:             key,
		Now:             time.Now,
		ExpiresInterval: 24 * time.Hour,
	}
}

// GenerateToken generates a JWT token with the given claims.
// The claims must be a jwt.MapClaims or embed jwt.RegisteredClaims.
func (security Security) GenerateToken(claims jwt.Claims) (token string, err error) {
	if _, ok := claims.(jwt.MapClaims); ok {
		claims.(jwt.MapClaims)["iat"] = security.Now().Unix()
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)

	return tok.SignedString(security.key)
}

// GenerateTokenToCookies generates a JWT token with the given claims and writes it to the cookies.
func (security Security) GenerateTokenToCookies(claims jwt.Claims, w http.ResponseWriter) (string, error) {
	token, err := security.GenerateToken(claims)
	if err != nil {
		return "", err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     JWTCookieName,
		Value:    token,
		Expires:  security.Now().Add(security.ExpiresInterval),
		HttpOnly: true,
		// SameSite: http.SameSiteStrictMode,
		// Secure:   true,
		MaxAge: int(security.ExpiresInterval.Seconds()),
	})

	return token, nil
}

func (security Security) ValidateToken(token string) (*jwt.Token, error) {
	t, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return security.key.Public(), nil
	},
		jwt.WithStrictDecoding(),
		jwt.WithValidMethods([]string{"ES256"}),
		jwt.WithLeeway(5*time.Second),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return nil, err
	}

	iat, err := t.Claims.GetIssuedAt()
	if err != nil || iat == nil || float64(iat.Unix())+security.ExpiresInterval.Seconds() < float64(security.Now().Unix()) {
		return nil, ErrExpired
	}

	return t, nil
}

type AutoAuthConfig struct {
	Enabled        bool
	VerifyUserInfo func(user, password string) (jwt.Claims, error) // Must check the username and password, and return the claims
}

type contextKey string

const (
	contextKeyJWT contextKey = "jwtInfo"
)

func WithValue(ctx context.Context, val any) context.Context {
	return context.WithValue(ctx, contextKeyJWT, val)
}

// TokenFromContext returns the validated token from the context, if found.
// To check if the user is authorized, use the [AuthWall] middleware, or create your own middleware.
// Even though it returns a jwt.MapClaims, the real underlying type is the one you chose when calling [Security.GenerateToken].
// Example:
//
//	token, err := fuego.TokenFromContext[MyCustomTokenType](ctx.Context())
func TokenFromContext(ctx context.Context) (jwt.Claims, error) {
	value := ctx.Value(contextKeyJWT)
	if value == nil {
		return nil, ErrTokenNotFound
	}
	claims, ok := value.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidTokenType
	}

	return claims, nil
}

// GetToken returns the validated token from the context, if found.
// To check if the user is authorized, use the [AuthWall] middleware, or create your own middleware.
// Example:
//
//	token, err := fuego.GetToken[MyCustomTokenType](ctx.Context())
func GetToken[T any](ctx context.Context) (T, error) {
	var myClaims T
	claims, err := TokenFromContext(ctx)
	if err != nil {
		return myClaims, err
	}

	myClaims, ok := claims.(T)
	if !ok {
		return myClaims, ErrInvalidTokenType
	}

	return myClaims, nil
}

func TokenFromHeader(r *http.Request) string {
	// Get the authorizationHeader from the header
	authorizationHeader := r.Header.Get("Authorization")
	if authorizationHeader == "" {
		return ""
	}

	if len(authorizationHeader) < 7 {
		return ""
	}

	if !strings.HasPrefix(authorizationHeader, "Bearer ") {
		return ""
	}

	// Removes "Bearer " from the header
	return strings.TrimSpace(authorizationHeader[7:])
}

const JWTCookieName = "jwt_token"

func TokenFromCookie(r *http.Request) string {
	cookie, err := r.Cookie(JWTCookieName)
	if err != nil {
		return ""
	}

	return cookie.Value
}

func TokenFromQueryParam(r *http.Request) string {
	return r.FormValue("jwt")
}

// TokenToContext is a middleware that checks if the user is authenticated from various authentication methods.
// Once found, the token is parsed, validated and the claims are set in the context.
// TLDR: after this middleware, the token is either non-existent or validated.
// You can use [TokenFromContext] to get the claims
func (security Security) TokenToContext(searchFunc ...func(*http.Request) string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the authorizationHeader from the header
			token := ""
			for _, f := range searchFunc {
				token = f(r)
				if token != "" {
					break
				}
			}

			if token == "" {
				// Unauthenticated, might be legit
				next.ServeHTTP(w, r)
				return
			}

			// Validate the token
			t, err := security.ValidateToken(token)
			if err != nil {
				SendJSONError(w, nil, err)
				return
			}

			// Get the claims
			claims := t.Claims.(jwt.MapClaims)

			// Set the subject and roles in the context
			ctx := r.Context()
			ctx = context.WithValue(ctx, contextKeyJWT, claims)
			r = r.WithContext(ctx)

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

func checkRolesOr(acceptedRoles ...string) func(userRoles ...string) bool {
	return func(userRoles ...string) bool {
		if len(acceptedRoles) == 0 {
			slog.Warn("You are using AuthWall with no accepted roles. This means that no users can be accepted.")
		}

		for _, role := range acceptedRoles {
			if slices.Contains(userRoles, role) {
				return true
			}
		}

		return false
	}
}

func checkRolesRegex(acceptedRolesRegex *regexp.Regexp) func(userRoles ...string) bool {
	return func(userRoles ...string) bool {
		for _, role := range userRoles {
			if acceptedRolesRegex.MatchString(role) {
				return true
			}
		}

		return false
	}
}

// AuthWall is a middleware that checks if the user is authorized.
// If not, it returns an error.
// If authorized roles are provided, the user must have at least one of its role in the list.
// For example:
//
//	AuthWall("admin", "chef") // Will block a user with the "waiter" role and allow a user with a role "chef".
//	AuthWall("chef") // Will block a user with the "admin" and "client" role.
//	AuthWall() // Will block all users. To simply Check if the user is authenticated, use the [TokenToContext] middleware.
//
// See the tests for more examples.
func AuthWall(authorizedRoles ...string) func(next http.Handler) http.Handler {
	return authWall(checkRolesOr(authorizedRoles...))
}

// AuthWallRegexp is a middleware that checks if the user is authorized.
// If not, it returns an error.
// If authorized roles are provided, the user must have at least one of its role in the list that matches the regex.
// For example:
//
//	myRegexRule := regexp.MustCompile(`^(super)?admin$`)
//	AuthWallRegex(myRegexRule) // Will block a user with the "waiter" role and allow a user with a role "admin".
//
// See the tests for more examples.
func AuthWallRegexp(acceptedRolesRegex *regexp.Regexp) func(next http.Handler) http.Handler {
	return authWall(checkRolesRegex(acceptedRolesRegex))
}

// AuthWallRegex is a middleware that checks if the user is authorized.
// If not, it returns an error.
// If authorized roles are provided, the user must have at least one of its role in the list that matches the regex.
// For example:
//
//	AuthWallRegex(`^(super)?admin$`) // Will block a user with the "waiter" role and allow a user with a role "admin".
//
// See the tests for more examples.
func AuthWallRegex(acceptedRolesRegex string) func(next http.Handler) http.Handler {
	re := regexp.MustCompile(acceptedRolesRegex)
	return AuthWallRegexp(re)
}

// AuthWall is a middleware that checks if the user is authorized.
// It takes a function that checks if the user is authorized.
func authWall(authorizeFunc func(userRoles ...string) bool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the authorizationHeader from the context (set by TokenToContext)
			claims, err := TokenFromContext(r.Context())
			if err != nil {
				SendJSONError(w, nil, ErrUnauthorized)
				return
			}

			// Get the subject and userRoles from the claims
			userRoles, ok := claims.(jwt.MapClaims)["roles"].([]string)
			if !ok {
				SendJSONError(w, nil, ErrInvalidTokenType)
				return
			}

			// Check if the user is authorized
			if !authorizeFunc(userRoles...) {
				SendJSONError(w, nil, ErrUnauthorized)
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

type tokenResponse struct {
	Token string `json:"token"`
}

// StdLoginHandler is a premade login handler.
// It takes a function that checks if the user is authorized.
// Example:
//
//	security := fuego.NewSecurity()
//	security.ExpiresInterval = 24 * time.Hour
//	fuego.Post(s, "/login", security.StdLoginHandler(verifyUserInfo))
//	...
//	func verifyUserInfo(r *http.Request) (jwt.Claims, error) {
//		// Get the username and password from the request
//		username := r.FormValue("username")
//		password := r.FormValue("password")
//		// ...
//		// Check if the username and password are correct.
//		// Usually, you would check in a database.
//		if username != "myUsername" || password != "myPassword" {
//			return nil, errors.New("invalid username or password")
//		}
//		// ...
//		// Return the claims
//		return &MyCustomToken{
//			It is recommended to embed jwt.RegisteredClaims in your custom struct that will define your JWT.
//			RegisteredClaims: jwt.RegisteredClaims{
//				Issuer:    username,
//				Subject:   username,
//				Audience:  jwt.ClaimStrings{"aud1", "aud2"},
//				ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
//				IssuedAt:  jwt.NewNumericDate(time.Now()),
//				ID:        "1234567890",
//			},
//			Username: "myUsername",
//		}, nil
func (security Security) StdLoginHandler(verifyUserInfo func(r *http.Request) (jwt.Claims, error)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := verifyUserInfo(r)
		if err != nil {
			SendJSONError(w, nil, err)
			return
		}

		// Send the token to the cookies
		token, err := security.GenerateTokenToCookies(claims, w)
		if err != nil {
			SendJSONError(w, nil, err)
			return
		}

		// Send the token to the response
		// no need to check err as SendJSON
		// responds with a 500 on error to the client
		_ = SendJSON(
			w,
			r,
			tokenResponse{
				Token: token,
			},
		)
	}
}

type LoginPayload struct {
	User     string `json:"user" validate:"required"` // Might be an email, a username, or anything else that identifies uniquely the user
	Password string `json:"password" validate:"required"`
}

// LoginHandler is a premade login handler.
// It takes a function that checks if the user is authorized.
// Example:
//
//	security := fuego.NewSecurity()
//	security.ExpiresInterval = 24 * time.Hour
//	fuego.Post(s, "/login", security.LoginHandler(verifyUserInfo))
//	...
//	func verifyUserInfo(r *http.Request) (jwt.Claims, error) {
//		// Get the username and password from the request
//		username := r.FormValue("username")
//		password := r.FormValue("password")
//		// ...
//		// Check if the username and password are correct.
//		// Usually, you would check in a database.
//		if username != "myUsername" || password != "myPassword" {
//			return nil, errors.New("invalid username or password")
//		}
//		// ...
//		// Return the claims
//		return &MyCustomToken{
//			It is recommended to embed jwt.RegisteredClaims in your custom struct that will define your JWT.
//			RegisteredClaims: jwt.RegisteredClaims{
//				Issuer:    username,
//				Subject:   username,
//				Audience:  jwt.ClaimStrings{"aud1", "aud2"},
//				ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
//				IssuedAt:  jwt.NewNumericDate(time.Now()),
//				ID:        "1234567890",
//			},
//			Username: "myUsername",
//		}, nil
func (security Security) LoginHandler(verifyUserInfo func(user, password string) (jwt.Claims, error)) func(ContextWithBody[LoginPayload]) (tokenResponse, error) {
	return func(c ContextWithBody[LoginPayload]) (tokenResponse, error) {
		body, err := c.Body()
		if err != nil {
			return tokenResponse{}, err
		}

		claims, err := verifyUserInfo(body.User, body.Password)
		if err != nil {
			return tokenResponse{}, err
		}

		// Send the token to the cookies
		token, err := security.GenerateTokenToCookies(claims, c.Response())
		if err != nil {
			return tokenResponse{}, err
		}

		// Send the token to the response
		return tokenResponse{
			Token: token,
		}, nil
	}
}

// RefreshHandler is a premade refresh handler.
// It refreshes the token with the same information as the previous one, but with a new issued date.
// It sends the new token to the cookies and to the response.
// Usage:
//
//	fuego.PostStd(s, "/auth/refresh", security.RefreshHandler)
func (security Security) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := TokenFromContext(r.Context())
	if err != nil {
		SendJSONError(w, nil, ErrUnauthorized)
		return
	}

	// Send the token to the cookies
	token, err := security.GenerateTokenToCookies(claims, w)
	if err != nil {
		SendJSONError(w, nil, err)
		return
	}

	// Send the token to the response
	// no need to check err as SendJSON
	// responds with a 500 on error to the client
	_ = SendJSON(
		w,
		nil,
		tokenResponse{
			Token: token,
		},
	)
}

// RemoveTokenFromCookies generates a JWT token with the given claims and writes it to the cookies.
// Usage:
//
//	fuego.PostStd(s, "/auth/logout", security.CookieLogoutHandler)
//
// Dependency to [Security] is for symmetry with [RefreshHandler].
func (security Security) CookieLogoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:    JWTCookieName,
		Expires: security.Now().Add(-security.ExpiresInterval),
	})
}
