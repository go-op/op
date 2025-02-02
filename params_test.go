package fuego

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePathParams(t *testing.T) {
	require.Equal(t, []string(nil), parsePathParams("/"))
	require.Equal(t, []string(nil), parsePathParams("/item/"))
	require.Equal(t, []string{"user"}, parsePathParams("POST /item/{user}"))
	require.Equal(t, []string{"user"}, parsePathParams("/item/{user}"))
	require.Equal(t, []string{"user", "bookname..."}, parsePathParams("/item/{user}/{bookname...}"))
	require.Equal(t, []string{"user", "id"}, parsePathParams("/item/{user}/{id}"))
	require.Equal(t, []string{"$"}, parsePathParams("/item/{$}"))
	require.Equal(t, []string{"user"}, parsePathParams("POST alt.com/item/{user}"))
}

func BenchmarkParsePathParams(b *testing.B) {
	b.Run("empty", func(b *testing.B) {
		for range b.N {
			parsePathParams("/")
		}
	})

	b.Run("several path params", func(b *testing.B) {
		for range b.N {
			parsePathParams("/item/{user}/{id}")
		}
	})
}

func FuzzParsePathParams(f *testing.F) {
	f.Add("/item/{user}")
	f.Add("/item/")
	f.Add("/item/{user}/{id}")
	f.Add("POST /item/{user}")
	f.Add("")

	f.Fuzz(func(t *testing.T, data string) {
		parsePathParams(data)
	})
}
