package utils

import (
	"fmt"
	"hash/fnv"
)

// Hash32 generates a 32-bit hash of a string
func Hash32(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// Hash64 generates a 64-bit hash of a string
func Hash64(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// ConsistentHash distributes a key across n buckets using consistent hashing
func ConsistentHash(key string, seed int, buckets int) int {
	h := fnv.New32a()
	h.Write([]byte(fmt.Sprintf("%d:%s", seed, key)))
	return int(h.Sum32()) % buckets
}
