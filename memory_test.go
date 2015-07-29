package numballoc

import (
	"testing"

	"github.com/pborman/uuid"
)

func TestNewMemoryRegionIsFilledWithZeroes(t *testing.T) {
	var (
		size uint32 = 256
		name        = "sharedmem-test-" + uuid.New()
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

	blocks := mem.Blocks()
	for i := range blocks {
		if blocks[i] != 0 {
			t.Fatalf(
				"expected index %d to be filled with 0s, Got: %#x",
				i, blocks[i],
			)
		}
	}
}

func TestCanBeShared(t *testing.T) {
	var (
		size uint32 = 256
		name        = "sharedmem-test-" + uuid.New()
		// pos: value
		expectedBlocks = map[uint]uint32{
			10: 0xF0F0F0F0,
			15: 0xAFFEC7ED,
			24: 0x0DE1E7ED,
			63: 0xCAFECAFE,
		}
	)

	m1, err := LoadShared(name, size)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m1.Close(); err != nil {
			t.Error(err)
		}
	}()

	m2, err := LoadShared(name, size)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m2.Close(); err != nil {
			t.Error(err)
		}
	}()

	defer func() {
		if err := DestroyShared(name); err != nil {
			t.Error(err)
		}
	}()

	for i, v := range expectedBlocks {
		m1.Blocks()[i] = v
	}
	for i, want := range expectedBlocks {
		got := m2.Blocks()[i]
		if got != want {
			t.Fatalf("Index %d: want %d, got %d", i, want, got)
		}
	}
}
