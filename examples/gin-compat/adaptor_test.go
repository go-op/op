package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFuegoGin(t *testing.T) {
	a := server()

	t.Run("simply test gin", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/gin", nil)
		w := httptest.NewRecorder()

		a.Gin.ServeHTTP(w, r)

		require.Equal(t, 200, w.Code)
	})

	t.Run("test fuego plugin", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/fuego", nil)
		w := httptest.NewRecorder()

		a.Gin.ServeHTTP(w, r)

		require.Equal(t, http.StatusOK, w.Code)
		require.JSONEq(t, `{"message":"Hello"}`, w.Body.String())
	})
}
