package fuegoecho

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/internal"
	"github.com/labstack/echo/v4"
)

type echoContext[B any] struct {
	internal.CommonContext[B]
	echoCtx echo.Context
}

var (
	_ fuego.ContextWithBody[any] = &echoContext[any]{}
	_ fuego.ContextFlowable[any] = &echoContext[any]{}
)

func (c echoContext[B]) Body() (B, error) {
	var body B
	err := c.echoCtx.Bind(&body)
	if err != nil {
		return body, err
	}

	return fuego.TransformAndValidate(c, body)
}

func (c echoContext[B]) Context() context.Context {
	return c.echoCtx.Request().Context()
}

func (c echoContext[B]) Cookie(name string) (*http.Cookie, error) {
	return c.echoCtx.Request().Cookie(name)
}

func (c echoContext[B]) Header(key string) string {
	return c.echoCtx.Request().Header.Get(key)
}

func (c echoContext[B]) MustBody() B {
	body, err := c.Body()
	if err != nil {
		panic(err)
	}
	return body
}

func (c echoContext[B]) PathParam(name string) string {
	return c.echoCtx.Param(name)
}

func (c echoContext[B]) MainLang() string {
	return strings.Split(c.MainLocale(), "-")[0]
}

func (c echoContext[B]) MainLocale() string {
	return strings.Split(c.Request().Header.Get("Accept-Language"), ",")[0]
}

func (c echoContext[B]) Redirect(code int, url string) (any, error) {
	c.echoCtx.Redirect(code, url)
	return nil, nil
}

func (c echoContext[B]) Render(templateToExecute string, data any, templateGlobsToOverride ...string) (fuego.CtxRenderer, error) {
	panic("unimplemented")
}

func (c echoContext[B]) Request() *http.Request {
	return c.echoCtx.Request()
}

func (c echoContext[B]) Response() http.ResponseWriter {
	return c.echoCtx.Response()
}

func (c echoContext[B]) SetCookie(cookie http.Cookie) {
	c.echoCtx.SetCookie(&cookie)
}

func (c echoContext[B]) HasCookie(name string) bool {
	_, err := c.Cookie(name)
	return err == nil
}

func (c echoContext[B]) HasHeader(key string) bool {
	_, ok := c.echoCtx.Request().Header[key]
	return ok
}

func (c echoContext[B]) SetHeader(key, value string) {
	c.echoCtx.Response().Header().Add(key, value)
}

func (c echoContext[B]) SetStatus(code int) {
	c.echoCtx.Response().WriteHeader(code)
}

func (c echoContext[B]) Serialize(data any) error {
	status := c.echoCtx.Response().Status
	if status == 0 {
		status = c.DefaultStatusCode
	}
	if status == 0 {
		status = http.StatusOK
	}
	c.echoCtx.JSON(status, data)
	return nil
}

func (c echoContext[B]) SerializeError(err error) {
	statusCode := http.StatusInternalServerError
	var errorWithStatusCode fuego.ErrorWithStatus
	if errors.As(err, &errorWithStatusCode) {
		statusCode = errorWithStatusCode.StatusCode()
	}
	c.echoCtx.JSON(statusCode, err)
}

func (c echoContext[B]) SetDefaultStatusCode() {
	if c.DefaultStatusCode == 0 {
		c.DefaultStatusCode = http.StatusOK
	}
	c.SetStatus(c.DefaultStatusCode)
}
