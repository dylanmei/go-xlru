package xlru

import (
	"testing"
	"time"
)

func ToBuffer(s string) []byte {
	return []byte(s)
}

func FromBuffer(b []byte) string {
	return string(b)
}

func Test_initial_state(t *testing.T) {
	cache := NewCache(5)
	stats := cache.Stats()

	if stats.Count != 0 {
		t.Errorf("number of values = %v, want 0", stats.Count)
	}

	if stats.Size != 0 {
		t.Errorf("size of values = %v, want 0", stats.Size)
	}

	if stats.Capacity != 5 {
		t.Errorf("cache capacity = %v, want 5", stats.Capacity)
	}
}

func Test_set_new_value(t *testing.T) {
	cache := NewCache(100)
	key := "key"
	cache.SetBytes(key, ToBuffer("hello"), NoExpiration)

	b, ok := cache.GetBytes(key)
	if !ok || FromBuffer(b) != "hello" {
		t.Errorf("wrong value for \"key\": %s != %v", b, "")
	}
}

func Test_get_value(t *testing.T) {
	cache := NewCache(100)
	cache.SetBytes("key", ToBuffer("hello"), NoExpiration)

	b, ok := cache.GetBytes("key")

	if !ok || FromBuffer(b) != "hello" {
		t.Errorf("wrong value for \"key\": %s != %v", b, "hello")
	}
}

func Test_value_too_large(t *testing.T) {
	cache := NewCache(2)
	err := cache.SetBytes("key", ToBuffer("abc"), NoExpiration)

	if err != ErrValueTooLarge {
		t.Errorf("expected ErrValueTooLarge, but was %v", err)
	}
}

func Test_get_missing_value(t *testing.T) {
	cache := NewCache(100)

	if _, ok := cache.GetBytes("blah"); ok {
		t.Error("empty cache returned a value")
	}
}

func Test_set_new_value_updates_cache_size(t *testing.T) {
	cache := NewCache(100)

	cache.SetBytes("key1", ToBuffer(""), NoExpiration)

	if stats := cache.Stats(); stats.Size != 0 {
		t.Errorf("cache size (%v) should be zero", stats.Size)
	}

	cache.SetBytes("key2", ToBuffer("hello"), NoExpiration)
	if stats := cache.Stats(); stats.Size != 5 {
		t.Errorf("cache size (%v) should be 5", stats.Size)
	}
}

func Test_set_update_value(t *testing.T) {
	cache := NewCache(100)
	cache.SetBytes("key", ToBuffer("abc"), NoExpiration)
	cache.SetBytes("key", ToBuffer("xyz"), NoExpiration)

	b, ok := cache.GetBytes("key")
	if !ok || FromBuffer(b) != "xyz" {
		t.Errorf("value wasn't updated: %s != %v", b, "xyz")
	}
}

func Test_set_update_value_updates_cache_size(t *testing.T) {
	cache := NewCache(100)

	cache.SetBytes("key", ToBuffer(""), NoExpiration)
	if stats := cache.Stats(); stats.Size != 0 {
		t.Errorf("cache size (%v) should be zero", stats.Size)
	}

	cache.SetBytes("key", ToBuffer("hello"), NoExpiration)
	if stats := cache.Stats(); stats.Size != 5 {
		t.Errorf("cache size (%v) should be 5", stats.Size)
	}
}

func Test_delete_value(t *testing.T) {
	cache := NewCache(100)
	if cache.Delete("key") {
		t.Error("value unexpectedly already in cache")
	}

	cache.SetBytes("key", ToBuffer("abc"), NoExpiration)

	if !cache.Delete("key") {
		t.Error("expected value to be in cache")
	}

	if stats := cache.Stats(); stats.Size != 0 {
		t.Errorf("cache size (%v) should be zero", stats.Size)
	}

	if _, ok := cache.GetBytes("key"); ok {
		t.Error("value was returned after deletion")
	}
}

func Test_clear_values(t *testing.T) {
	cache := NewCache(100)
	cache.SetBytes("key", ToBuffer("abc"), NoExpiration)
	cache.Clear()

	if stats := cache.Stats(); stats.Size != 0 {
		t.Errorf("cache size (%v) should be zero", stats.Size)
	}
}

func Test_capacity_limit(t *testing.T) {
	capacity := int64(3)
	cache := NewCache(capacity)

	// Insert up to the cache's capacity.
	cache.SetBytes("key1", ToBuffer("a"), NoExpiration)
	cache.SetBytes("key2", ToBuffer("b"), NoExpiration)
	cache.SetBytes("key3", ToBuffer("c"), NoExpiration)

	if stats := cache.Stats(); stats.Size != capacity {
		t.Errorf("cache size (%v) should be %v", stats.Size, capacity)
	}

	// Insert one more; something should be evicted to make room.
	cache.SetBytes("key4", ToBuffer("d"), NoExpiration)
	if stats := cache.Stats(); stats.Size != capacity {
		t.Errorf("cache size (%v) should be %v", stats.Size, capacity)
	}
}

func Test_least_recently_used_value_is_evicted(t *testing.T) {
	cache := NewCache(3)

	cache.SetBytes("key1", ToBuffer("a"), NoExpiration)
	cache.SetBytes("key2", ToBuffer("b"), NoExpiration)
	cache.SetBytes("key3", ToBuffer("c"), NoExpiration)
	// lru: [key3, key2, key1]

	// Look up the values, affecting lru ordering
	cache.GetBytes("key3")
	cache.GetBytes("key2")
	cache.GetBytes("key1")
	// lru: [key1, key2, key3]

	cache.SetBytes("key0", ToBuffer("z"), NoExpiration)
	// lru: [key0, key1, key2]

	// least recently used value should have been evicted
	if _, ok := cache.GetBytes("key3"); ok {
		t.Error("value key3 was not evicted")
	}

	// The others are still in cache.
	if _, ok := cache.GetBytes("key0"); !ok {
		t.Error("value key0 was not cached")
	}
	if _, ok := cache.GetBytes("key1"); !ok {
		t.Error("value key1 is missing")
	}
	if _, ok := cache.GetBytes("key2"); !ok {
		t.Error("value key2 is missing")
	}
}

func Test_get_value_that_is_not_yet_expired(t *testing.T) {
	cache := NewCache(100)

	cache.SetBytes("key", ToBuffer("hello"), 2*time.Millisecond)
	<-time.After(1 * time.Millisecond)

	if _, ok := cache.GetBytes("key"); !ok {
		t.Errorf("value should not have been evicted")
	}
}

func Test_get_value_that_is_expired(t *testing.T) {
	cache := NewCache(100)

	cache.SetBytes("key", ToBuffer("hello"), 1*time.Millisecond)
	<-time.After(2 * time.Millisecond)

	if _, ok := cache.GetBytes("key"); ok {
		t.Errorf("expired value should have been evicted")
	}
}

func Test_update_does_not_extend_expiration(t *testing.T) {
	cache := NewCache(100)

	cache.SetBytes("key", ToBuffer("hello1"), 2*time.Millisecond)
	<-time.After(1 * time.Millisecond)

	cache.SetBytes("key", ToBuffer("hello2"), 2*time.Millisecond)
	<-time.After(1 * time.Millisecond)

	if _, ok := cache.GetBytes("key"); ok {
		t.Errorf("expired value should have been evicted")
	}
}

func Test_update_does_not_remove_expiration(t *testing.T) {
	cache := NewCache(100)

	cache.SetBytes("key", ToBuffer("hello1"), 2*time.Millisecond)
	cache.SetBytes("key", ToBuffer("hello2"), NoExpiration)
	<-time.After(2 * time.Millisecond)

	if _, ok := cache.GetBytes("key"); ok {
		t.Errorf("expired value should have been evicted")
	}
}

func Test_expired_value_evicted_before_least_recently_used_value(t *testing.T) {
	cache := NewCache(2)

	cache.SetBytes("key1", ToBuffer("a"), NoExpiration)
	cache.SetBytes("key2", ToBuffer("b"), 2*time.Millisecond)
	// lru: [key2, key1]

	<-time.After(2 * time.Millisecond)
	cache.SetBytes("key3", ToBuffer("c"), NoExpiration)

	// expired value should have been evicted
	if _, ok := cache.GetBytes("key2"); ok {
		t.Error("value key2 was not evicted")
	}

	// The others are still in cache.
	if _, ok := cache.GetBytes("key1"); !ok {
		t.Error("value key1 was not cached")
	}
	if _, ok := cache.GetBytes("key3"); !ok {
		t.Error("value key3 is missing")
	}
}

func Test_expired_values_are_not_counted_against_stats(t *testing.T) {
	cache := NewCache(2)
	cache.SetBytes("key1", ToBuffer("a"), 1*time.Millisecond)
	cache.SetBytes("key2", ToBuffer("b"), 1*time.Millisecond)
	<-time.After(1 * time.Millisecond)

	stats := cache.Stats()
	if stats.Count != 0 {
		t.Errorf("number of values should be 0, but was %v", stats.Count)
	}

	if stats.Size != 0 {
		t.Errorf("size of values should be 0, but was %v", stats.Size)
	}
}
