package hostfuncs

import (
	"testing"
)

// FuzzPackedPtrLen fuzzes pointer packing for overflow
func FuzzPackedPtrLen(f *testing.F) {
	seeds := []struct{ ptr, len uint32 }{
		{0, 0},
		{0xFFFFFFFF, 0xFFFFFFFF},
		{0x80000000, 0x80000000},
		{1, 1},
	}

	for _, seed := range seeds {
		f.Add(seed.ptr, seed.len)
	}

	f.Fuzz(func(t *testing.T, ptr, length uint32) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on ptr=%d len=%d: %v", ptr, length, r)
			}
		}()

		// Pack and unpack should be inverse operations
		packed := packPtrLen(ptr, length)
		gotPtr, gotLen := unpackPtrLen(packed)

		if gotPtr != ptr || gotLen != length {
			t.Errorf("Pack/unpack mismatch: input(%d,%d) output(%d,%d)",
				ptr, length, gotPtr, gotLen)
		}
	})
}
