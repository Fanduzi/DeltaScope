// Package output provides shared output formatting utilities.
package output

import (
	"strings"
	"sync"
)

var builderPool = sync.Pool{
	New: func() any {
		return new(strings.Builder)
	},
}

// GetBuilder returns a strings.Builder from the pool.
func GetBuilder() *strings.Builder {
	return builderPool.Get().(*strings.Builder)
}

// PutBuilder resets and returns a strings.Builder to the pool.
func PutBuilder(b *strings.Builder) {
	b.Reset()
	builderPool.Put(b)
}
