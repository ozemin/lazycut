package video

import (
	"container/list"
	"sync"
	"time"
)

const DefaultCacheCapacity = 100

type CacheKey struct {
	Position time.Duration
	Width    int
	Height   int
}

type cacheEntry struct {
	key   CacheKey
	frame string
}

type FrameCache struct {
	capacity int
	items    map[CacheKey]*list.Element
	order    *list.List
	mu       sync.RWMutex
	fps      float64
}

func NewFrameCache(capacity int, fps float64) *FrameCache {
	if capacity <= 0 {
		capacity = DefaultCacheCapacity
	}
	return &FrameCache{
		capacity: capacity,
		items:    make(map[CacheKey]*list.Element),
		order:    list.New(),
		fps:      fps,
	}
}

func (c *FrameCache) quantizePosition(position time.Duration) time.Duration {
	if c.fps <= 0 {
		return position
	}
	frameDuration := time.Duration(float64(time.Second) / c.fps)
	frameIndex := int64(position / frameDuration)
	return time.Duration(frameIndex) * frameDuration
}

func (c *FrameCache) Get(position time.Duration, width, height int) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := CacheKey{
		Position: c.quantizePosition(position),
		Width:    width,
		Height:   height,
	}

	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		return elem.Value.(*cacheEntry).frame, true
	}
	return "", false
}

func (c *FrameCache) Put(position time.Duration, width, height int, frame string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := CacheKey{
		Position: c.quantizePosition(position),
		Width:    width,
		Height:   height,
	}

	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		elem.Value.(*cacheEntry).frame = frame
		return
	}

	if c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.items, oldest.Value.(*cacheEntry).key)
		}
	}

	entry := &cacheEntry{key: key, frame: frame}
	elem := c.order.PushFront(entry)
	c.items[key] = elem
}
