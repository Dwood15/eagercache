# EagerCache

A homegrown, in-memory, expiring key-value caching mechanism well on its way to being production-readiness.
 
EagerCache does not vendor or use any third-pary packages.

The goal of EagerCache is to provide an easy interface without sacrificing complexity. 

## Intended Usage

```Go
package main

import "github.com/dwood15/eagercache"

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

    //Before throwing the cache away, ALWAYS call Implode() or memory may not get released.
    bars.Implode()
}
```

