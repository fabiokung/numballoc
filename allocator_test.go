package numballoc

import (
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/docker/docker/pkg/reexec"
	"github.com/pborman/uuid"
)

func TestMain(m *testing.M) {
	reexec.Register("allocate", allocate)
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}

func TestCanAllocateAllNumbers(t *testing.T) {
	var (
		size uint32 = 256 // bytes: 256 * 8 = 2048 numbers
		name        = "bitmap-test-" + uuid.New()
	)

	numbers := make([]uint64, 0, size*8)
	// shuffle
	for i := uint64(0); i < uint64(size)*8; i++ {
		numbers = append(numbers, i)
	}
	for i := range numbers {
		j := rand.Intn(i + 1)
		numbers[i], numbers[j] = numbers[j], numbers[i]
	}

	mem, err := LoadShared(name, size)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := mem.Close(); err != nil {
			t.Error(err)
		}
		if err := DestroyShared(name); err != nil {
			t.Error(err)
		}
	}()

	allocator := ConcurrentBitmap(mem)
	// allocate size * 8 numbers
	for i := uint64(0); i < uint64(size)*8; i++ {
		if _, err := allocator.Allocate(); err != nil {
			t.Fatal(err)
		}
	}
	// all allocated?
	if _, err := allocator.Allocate(); err != ErrNoFreeNumber {
		t.Fatalf("expected %v, got %v", err)
	}
	// free all, random order
	for _, n := range numbers {
		if err := allocator.Free(n); err != nil {
			t.Fatal(err)
		}
	}
}

func TestParallelAllocation(t *testing.T) {
	var (
		size uint32 = 8192 // bytes: 8192 * 8 = 65536 numbers
		name        = "bitmap-test-" + uuid.New()
	)
	mem, err := LoadShared(name, size)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := mem.Close(); err != nil {
			t.Error(err)
		}
		if err := DestroyShared(name); err != nil {
			t.Error(err)
		}
	}()

	allocator := ConcurrentBitmap(mem)

	var wg sync.WaitGroup
	// 4 x 16384 allocations
	wg.Add(4)
	for i := 0; i < 4; i++ {
		runAllocate(t, &wg, name, strconv.Itoa(int(size)), "16384")
	}
	wg.Wait()
	// all allocated?
	if _, err := allocator.Allocate(); err != ErrNoFreeNumber {
		t.Fatalf("expected %v, got %v", ErrNoFreeNumber, err)
	}
}

func runAllocate(t *testing.T, wg *sync.WaitGroup, memoryRegion, size, qty string) {
	cmd := reexec.Command("allocate", memoryRegion, size, qty)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		defer wg.Done()
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}()
}

func allocate() {
	memoryRegion := os.Args[1]
	size, err := strconv.Atoi(os.Args[2])
	if err != nil {
		panic(err)
	}
	qty, err := strconv.Atoi(os.Args[3])
	if err != nil {
		panic(err)
	}

	mem, err := LoadShared(memoryRegion, uint32(size))
	if err != nil {
		panic(err)
	}
	defer mem.Close()

	allocator := ConcurrentBitmap(mem)
	for i := 0; i < qty; i++ {
		if _, err := allocator.Allocate(); err != nil {
			panic(err)
		}
	}
}