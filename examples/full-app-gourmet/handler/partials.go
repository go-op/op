package handler

import (
	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/store/types"
)

func (rs Resource) unitPreselected(c fuego.ContextNoBody) (fuego.CtxRenderer, error) {
	id := c.QueryParam("IngredientID")

	ingredient, err := rs.IngredientsQueries.GetIngredient(c.Context(), id)
	if err != nil {
		return nil, err
	}

	return c.Render("preselected-unit.partial.html", fuego.H{
		"Units":        types.UnitValues,
		"SelectedUnit": ingredient.DefaultUnit,
	})
}
