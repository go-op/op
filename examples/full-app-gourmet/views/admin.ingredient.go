package views

import (
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/store"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/templa/admin"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/templa/components"
)

func (rs Resource) adminOneIngredient(c fuego.ContextWithBody[store.UpdateIngredientParams]) (any, error) {
	id := c.PathParam("id")

	if c.Request().Method == "PUT" {
		updateIngredientArgs, err := c.Body()
		if err != nil {
			return nil, err
		}

		updateIngredientArgs.ID = c.PathParam("id")

		_, err = rs.IngredientsQueries.UpdateIngredient(c, updateIngredientArgs)
		if err != nil {
			return nil, err
		}

		c.Response().Header().Set("HX-Trigger", "entity-updated")
		return c.Redirect(http.StatusSeeOther, "/admin/ingredients")
	}

	ingredient, err := rs.IngredientsQueries.GetIngredient(c, id)
	if err != nil {
		return nil, err
	}

	slog.Debug("ingredient", "ingredient", ingredient, "strconv", strconv.FormatBool(ingredient.AvailableAllYear))

	return admin.IngredientPage(ingredient), nil
}

func (rs Resource) adminIngredientCreationPage(c fuego.ContextWithBody[store.CreateIngredientParams]) (any, error) {
	return admin.IngredientNew(), nil
}

func (rs Resource) adminCreateIngredient(c fuego.ContextWithBody[store.CreateIngredientParams]) (any, error) {
	createIngredientArgs, err := c.Body()
	if err != nil {
		return nil, err
	}

	_, err = rs.IngredientsQueries.CreateIngredient(c, createIngredientArgs)
	if err != nil {
		return nil, err
	}

	c.Response().Header().Set("HX-Trigger", "ingredient-created")

	return c.Redirect(301, "/admin/ingredients")
}

func (rs Resource) adminIngredients(c fuego.ContextNoBody) (fuego.Templ, error) {
	searchParams := components.SearchParams{
		Name:    c.QueryParam("name"),
		PerPage: c.QueryParamInt("perPage"),
		Page:    c.QueryParamInt("page"),
		URL:     "/admin/ingredients",
		Lang:    c.MainLang(),
	}

	slog.Debug("params", "params", searchParams)

	ingredients, err := rs.IngredientsQueries.SearchIngredients(c.Context(), store.SearchIngredientsParams{
		Search: sql.NullString{String: searchParams.Name, Valid: true},
		Limit:  int64(searchParams.PerPage),
		Offset: int64(searchParams.Page-1) * int64(searchParams.PerPage),
	})
	if err != nil {
		return nil, err
	}

	return admin.IngredientList(ingredients, searchParams), nil
}
