// compare_memory_usage.go
//
// Creates 100,000 rate-limit entries two ways against a real Redis
// instance and reports the actual `used_memory` delta from INFO memory,
// so the percentage reduction reported is measured, not assumed.
//
// Baseline: one STRING key per identifier (as many naive implementations
// do): SET ratelimit:token_bucket:user_N "<json>"
//
// Optimized: identifiers sharded into NUM_SHARDS hash keys, one HSET
// field per identifier, per the hash_operations.lua grouping strategy.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	goredis "github.com/redis/go-redis/v9"
)

const (
	numEntries = 100000
	numShards  = 256 // keeps each hash well under the listpack-encoding entry limit
)

func main() {
	addr := "localhost:6379"
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		addr = v
	}

	rdb := goredis.NewClient(&goredis.Options{Addr: addr})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}

	rdb.FlushDB(ctx)
	baseline := measure(ctx, rdb, "baseline (string keys)", func(pipe goredis.Pipeliner, i int) {
		key := fmt.Sprintf("ratelimit:token_bucket:user_%d", i)
		val := fmt.Sprintf(`{"tokens":%d,"ts":1731000000000}`, i%200)
		pipe.Set(ctx, key, val, 0)
	})

	rdb.FlushDB(ctx)
	optimized := measure(ctx, rdb, "optimized (grouped hash)", func(pipe goredis.Pipeliner, i int) {
		shard := i % numShards
		key := fmt.Sprintf("ratelimit:token_bucket:shard:%d", shard)
		field := "user_" + strconv.Itoa(i)
		val := fmt.Sprintf("%d|1731000000000", i%200)
		pipe.HSet(ctx, key, field, val)
	})

	reduction := 100 * (1 - float64(optimized)/float64(baseline))
	fmt.Println("---")
	fmt.Printf("Baseline used_memory delta:  %d bytes (%d entries)\n", baseline, numEntries)
	fmt.Printf("Optimized used_memory delta: %d bytes (%d entries, %d shards)\n", optimized, numEntries, numShards)
	fmt.Printf("Measured reduction: %.1f%%\n", reduction)
	if reduction < 40 {
		fmt.Println("NOTE: measured reduction is below the 40% target at this shard count/value size.")
		fmt.Println("      Reduction scales with shard count (fewer, larger hashes = less per-key overhead)")
		fmt.Println("      and shrinks as payload size grows relative to per-key overhead. Tune numShards")
		fmt.Println("      and encoding to hit your target on your actual key/value shapes.")
	}
}

func measure(ctx context.Context, rdb *goredis.Client, label string, add func(pipe goredis.Pipeliner, i int)) int64 {
	before := usedMemory(ctx, rdb)

	pipe := rdb.Pipeline()
	for i := 0; i < numEntries; i++ {
		add(pipe, i)
		if i%1000 == 999 {
			if _, err := pipe.Exec(ctx); err != nil {
				log.Fatalf("pipeline exec failed: %v", err)
			}
			pipe = rdb.Pipeline()
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Fatalf("final pipeline exec failed: %v", err)
	}

	after := usedMemory(ctx, rdb)
	delta := after - before
	fmt.Printf("%-30s before=%d after=%d delta=%d bytes\n", label, before, after, delta)
	return delta
}

func usedMemory(ctx context.Context, rdb *goredis.Client) int64 {
	info, err := rdb.Info(ctx, "memory").Result()
	if err != nil {
		log.Fatalf("INFO memory failed: %v", err)
	}
	return parseUsedMemory(info)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	return lines
}

func parseUsedMemory(info string) int64 {
	for _, line := range splitLines(info) {
		if len(line) > 12 && line[:12] == "used_memory:" {
			valStr := line[12:]
			valStr = trimCR(valStr)
			v, err := strconv.ParseInt(valStr, 10, 64)
			if err == nil {
				return v
			}
		}
	}
	log.Fatalf("used_memory not found in INFO output")
	return 0
}

func trimCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}
