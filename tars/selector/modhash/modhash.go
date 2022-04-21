package modhash

import (
	"errors"
	"sync"

	"github.com/TarsCloud/TarsGo/tars/selector"
	"github.com/TarsCloud/TarsGo/tars/util/endpoint"
)

type ModHash struct {
	sync.RWMutex
	endpoints               []endpoint.Endpoint
	mapValues               map[string]struct{}
	staticWeightRouterCache []int
	staticWeightRouterCount int
}

var _ selector.Selector = &ModHash{}

func New() *ModHash {
	return &ModHash{
		mapValues: make(map[string]struct{}),
	}
}

func (w *ModHash) Select(msg selector.Message) (endpoint.Endpoint, error) {
	w.RLock()
	defer w.RUnlock()
	var point endpoint.Endpoint
	if len(w.endpoints) == 0 {
		return point, errors.New("modhash: no such endpoint.Endpoint")
	}
	if w.staticWeightRouterCache != nil && w.staticWeightRouterCount != 0 {
		idx := w.staticWeightRouterCache[int(msg.HashCode())%w.staticWeightRouterCount]
		return w.endpoints[idx], nil
	}
	return w.endpoints[int(msg.HashCode())%len(w.endpoints)], nil
}

func (w *ModHash) Add(node endpoint.Endpoint) error {
	w.Lock()
	defer w.Unlock()
	if _, ok := w.mapValues[node.HashKey()]; ok {
		return errors.New("modhash: node already exists")
	}
	w.endpoints = append(w.endpoints, node)
	w.mapValues[node.HashKey()] = struct{}{}
	w.reBuild()
	return nil
}

func (w *ModHash) Remove(node endpoint.Endpoint) error {
	w.Lock()
	defer w.Unlock()
	if _, ok := w.mapValues[node.HashKey()]; !ok {
		return errors.New("modhash: host already removed")
	}
	delete(w.mapValues, node.HashKey())
	for i, n := range w.endpoints {
		if n.HashKey() == node.HashKey() {
			w.endpoints = append(w.endpoints[:i], w.endpoints[i+1:]...)
			break
		}
	}
	w.reBuild()
	return nil
}

func (w *ModHash) reBuild() {
	w.staticWeightRouterCache = selector.BuildStaticWeightList(w.endpoints)
	w.staticWeightRouterCount = len(w.staticWeightRouterCache)
}
