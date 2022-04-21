package random

import (
	"errors"
	"math/rand"
	"sync"

	"github.com/TarsCloud/TarsGo/tars/selector"
	"github.com/TarsCloud/TarsGo/tars/util/endpoint"
)

type Random struct {
	sync.RWMutex
	endpoints               []endpoint.Endpoint
	mapValues               map[string]struct{}
	staticWeightRouterCache []int
	staticWeightRouterCount int
}

var _ selector.Selector = &Random{}

func New() *Random {
	return &Random{
		mapValues: make(map[string]struct{}),
	}
}

func (w *Random) Select(_ selector.Message) (endpoint.Endpoint, error) {
	w.RLock()
	defer w.RUnlock()
	var point endpoint.Endpoint
	if len(w.endpoints) == 0 {
		return point, errors.New("random: no such endpoint.Endpoint")
	}
	if w.staticWeightRouterCache != nil && w.staticWeightRouterCount != 0 {
		randomIndex := rand.Intn(w.staticWeightRouterCount)
		idx := w.staticWeightRouterCache[randomIndex]
		return w.endpoints[idx], nil
	}
	randomIndex := rand.Intn(len(w.endpoints))
	return w.endpoints[randomIndex], nil
}

func (w *Random) Add(node endpoint.Endpoint) error {
	w.Lock()
	defer w.Unlock()
	if _, ok := w.mapValues[node.HashKey()]; ok {
		return errors.New("random: node already exists")
	}
	w.endpoints = append(w.endpoints, node)
	w.mapValues[node.HashKey()] = struct{}{}
	w.reBuild()
	return nil
}

func (w *Random) Remove(node endpoint.Endpoint) error {
	w.Lock()
	defer w.Unlock()
	if _, ok := w.mapValues[node.HashKey()]; !ok {
		return errors.New("random: host already removed")
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

func (w *Random) reBuild() {
	w.staticWeightRouterCache = selector.BuildStaticWeightList(w.endpoints)
	w.staticWeightRouterCount = len(w.staticWeightRouterCache)
}
