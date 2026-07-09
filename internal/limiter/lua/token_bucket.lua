-- token_bucket.lua
-- KEYS[1] = bucket hash key, e.g. ratelimit:token_bucket:{tenant}:{identifier}
-- ARGV[1] = capacity (max tokens)
-- ARGV[2] = refill_rate (tokens per second)
-- ARGV[3] = now (milliseconds, from calling client for determinism across nodes)
-- ARGV[4] = requested (number of tokens this request wants to consume)
-- ARGV[5] = ttl_seconds (key expiration, cleans up idle buckets)
--
-- Returns: { allowed(0/1), remaining_tokens, retry_after_ms, limit }
--
-- State is stored as a Redis Hash with fields "tokens" and "ts" (last refill time, ms).
-- All reads/writes happen inside this single EVAL, so refill+consume is atomic and
-- there is no read-modify-write race between concurrent callers on different nodes.

local key          = KEYS[1]
local capacity      = tonumber(ARGV[1])
local refill_rate   = tonumber(ARGV[2])
local now           = tonumber(ARGV[3])
local requested      = tonumber(ARGV[4])
local ttl_seconds   = tonumber(ARGV[5])

local data = redis.call("HMGET", key, "tokens", "ts")
local tokens = tonumber(data[1])
local last_ts = tonumber(data[2])

if tokens == nil then
  tokens = capacity
  last_ts = now
end

-- Refill based on elapsed time since last touch
local elapsed_ms = math.max(0, now - last_ts)
local refill = (elapsed_ms / 1000.0) * refill_rate
tokens = math.min(capacity, tokens + refill)

local allowed = 0
local retry_after_ms = 0

if tokens >= requested then
  tokens = tokens - requested
  allowed = 1
else
  -- how long until enough tokens accumulate
  local deficit = requested - tokens
  if refill_rate > 0 then
    retry_after_ms = math.ceil((deficit / refill_rate) * 1000)
  else
    retry_after_ms = -1 -- never refills
  end
end

redis.call("HSET", key, "tokens", tokens, "ts", now)
redis.call("EXPIRE", key, ttl_seconds)

return { allowed, math.floor(tokens), retry_after_ms, capacity }
