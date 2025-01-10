package models

import (
	"context"
	"errors"

	"github.com/go-fuego/fuego"
)

// Example of generic response
type BareSuccessResponse[Res any] struct {
	StatusCode int    `json:"statusCode"`
	Result     Res    `json:"result"`
	Message    string `json:"message"`
}

type Pets struct {
	ID         string     `json:"id" validate:"required" example:"pet-123456"`
	Name       string     `json:"name" validate:"required" example:"Napoleon"`
	Age        int        `json:"age" example:"2" description:"Age of the pet, in years"`
	IsAdopted  bool       `json:"is_adopted" description:"Is the pet adopted"`
	References References `json:"references"`
}

type PetsCreate struct {
	Name       string     `json:"name" validate:"required,min=1,max=100" example:"Napoleon"`
	Age        int        `json:"age" validate:"min=0,max=100" example:"2" description:"Age of the pet, in years"`
	IsAdopted  bool       `json:"is_adopted" description:"Is the pet adopted"`
	References References `json:"references"`
}

type PetsUpdate struct {
	Name       string     `json:"name,omitempty" validate:"min=1,max=100" example:"Napoleon" description:"Name of the pet"`
	Age        int        `json:"age,omitempty" validate:"max=100" example:"2"`
	IsAdopted  *bool      `json:"is_adopted,omitempty" description:"Is the pet adopted"`
	References References `json:"references"`
}

type References struct {
	Type  string `json:"type" example:"pet-123456" description:"type of reference"`
	Value string `json:"value"`
}

var _ fuego.InTransformer = &Pets{}

func (*Pets) InTransform(context.Context) error {
	return errors.New("pets must only be used as output")
}
