package numballoc

import (
	"errors"
	"log"
	"sync/atomic"
)

var ErrNoFreeNumber = errors.New("could not allocate a free number")

type Allocator interface {
	// max number that can be allocated
	Max() uint64
	Allocate() (uint64, error)
	Free(uint64) error
}

// ConcurrentBitmap is a lock-free allocator that stores allocations (free/used
// numbers) as a bitmap. Each number is a position in the bitmap: 0 means free,
// 1 means allocated.
//
// mem.Size() is in bytes and each bit is a number, thus max will be
// min(mem.Size() * 8, max), or mem.Size() * 8 when 0 is provided.
func ConcurrentBitmap(mem Memory, max uint64) Allocator {
	// mem.Size() is in bytes, each bit is a possible allocation
	m := uint64(mem.Size()) << 3 // mem.Size() * 8
	if max != 0 && max < m {
		m = max
	}
	return &concurrentBitmap{hint: 0, mem: mem, max: m}
}

type concurrentBitmap struct {
	hint uint32 // where to start searching
	mem  Memory
	max  uint64
}

func (a *concurrentBitmap) Max() uint64 {
	return a.max
}

// Allocate will store the block position of the last allocated number/bit, so
// the search can continue from there. Hopefully this amortizes the O(N) cost of
// scanning the bitmap over time, when allocations and deallocations are
// balanced
func (a *concurrentBitmap) Allocate() (uint64, error) {
	var (
		blocks    = a.mem.Blocks()
		lastBlock = uint32(a.Max() >> 5) // max / 32, each block has 32 bits
		hint      = a.hint
	)
	if r := a.Max() % 32; r != 0 {
		// needs some more bits in an extra block
		lastBlock++
	}
	if hint >= lastBlock {
		hint %= lastBlock
	}

blocks:
	for j, i := uint32(0), hint; j <= lastBlock; i++ {
		j++
		if i >= lastBlock {
			i %= lastBlock
		}
		base := uint64(i) << 5 // i * 32

		log.Printf("i: %d, j: %d, base: %d, lastBlock: %d", i, j, base, lastBlock)
		block := atomic.LoadUint32(&blocks[i])
		if block == 0xFFFFFFFF {
			continue // all being used
		}

	retry:
		// try all 32 bits on this block
		for mask, offset := uint32(0x80000000), uint32(0); mask != 0x00000000; mask >>= 1 {
			if i == lastBlock-1 {
				// this is the last block, only try the necessary bits
				if r := uint32(a.Max() % 32); r != 0 && offset >= r {
					break retry
				}
			}
			bitSet := block | mask
			if bitSet == block {
				offset++
				continue retry // bit was already allocated
			}
			if atomic.CompareAndSwapUint32(&blocks[i], block, bitSet) {
				// allocated! start from here next time
				atomic.StoreUint32(&a.hint, i)
				return base + uint64(offset), nil
			} else {
				// block has changed, reload and retry
				block = atomic.LoadUint32(&blocks[i])
				if block == 0xFFFFFFFF {
					continue blocks // all being used
				}
				goto retry
			}
			offset++
		}
	}
	return 0, ErrNoFreeNumber
}

// Free blocks until the number has been successfully released and made
// available for future allocations
//
// TODO: max retries? timeout?
func (a *concurrentBitmap) Free(n uint64) error {
	var (
		blocks   = a.mem.Blocks()
		blockIdx = n >> 5 // n / 32
		offset   = n % 32
		mask     = uint32(0x80000000) >> offset
		block    = atomic.LoadUint32(&blocks[blockIdx])
		bitClear = block &^ mask
	)
	for {
		if bitClear == block {
			return nil // already free
		}
		if atomic.CompareAndSwapUint32(&blocks[blockIdx], block, bitClear) {
			return nil
		}
		// block has changed, reload:
		block = atomic.LoadUint32(&blocks[blockIdx])
		bitClear = block &^ mask
	}
	return nil
}
