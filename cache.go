package fwncs

import (
	"log"
	"sync"
	"time"
)

type item struct {
	value   interface{}
	expires int64
}

type Cache struct {
	items map[string]*item
	mu    sync.Mutex
}

func NewCache() *Cache {
	c := &Cache{items: make(map[string]*item)}
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		{
			select {
			case <-t.C:
				c.mu.Lock()
				for k, v := range c.items {
					if v.Expired(time.Now().UnixNano()) {
						log.Printf("%v has expires at %d", c.items, time.Now().UnixNano())
						delete(c.items, k)
					}
				}
				c.mu.Unlock()
			}
		}
	}()
	return c
}

func (i *item) Expired(time int64) bool {
	if i.expires == 0 {
		return true
	}
	return time > i.expires
}

func (c *Cache) Get(key string) interface{} {
	c.mu.Lock()
	var s interface{} = nil
	if v, ok := c.items[key]; ok {
		s = v.value
	}
	c.mu.Unlock()
	return s
}

func (c *Cache) Put(key string, value interface{}, expires int64) {
	c.mu.Lock()
	if _, ok := c.items[key]; !ok {
		c.items[key] = &item{
			value:   value,
			expires: expires,
		}
	}
	c.mu.Unlock()
}
