package views

import (
	"database/sql"
	"net/http"
	"slices"
	"strings"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/store"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/templa/admin"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/templa/components"
)

func (rs Resource) adminDeleteRecipe(c fuego.ContextNoBody) (any, error) {
	id := c.PathParam("id")

	err := rs.RecipesQueries.DeleteRecipe(c.Context(), id)
	if err != nil {
		return nil, err
	}

	return rs.adminRecipes(c)
}

func (rs Resource) adminRecipes(c fuego.ContextNoBody) (fuego.Templ, error) {
	searchParams := components.SearchParams{
		Name:    c.QueryParam("name"),
		PerPage: c.QueryParamInt("perPage"),
		Page:    c.QueryParamInt("page"),
		URL:     "/admin/recipes",
		Lang:    c.MainLang(),
	}
	recipes, err := rs.RecipesQueries.SearchRecipes(
		c.Context(),
		store.SearchRecipesParams{
			Search:      sql.NullString{String: searchParams.Name, Valid: true},
			Limit:       int64(searchParams.PerPage),
			Offset:      int64(searchParams.Page-1) * int64(searchParams.PerPage),
			MaxCalories: 9999999,
			MaxTime:     9999999,
			Published:   false,
		},
	)
	if err != nil {
		return nil, err
	}

	return admin.RecipeList(recipes, searchParams), nil
}

func (rs Resource) adminOneRecipe(c fuego.ContextWithBody[store.UpdateRecipeParams]) (any, error) {
	id := c.Request().PathValue("id")

	if c.Request().Method == "PUT" {
		updateRecipeBody, err := c.Body()
		if err != nil {
			return nil, err
		}

		updateRecipeBody.ID = id

		_, err = rs.RecipesQueries.UpdateRecipe(c.Context(), updateRecipeBody)
		if err != nil {
			return nil, err
		}

		c.Response().Header().Set("HX-Trigger", "entity-updated")
		return c.Redirect(http.StatusSeeOther, "/admin/recipes")
	}

	recipe, err := rs.RecipesQueries.GetRecipe(c.Context(), id)
	if err != nil {
		return nil, err
	}

	dosings, err := rs.IngredientsQueries.GetIngredientsOfRecipe(c.Context(), id)
	if err != nil {
		return nil, err
	}

	allIngredients, err := rs.IngredientsQueries.GetIngredients(c.Context())
	if err != nil {
		return nil, err
	}

	slices.SortFunc(allIngredients, func(a, b store.Ingredient) int {
		return strings.Compare(a.Name, b.Name)
	})

	return admin.RecipePage(admin.RecipePageProps{
		Recipe:         recipe,
		Dosings:        dosings,
		AllIngredients: allIngredients,
	}), nil
}

func (rs Resource) editRecipe(c fuego.ContextWithBody[store.UpdateRecipeParams]) (any, error) {
	updateRecipeArgs, err := c.Body()
	if err != nil {
		return "", err
	}

	updateRecipeArgs.ID = c.PathParam("id")

	_, err = rs.RecipesQueries.UpdateRecipe(c.Context(), updateRecipeArgs)
	if err != nil {
		return "", err
	}

	return c.Redirect(http.StatusMovedPermanently, "/admin/recipes")
}

func (rs Resource) adminAddRecipes(c fuego.ContextWithBody[store.CreateRecipeParams]) (any, error) {
	body, err := c.Body()
	if err != nil {
		return "", err
	}

	r, err := rs.RecipesQueries.CreateRecipe(c.Context(), body)
	if err != nil {
		return "", err
	}

	return c.Redirect(http.StatusSeeOther, "/admin/recipes/"+r.ID)
}

func (rs Resource) adminCreateRecipePage(c fuego.ContextNoBody) (fuego.Templ, error) {
	return admin.RecipeNew(), nil
}

func (rs Resource) adminAddDosing(c fuego.ContextWithBody[store.CreateDosingParams]) (any, error) {
	body, err := c.Body()
	if err != nil {
		return "", err
	}

	_, err = rs.DosingQueries.CreateDosing(c.Context(), body)
	if err != nil {
		return "", err
	}

	return c.Redirect(http.StatusMovedPermanently, "/admin/recipes/"+body.RecipeID)
}
