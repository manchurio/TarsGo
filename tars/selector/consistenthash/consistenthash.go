package consistenthash

import (
	"errors"
	"fmt"
	"hash/crc32"
	"sort"
	"sync"

	"github.com/TarsCloud/TarsGo/tars/selector"
	"github.com/TarsCloud/TarsGo/tars/util/endpoint"
)

// ChMap consistent hash map
type ChMap struct {
	sync.RWMutex
	replicates int
	sortedKeys []uint32
	hashRing   map[uint32]endpoint.Endpoint
	mapValues  map[string]struct{}
}

// NewChMap  create a ChMap which has replicates of virtual nodes.
func NewChMap(replicates int) *ChMap {
	return &ChMap{
		replicates: replicates,
		hashRing:   make(map[uint32]endpoint.Endpoint),
		mapValues:  make(map[string]struct{}),
	}
}

func (c *ChMap) Select(msg selector.Message) (point endpoint.Endpoint, err error) {
	var ok bool
	point, ok = c.FindUint32(msg.HashCode())
	if !ok {
		return point, fmt.Errorf("consistenthash: select not found endpoint.Endpoint")
	}
	return point, nil
}

// Find finds a nodes to put the string key
func (c *ChMap) Find(key string) (endpoint.Endpoint, bool) {
	c.RLock()
	defer c.RUnlock()
	var point endpoint.Endpoint
	if len(c.sortedKeys) == 0 {
		return point, false
	}
	hashKey := crc32.ChecksumIEEE([]byte(key))
	index := sort.Search(len(c.sortedKeys), func(x int) bool {
		return c.sortedKeys[x] >= hashKey
	})
	if index >= len(c.sortedKeys) {
		index = 0
	}

	return c.hashRing[c.sortedKeys[index]], true
}

// FindUint32  finds a nodes to put the uint32 key
func (c *ChMap) FindUint32(key uint32) (endpoint.Endpoint, bool) {
	c.RLock()
	defer c.RUnlock()
	var point endpoint.Endpoint
	if len(c.sortedKeys) == 0 {
		return point, false
	}
	index := sort.Search(len(c.sortedKeys), func(x int) bool {
		return c.sortedKeys[x] >= key
	})
	if index >= len(c.sortedKeys) {
		index = 0
	}

	return c.hashRing[c.sortedKeys[index]], true
}

// Add : add the node to the hash ring
func (c *ChMap) Add(node endpoint.Endpoint) error {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.mapValues[node.HashKey()]; ok {
		return errors.New("consistenthash: node already exists")
	}
	for i := 0; i < c.replicates; i++ {
		virtualHost := fmt.Sprintf("%d#%s", i, node.HashKey())
		virtualKey := crc32.ChecksumIEEE([]byte(virtualHost))
		c.hashRing[virtualKey] = node
		c.sortedKeys = append(c.sortedKeys, virtualKey)
	}
	sort.Slice(c.sortedKeys, func(x int, y int) bool {
		return c.sortedKeys[x] < c.sortedKeys[y]
	})
	c.mapValues[node.HashKey()] = struct{}{}
	return nil
}

// Remove the node and all the virtual nodes from the key
func (c *ChMap) Remove(node endpoint.Endpoint) error {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.mapValues[node.HashKey()]; !ok {
		return errors.New("consistenthash: host already removed")
	}
	delete(c.mapValues, node.HashKey())
	for i := 0; i < c.replicates; i++ {
		virtualHost := fmt.Sprintf("%d#%s", i, node.HashKey())
		virtualKey := crc32.ChecksumIEEE([]byte(virtualHost))
		delete(c.hashRing, virtualKey)
	}
	c.reBuildHashRing()
	return nil
}

func (c *ChMap) reBuildHashRing() {
	c.sortedKeys = nil
	for vk := range c.hashRing {
		c.sortedKeys = append(c.sortedKeys, vk)
	}
	sort.Slice(c.sortedKeys, func(x, y int) bool {
		return c.sortedKeys[x] < c.sortedKeys[y]
	})
}
