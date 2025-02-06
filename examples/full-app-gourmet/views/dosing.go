package views

import (
	"context"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/examples/full-app-gourmet/store"
)

type DosingRepository interface {
	CreateDosing(ctx context.Context, arg store.CreateDosingParams) (store.Dosing, error)
	GetDosings(ctx context.Context) ([]store.Dosing, error)
}

var _ DosingRepository = (*store.Queries)(nil)

type dosingResource struct {
	Queries DosingRepository
}

func (rs dosingResource) MountRoutes(s *fuego.Server) {
	dosingGroup := fuego.Group(s, "/dosings")
	fuego.Post(dosingGroup, "/new", rs.newDosing)
}

func (rs dosingResource) newDosing(c fuego.ContextWithBody[store.CreateDosingParams]) (store.Dosing, error) {
	body, err := c.Body()
	if err != nil {
		return store.Dosing{}, err
	}

	dosing, err := rs.Queries.CreateDosing(c.Context(), body)
	if err != nil {
		return store.Dosing{}, err
	}

	return dosing, nil
}
