# EagerCache

A homegrown, in-memory, expiring key-value caching mechanism well on its way to being production-readiness.
 
EagerCache does not vendor or use any third-pary packages.

The goal of EagerCache is to provide an easy interface without sacrificing complexity. 

## Currently available for testing purposes only

Pending a comprehensive suite of unit tests, use of this package in production is not recommended. 

## Intended Usage

```Go
package main

import "github.com/Dwood15/eagercache"

type ItemTypeToStore struct {
    baz bool
}

func YourItemUpdater(key string) interface{} {
    var item ItemTypeToStore
    // Do computation/read/load/network request
    //...
    //pointer to ItemTypeToStore is required
    return &item
}

func init() {
    //Initiate the background cleaner. Doesn't matter whether init or main. 
    eagercache.StartCleaner(2 * time.Minute)
}

func main() {
    // Get a cache instance.
    var bars = eagercache.CreateCache(5 * time.Minue, YourItemUpdater)

    // ...
    bar := bars.Retrieve("barKeyToLoad").(*ItemTypeToStore)

    //do stuff with bar

    //Before throwing the cache away, ALWAYS call Implode() or memory won't get released.
    bars.Implode()
}
```

## Feature Ideas
- 'Go Generate' helper which allows registering the cache during a generate step to allow the compiler to optimize more easily.
- Instead of using interface{} as the return value of the updater func, use unsafe.Pointer.

## Known Limitation
- Cache misses/key updates will block on unrelated keys. Mutexes for individual entries may help with this.

