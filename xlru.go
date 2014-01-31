package xlru

import (
	"container/list"
	"errors"
	"sync"
	"time"
)

const NoExpiration = 0 * time.Millisecond

var ErrValueTooLarge = errors.New("xlru: the value is larger than capacity")

type Cache struct {
	mu       sync.Mutex
	size     int64
	capacity int64

	// list & table of cache entries
	list  *list.List
	table map[string]*list.Element
}

type Stats struct {
	Count    int64
	Size     int64
	Capacity int64
	Oldest   time.Time
}

type entry struct {
	key     string
	value   []byte
	size    int64
	created time.Time
	touched time.Time
	expires time.Duration
}

func NewCache(capacity int64) *Cache {
	return &Cache{
		list:     list.New(),
		table:    make(map[string]*list.Element),
		capacity: capacity,
	}
}

func (c *Cache) GetBytes(key string) (b []byte, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	element := c.table[key]
	if element == nil {
		return nil, false
	}

	entry := element.Value.(*entry)
	if entry.expired() {
		return nil, false
	}

	c.touch(element)
	return entry.value, true
}

func (c *Cache) SetBytes(key string, value []byte, expires time.Duration) error {
	if int64(len(value)) > c.capacity {
		return ErrValueTooLarge
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if element := c.table[key]; element != nil {
		c.update(element, value)
	} else {
		c.insert(key, value, expires)
	}

	return nil
}

func (c *Cache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	element := c.table[key]
	if element == nil {
		return false
	}

	c.list.Remove(element)
	delete(c.table, key)
	c.size -= element.Value.(*entry).size
	return true
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.list.Init()
	c.table = make(map[string]*list.Element)
	c.size = 0
}

func (c *Cache) Count() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return int64(c.list.Len())
}

func (c *Cache) Size() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.size
}

func (c *Cache) Capacity() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.capacity
}

func (c *Cache) Keys() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	keys := make([]string, 0, c.list.Len())
	for e := c.list.Front(); e != nil; e = e.Next() {
		keys = append(keys, e.Value.(*entry).key)
	}
	return keys
}

func (c *Cache) Stats() *Stats {
	c.mu.Lock()
	defer c.mu.Unlock()

	var oldest time.Time
	var size int64
	var count int64

	for el := c.list.Back(); el != nil; el = el.Prev() {
		entry := el.Value.(*entry)
		if entry.expired() {
			continue
		}

		count += 1
		size += entry.size
		if oldest.IsZero() {
			oldest = entry.touched
		}
	}

	return &Stats{count, size, c.capacity, oldest}
}

func (c *Cache) update(element *list.Element, value []byte) {
	size := int64(len(value))
	difference := size - element.Value.(*entry).size
	element.Value.(*entry).value = value
	element.Value.(*entry).size = size
	c.size += difference
	c.touch(element)
	c.enforceCapacity()
}

func (c *Cache) insert(key string, value []byte, expires time.Duration) {
	now := time.Now()
	size := int64(len(value))
	entry := &entry{key, value, size, now, now, expires}
	element := c.list.PushFront(entry)
	c.table[key] = element
	c.size += entry.size
	c.enforceCapacity()
}

func (c *Cache) touch(element *list.Element) {
	c.list.MoveToFront(element)
	element.Value.(*entry).touched = time.Now()
}

func (c *Cache) enforceCapacity() {
	// evict expired values
	for el := c.list.Back(); el != nil; el = el.Prev() {
		entry := el.Value.(*entry)

		if entry.expired() {
			c.list.Remove(el)
			delete(c.table, entry.key)

			c.size -= entry.size
		}
	}

	// evict least recently used
	for c.size > c.capacity {
		last := c.list.Back()
		entry := last.Value.(*entry)

		c.list.Remove(last)
		delete(c.table, entry.key)

		c.size -= entry.size
	}
}

func (e *entry) expired() bool {
	if e.expires == NoExpiration {
		return false
	}

	deadline := e.created.Add(e.expires)
	if time.Now().After(deadline) {
		return true
	}

	return false
}
