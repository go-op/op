package handler

import (
	"github.com/go-fuego/fuego"
)

type AdminResource struct {
	DosingQueries      DosingRepository
	RecipesQueries     RecipeRepository
	IngredientsQueries IngredientRepository
}

func (rs Resource) pageAdmin(c fuego.ContextNoBody) (fuego.Templ, error) {
	return rs.adminRecipes(c)
}
