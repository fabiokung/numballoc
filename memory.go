package numballoc

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/fabiokung/shm"
)

type Memory interface {
	Blocks() []uint32
	Size() uint32 // in bytes
}

type SharedMemory interface {
	Memory

	// Close frees up all references (so they can be GC'ed) and unmaps the
	// shared memory region
	Close() error
}

type sharedMemory struct {
	blocks []uint32
	raw    []byte
}

type sliceType struct {
	data unsafe.Pointer
	len  int
	cap  int
}

// LoadShared maps a shared memory region. Size must be consistent with all
// others mapping the same region (i.e.: the same name).
//
// Loading a shared memory region using the wrong size can lead to segmentation
// faults (SIGSEGV), or truncated data.
func LoadShared(name string, size uint32) (SharedMemory, error) {
	file, err := shm.Open(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	var isNew bool
	if err == nil {
		isNew = true
		if err := syscall.Ftruncate(
			int(file.Fd()), int64(size),
		); err != nil {
			return nil, err
		}
	} else {
		if file, err = shm.Open(name, os.O_RDWR, 0600); err != nil {
			return nil, err
		}
	}
	defer file.Close()

	data, err := syscall.Mmap(int(file.Fd()), 0, int(size),
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED,
	)
	if isNew {
		// clear all bits
		for i := range data {
			data[i] = 0
		}
	}
	// size is in bytes, blocks have 4 bytes (uint32)
	blocksLen := size >> 2 // size / 4
	if blocksLen == 0 {
		blocksLen = 1 // at least 1 block
	}
	blocks := *(*sliceType)(unsafe.Pointer(&data))
	blocks.len = int(blocksLen)
	blocks.cap = int(blocksLen)
	return &sharedMemory{
		blocks: *(*[]uint32)(unsafe.Pointer(&blocks)),
		raw:    data,
	}, nil
}

// DestroyShared cleans up a shared memory region
func DestroyShared(name string) error {
	return shm.Unlink(name)
}

func (b *sharedMemory) Blocks() []uint32 {
	return b.blocks
}

func (b *sharedMemory) Size() uint32 {
	return uint32(len(b.raw))
}

func (b *sharedMemory) Close() error {
	if err := syscall.Munmap(b.raw); err != nil {
		return err
	}
	b.blocks = nil
	b.raw = nil
	return nil
}
