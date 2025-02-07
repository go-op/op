package handler

import (
	"context"
	"log/slog"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/store"
)

type FavoriteRepository interface {
	AddFavorite(ctx context.Context, arg store.AddFavoriteParams) (store.UsersRecipesFavorite, error)
	RemoveFavorite(ctx context.Context, arg store.RemoveFavoriteParams) error
	GetFavoritesByUser(ctx context.Context, username string) ([]store.GetFavoritesByUserRow, error)
}

func (rs Resource) addFavorite(c fuego.ContextNoBody) (store.UsersRecipesFavorite, error) {
	payload := store.AddFavoriteParams{
		Username: c.PathParam("username"),
		RecipeID: c.QueryParam("recipeID"),
	}
	slog.Info("adding favorite", "payload", payload)
	fav, err := rs.FavoritesQueries.AddFavorite(c, payload)
	if err != nil {
		slog.Error("error adding favorite", "err", err)
		return store.UsersRecipesFavorite{}, err
	}
	return fav, nil
}

func (rs Resource) removeFavorite(c fuego.ContextNoBody) (any, error) {
	err := rs.FavoritesQueries.RemoveFavorite(c, store.RemoveFavoriteParams{
		Username: c.PathParam("username"),
		RecipeID: c.QueryParam("recipeID"),
	})
	return nil, err
}

func (rs Resource) getFavoritesByUser(c fuego.ContextNoBody) ([]store.GetFavoritesByUserRow, error) {
	return rs.FavoritesQueries.GetFavoritesByUser(c, c.PathParam("username"))
}
