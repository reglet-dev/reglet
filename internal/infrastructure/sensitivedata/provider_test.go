package sensitivedata

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvider_Track(t *testing.T) {
	p := NewProvider()

	p.Track("secret1")
	p.Track("secret2")
	p.Track("") // Should be ignored

	values := p.AllValues()
	assert.Len(t, values, 2)
	assert.Contains(t, values, "secret1")
	assert.Contains(t, values, "secret2")
}

func TestProvider_Concurrency(t *testing.T) {
	p := NewProvider()
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p.Track("secret")
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = p.AllValues()
		}()
	}

	wg.Wait()
	assert.Len(t, p.AllValues(), 100)
}

func TestProvider_Immutability(t *testing.T) {
	p := NewProvider()
	p.Track("secret")

	values := p.AllValues()
	values[0] = "hacked"

	original := p.AllValues()
	assert.Equal(t, "secret", original[0], "Returned slice should be a copy")
}
