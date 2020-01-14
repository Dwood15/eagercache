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
	//how we maintain
	cachePooler struct {
		mu        sync.RWMutex
		cleanRate time.Duration

		//pointers for concurrent accesses
		pool []*cache
	}

	// encapsulate cache entries and indicate when it should expire
	expirable struct {
		neverAccessed bool
		expiresAt     time.Time
		value         interface{}
	}

	// protectedCache is the ACTUAL implementation of the cache mechanism
	cache struct {
		mu         sync.RWMutex
		expireRate time.Duration
		data       map[string]expirable
		updater    func(string) interface{}
		poolIndex  int
	}
)

var (
	p = cachePooler{
		cleanRate: 2 * time.Minute,
	}

	scrubberLauncher = new(sync.Once)
)

func scrubber() {
	for {
		//Initiate the cache cleans on arbitrary intervals
		//Slow cleaning in order to avoid burning CPU cycles to the garbage collector
		time.Sleep(p.cleanRate)
		p.mu.RLock()
		for i := 0; i < len(p.pool); i++ {
			if (p.pool)[i] == nil {
				continue
			}
			(p.pool)[i].processExpired()
		}
		p.mu.RUnlock()
	}
}

func (cp *cachePooler) addCache(c *cache) {
	cp.mu.Lock()
	c.poolIndex = len(cp.pool)
	cp.pool = append(cp.pool, c)
	cp.mu.Unlock()
}

func (cp *cachePooler) removeCache(c *cache) {
	if c == nil || c.poolIndex < 0 {
		return
	}

	p.mu.Lock()
	if cp.pool[c.poolIndex] != c {
		panic("cache poolIndex doesn't map to the same pointer as was passed in")
	}

	cp.pool[c.poolIndex] = nil
	c.poolIndex = -1
	p.mu.Unlock()
}
