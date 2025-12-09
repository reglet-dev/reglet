//go:build wasip1

package abi

import (
	"sync"
	"testing"
)

func TestPackPtrLen(t *testing.T) {
	ptr := uint32(0x12345678)
	length := uint32(0xABCDEF00)
	packed := PackPtrLen(ptr, length)

	expected := (uint64(ptr) << 32) | uint64(length)
	if packed != expected {
		t.Errorf("PackPtrLen(%x, %x) = %x; want %x", ptr, length, packed, expected)
	}

	p, l := UnpackPtrLen(packed)
	if p != ptr {
		t.Errorf("UnpackPtrLen returned ptr %x; want %x", p, ptr)
	}
	if l != length {
		t.Errorf("UnpackPtrLen returned length %x; want %x", l, length)
	}
}

func TestPackPtrLen_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("PackPtrLen did not panic with null pointer and non-zero length")
		}
	}()
	PackPtrLen(0, 100)
}

func TestUnpackPtrLen_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("UnpackPtrLen did not panic with null pointer and non-zero length")
		}
	}()
	// invalid packed: ptr=0, len=1
	packed := uint64(1)
	UnpackPtrLen(packed)
}

func TestAllocateDeallocate(t *testing.T) {
	size := uint32(1024)
	ptr := allocate(size)
	if ptr == 0 {
		t.Fatalf("allocate returned 0")
	}

	// Check memory manager tracking
	memoryManager.Lock()
	buf, ok := memoryManager.ptrs[ptr]
	memoryManager.Unlock()

	if !ok {
		t.Errorf("allocated pointer not tracked in memoryManager")
	}
	if uint32(len(buf)) != size {
		t.Errorf("allocated buffer size = %d; want %d", len(buf), size)
	}

	// Write to memory
	data := []byte("hello world")
	copyToMemory(ptr, data)

	// Read back
	readData := readFromMemory(ptr, uint32(len(data)))
	if string(readData) != string(data) {
		t.Errorf("readFromMemory = %q; want %q", readData, data)
	}

	deallocate(ptr, size)

	// Check memory manager untracking
	memoryManager.Lock()
	_, ok = memoryManager.ptrs[ptr]
	memoryManager.Unlock()

	if ok {
		t.Errorf("pointer still tracked after deallocate")
	}
}

func TestFreeAllTracked(t *testing.T) {
	// Reset state
	FreeAllTracked()

	// Allocate multiple
	// p1 := allocate(100)
	// p2 := allocate(200)

	memoryManager.Lock()
	count := len(memoryManager.ptrs)
	memoryManager.Unlock()

	if count != 2 {
		t.Fatalf("expected 2 tracked pointers, got %d", count)
	}

	FreeAllTracked()

	memoryManager.Lock()
	count = len(memoryManager.ptrs)
	memoryManager.Unlock()

	if count != 0 {
		t.Errorf("expected 0 tracked pointers after FreeAllTracked, got %d", count)
	}

	// Pointers should still be valid memory (Go GC handles actual freeing),
	// but they are no longer pinned by us.
	// We can't easily test GC behavior here, but we verified map clearing.
}

func TestPtrFromBytes(t *testing.T) {
	data := []byte("test data")
	packed := PtrFromBytes(data)

	_, length := UnpackPtrLen(packed)
	if length != uint32(len(data)) {
		t.Errorf("packed length = %d; want %d", length, len(data))
	}

	// Check content
	readData := BytesFromPtr(packed)
	if string(readData) != string(data) {
		t.Errorf("BytesFromPtr = %q; want %q", readData, data)
	}

	DeallocatePacked(packed)
}

func TestConcurrency(t *testing.T) {
	// Reset
	FreeAllTracked()

	var wg sync.WaitGroup
	count := 100

	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			packed := PtrFromBytes([]byte("concurrent"))
			// Simulate some work
			_ = BytesFromPtr(packed)
			DeallocatePacked(packed)
		}()
	}
	wg.Wait()

	memoryManager.Lock()
	tracked := len(memoryManager.ptrs)
	memoryManager.Unlock()

	if tracked != 0 {
		t.Errorf("race condition? expected 0 tracked pointers, got %d", tracked)
	}
}
