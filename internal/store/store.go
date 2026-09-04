package store

import (
	"sync"

	"butler/internal/model"
)

type Store struct {
	mu    sync.Mutex
	nodes []*model.Node
}

func (store *Store) Add(node *model.Node) {
	store.nodes = append(store.nodes, node)
}
func (store *Store) Remove() {}
func (store *Store) Update() {}
func (store *Store) Find()   {}
