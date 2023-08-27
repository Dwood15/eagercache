// Package eagercache provides interfaces for threadsafe, mildly-generic, in-memory caches
// These caches are intended to maximize performance as much as possible by helping to
// minimize expensive operations such as locking OS calls (File Reads, Network calls, et cetera)
// License: MIT
package eagercache

import (
	"sync"
	"time"
)

type (
	// EntryUpdater the function passed to CreateCache, which is called on every cache
	// miss. The return value from EntryUpdater is expected to always be a pointer, but
	// is never validated, at runtime. Returning non-pointers may produce unexpected
	// results.
	EntryUpdater func(string) interface{}
)

// StartCleaner launches a background scrubbing goroutine. The cleaner does a couple things:
//  1. Loops over all entries in all caches created via CreateCache.
//  2. Checks if an entry is expired, otherwise it skips that entryj.
//  3. If the entry has been accessed at least once, it calls that cache's updater func, passing
//
// to the updater func, the key of the stale entry.
//  4. If the entry has NOT been accessed at least once, it calls delete on the underlying map,
//
// pruning that entry.
// cleanRate specifies the time between attempts to shrink the underlying maps.
func StartCleaner(cleanRate time.Duration) {
	p.cleanRate = cleanRate
	scrubberLauncher.Do(func() {
		go scrubber()
	})
}

// CreateCache allocates a cache and adds a reference to it to the pool of caches for regular cleaning.
// The new Cache is registerd with a background cachePooler that regularly cleans out expired entries.
// If an Expired entry was accessed at least once since the last cleaning time, the cleaner will update
// the entry. If the expired entry was not accessed at least once, it will be removed and looked up next read.
//
// The updater func is required and expected to be threadsafe.
func CreateCache(expireRate time.Duration, updater EntryUpdater) *Cache {
	if updater == nil {
		panic("the updater-func be a non-nil reference to a EntryUpdater")
	}

	c := &Cache{
		expireRate: expireRate,
		data:       map[string]expirable{},
		updater:    updater,
		mu:         new(sync.RWMutex),
	}

	// register the cache so expired entriesthe cleaner
	p.addCache(c)

	return c
}

// Retrieve a value from the cache. On a cache miss, calls the updater func with the provided key
func (c *Cache) Retrieve(key string) interface{} {
	c.mu.RLock()
	cached, isCacheHit := c.data[key]
	c.mu.RUnlock()

	if !isCacheHit || cached.wasAccessedInInterval == false {
		cached = c.retrieveEntry(key, cached)
	}

	return cached.value
}

// Implode inactivates the cache from the eager cleaning and eager updating processes.
// Once Implode is called, SUBSEQUENT USES of the cache WILL PANIC
func (c *Cache) Implode() {
	if c == nil {
		return
	}

	c.mu.Lock()
	c.data = nil
	c.updater = nil
	c.expireRate = -1
	p.removeCache(c)
	c.mu.Unlock()
}

// called from scrubber in pool.go
func (c *Cache) processExpired() {
	// Faster to acquire the write lock throughout the delete process than
	// to acquire locks individually for each delete
	c.mu.Lock()
	var wg sync.WaitGroup
	n := time.Now()

	for key, entry := range c.data {
		if !entry.expiresAt.Before(n) {
			continue
		}

		if !entry.wasAccessedInInterval {
			delete(c.data, key)
			continue
		}

		wg.Add(1)
		// TODO: Chunk these ops so only a few are launched in goroutines at a time?
		go func(k string, n time.Time) {
			c.data[k] = expirable{
				value:     c.updater(k),
				expiresAt: n.Add(c.expireRate),
			}
			wg.Done()
		}(key, n)
	}
	wg.Wait()
	c.mu.Unlock()
}

func (c *Cache) retrieveEntry(key string, entry expirable) expirable {
	c.mu.Lock()

	if entry.wasAccessedInInterval == false {
		entry.expiresAt = time.Now().Add(c.expireRate)
		entry.value = c.updater(key)
	}

	entry.wasAccessedInInterval = true
	c.data[key] = entry
	c.mu.Unlock()

	return entry
}
