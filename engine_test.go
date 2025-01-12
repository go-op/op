package fuego

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithErrorHandler(t *testing.T) {
	t.Run("default engine", func(t *testing.T) {
		e := NewEngine()
		err := NotFoundError{
			Err: errors.New("Not Found :c"),
		}
		errResponse := e.ErrorHandler(err)
		require.ErrorAs(t, errResponse, &HTTPError{})
	})
	t.Run("custom handler", func(t *testing.T) {
		e := NewEngine(
			WithErrorHandler(func(err error) error {
				return fmt.Errorf("%w foobar", err)
			}),
		)
		err := NotFoundError{
			Err: errors.New("Not Found :c"),
		}
		errResponse := e.ErrorHandler(err)
		require.ErrorAs(t, errResponse, &HTTPError{})
		require.ErrorContains(t, errResponse, "Not Found :c foobar")
	})
	t.Run("should be fatal", func(t *testing.T) {
		require.Panics(t, func() {
			NewEngine(
				WithErrorHandler(nil),
			)
		})
	})

	t.Run("disable error handler", func(t *testing.T) {
		e := NewEngine(DisableErrorHandler())
		err := NotFoundError{
			Err: errors.New("Not Found"),
		}
		errResponse := e.ErrorHandler(err)
		require.Equal(t, "Not Found", errResponse.Error())
	})
}
