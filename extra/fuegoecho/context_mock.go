package fuegoecho

import (
	"net/http"
	"net/url"
)

type ContextTest[B any] struct {
	echoContext[B]
	BodyInjected  B
	ErrorInjected error

	Params url.Values
}

func (c *ContextTest[B]) Body() (B, error) {
	return c.BodyInjected, c.ErrorInjected
}

func (c *ContextTest[B]) Request() *http.Request {
	return c.echoCtx.Request()
}

func (c *ContextTest[B]) Response() http.ResponseWriter {
	return c.echoCtx.Response().Writer
}

// QueryParam implements fuego.Ctx
func (c *ContextTest[B]) QueryParam(key string) string {
	return c.Params.Get(key)
}
