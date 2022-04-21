package rr

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/TarsCloud/TarsGo/tars/selector"
	"github.com/TarsCloud/TarsGo/tars/util/endpoint"
)

type RoundRobin struct {
	sync.RWMutex
	endpoints                []endpoint.Endpoint
	mapValues                map[string]struct{}
	lastPosition             int32
	lastStaticWeightPosition int32
	staticWeightRouterCache  []int
	staticWeightRouterCount  int
}

var _ selector.Selector = &RoundRobin{}

func New() *RoundRobin {
	return &RoundRobin{
		mapValues: make(map[string]struct{}),
	}
}

func (w *RoundRobin) Select(_ selector.Message) (endpoint.Endpoint, error) {
	w.RLock()
	defer w.RUnlock()
	var point endpoint.Endpoint
	if len(w.endpoints) == 0 {
		return point, errors.New("rr: no such endpoint.Endpoint")
	}
	if w.staticWeightRouterCache != nil && w.staticWeightRouterCount != 0 {
		idx := atomic.AddInt32(&w.lastStaticWeightPosition, 1)
		return w.endpoints[w.staticWeightRouterCache[int(idx)%w.staticWeightRouterCount]], nil
	}
	idx := atomic.AddInt32(&w.lastPosition, 1)
	point = w.endpoints[int(idx)%len(w.endpoints)]
	return point, nil
}

func (w *RoundRobin) Add(node endpoint.Endpoint) error {
	w.Lock()
	defer w.Unlock()
	if _, ok := w.mapValues[node.HashKey()]; ok {
		return errors.New("rr: node already exists")
	}
	w.endpoints = append(w.endpoints, node)
	w.mapValues[node.HashKey()] = struct{}{}
	w.reBuild()
	return nil
}

func (w *RoundRobin) Remove(node endpoint.Endpoint) error {
	w.Lock()
	defer w.Unlock()
	if _, ok := w.mapValues[node.HashKey()]; !ok {
		return errors.New("rr: host already removed")
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

func (w *RoundRobin) reBuild() {
	w.lastStaticWeightPosition = 0
	w.staticWeightRouterCache = selector.BuildStaticWeightList(w.endpoints)
	w.staticWeightRouterCount = len(w.staticWeightRouterCache)
}
