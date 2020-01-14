// Package eagercache provides interfaces for threadsafe, mildly-generic, in-memory caches
//These caches are intended to maximize performance as much as possible by helping to
//minimize expensive operations such as locking OS calls (File Reads, Network calls, et cetera)
// License: MIT
package eagercache

import (
	"sync"
	"time"
)

type (
	// Cache provides the public iinterface for threadsafe object storage and
	//accesses. In order to avoid problems with dereferences and copies of the cache
	//structs, it is an interface, rather than concrete implementation.
	Cache interface {
		// Retrieve a value from the cache. On a cache miss, calls the updater with the key
		Retrieve(string) interface{}
		//Implode inactivates the cache from the eager cleaning and eager updating processes.
		//Once Implode is called, CreateCache MUST be called again, and the imploded cache
		//MAY NOT be re-used.
		//
		//This is a convenience func and may or may not be kept in future releases, depending
		//upon user feedback.
		Implode()
	}

	// EntryUpdater the function passed to CreateCache, which is called on every cache
	//miss. The return value from EntryUpdater is expected to always be a pointer, but
	//is never validated, at runtime. Returning non-pointers may produce unexpected
	//results.
	//
	// Debating upon using unsafe.Pointer instead of interface, to can enforce
	//type-correctness at compiletime without reflection.
	EntryUpdater func(string) interface{}
)

// StartCleaner launches a background scrubbing goroutine. The cleaner does a couple things:
//  1. Loops over all entries in all caches created via CreateCache.
//  2. Checks if an entry is expired, otherwise it skips that entryj.
//  3. If the entry has been accessed at least once, it calls that cache's updater func, passing
// to the updater func, the key of the stale entry.
//  4. If the entry has NOT been accessed at least once, it calls delete on the underlying map,
// pruning that entry.
// cleanRate specifies the time between attempts to shrink the underlying maps.
func StartCleaner(cleanRate time.Duration) {
	p.cleanRate = cleanRate
	scrubberLauncher.Do(func() {
		go scrubber()
	})
}

// CreateCache allocates a cache and adds a reference to it to the pool of caches for regular cleaning.
//The new Cache is registerd with a background cachePooler that regularly cleans out expired entries.
//If an Expired entry was accessed at least once since the last cleaning time, the cleaner will update
//the entry. If the expired entry was not accessed at least once, it will be removed and looked up next read.
//
//The updater func is required and expected to be threadsafe.
func CreateCache(expireRate time.Duration, updater EntryUpdater) Cache {
	if updater == nil {
		panic("the updater-func be a non-nil reference to a EntryUpdater")
	}

	c := &cache{
		expireRate: expireRate,
		data:       map[string]expirable{},
		updater:    updater,
	}

	//register the cache so expired entriesthe cleaner
	p.addCache(c)

	return Cache(c)
}

// Retrieve a value from the cache. On a cache miss, calls the updater func with the provided key
func (c *cache) Retrieve(key string) (val interface{}) {
	//pluck from the cache
	c.mu.RLock()
	cached, ok := c.data[key]

	if val = cached.value; ok && !cached.neverAccessed {
		c.mu.RUnlock()
		return
	}

	c.mu.RUnlock()
	return c.entryUpdate(key, cached, !ok)
}

// Implode inactivates the cache from the eager cleaning and eager updating processes.
//Once Implode is called, SUBSEQUENT USES of the cache WILL PANIC
func (c *cache) Implode() {
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

//called from scrubber in pool.go
func (c *cache) processExpired() {
	//Faster to acquire the write lock throughout the delete process than
	//to acquire locks individually for each delete
	c.mu.Lock()
	var wg sync.WaitGroup
	n := time.Now()

	for key, entry := range c.data {
		if !entry.expiresAt.Before(n) {
			continue
		}

		if entry.neverAccessed {
			delete(c.data, key)
			continue
		}

		wg.Add(1)
		//TODO: Chunk these ops so only a few are launched in goroutines at a time?
		go func(k string, n time.Time) {
			c.data[k] = expirable{
				value:         c.updater(k),
				neverAccessed: true,
				expiresAt:     n.Add(c.expireRate),
			}
			wg.Done()
		}(key, n)
	}
	wg.Wait()
	c.mu.Unlock()
}

func (c *cache) entryUpdate(key string, entry expirable, callUpdater bool) expirable {
	c.mu.Lock()

	if entry.neverAccessed {
		entry.neverAccessed = false
	}

	if callUpdater {
		entry.expiresAt = time.Now().Add(c.expireRate)
		entry.value = c.updater(key)
	}

	c.data[key] = entry
	c.mu.Unlock()

	return entry
}
