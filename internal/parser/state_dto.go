package parser

import (
	"butler/internal/model"
)

type StateDTO struct {
	Lastfired map[string]string
}

func (sd StateDTO) toState() (model.State, error) {
	return model.State{}, nil
}
