module github.com/go-fuego/fuego/examples/full-app-gourmet

go 1.23.3

require (
	github.com/a-h/templ v0.3.819
	github.com/go-chi/chi/v5 v5.2.0
	github.com/go-fuego/fuego v0.17.0
	github.com/go-fuego/fuego/extra/markdown v0.0.0-20250107151251-f857cfde3dd9
	github.com/go-fuego/fuego/middleware/basicauth v0.15.1
	github.com/go-fuego/fuego/middleware/cache v0.0.0-20250107151251-f857cfde3dd9
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/golang-migrate/migrate/v4 v4.18.1
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/lmittmann/tint v1.0.6
	github.com/rs/cors v1.11.1
	github.com/stretchr/testify v1.10.0
	golang.org/x/text v0.21.0
	modernc.org/sqlite v1.34.4
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/getkin/kin-openapi v0.128.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.23.0 // indirect
	github.com/gomarkdown/markdown v0.0.0-20241205020045-f7e15b2f3e62 // indirect
	github.com/gorilla/schema v1.4.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/invopop/yaml v0.3.1 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/crypto v0.32.0 // indirect
	golang.org/x/exp v0.0.0-20250106191152-7588d65b2ba8 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	modernc.org/gc/v3 v3.0.0-20250105121824-520be1a3aee6 // indirect
	modernc.org/libc v1.61.7 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.8.1 // indirect
	modernc.org/strutil v1.2.1 // indirect
	modernc.org/token v1.1.0 // indirect
)

replace github.com/go-fuego/fuego => ../..

replace github.com/go-fuego/fuego/extra/markdown => ../../extra/markdown

replace github.com/go-fuego/fuego/middleware/cache => ../../middleware/cache
