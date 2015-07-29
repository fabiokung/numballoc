## Number allocators

Thread (and sometimes multi-process) safe number allocators.

### Concurrent Bitmap

Lock-free concurrent allocator that stores state (free/used numbers) as a
bitmap. Each number is a position in the bitmap: 0 means free, 1 means
allocated. In conjunction with `SharedMemory`, it can be used as a safe way to
allocate numbers across multiple processes.

Allocations are O(N) in the worst case, but the algorithm uses hints as an
attempt to avoid full scans in most cases and provide a better amortized cost
(probabilistic complexity analysis pending!), as long as allocations and
deallocations are reasonably balanced.

```go
package main

import (
	"github.com/fabiokung/numballoc"
)
func main() {
	// shared memory can be safely used by multiple processes
	mem, err := numballoc.LoadShared(name, size)
	if err != nil {
		panic(err)
	}
	defer mem.Close()

	allocator := ConcurrentBitmap(mem)
	number, err := allocator.Allocate()
	if err != nil {
		panic(err)
	}
	// number is guaranteed to be unique across all processes sharing the
	// same memory region
}
```

### Concurrent Queue (LinkedList)

TBD: based on [fabiokung/cqueue][cqueue]

[cqueue]: https://github.com/fabiokung/cqueue
