-- sliding_window.lua
-- KEYS[1] = sorted-set key, e.g. ratelimit:sliding_window:{tenant}:{identifier}
-- ARGV[1] = window_ms (window size in milliseconds)
-- ARGV[2] = limit (max requests allowed within window)
-- ARGV[3] = now (milliseconds)
-- ARGV[4] = requested (number of "slots" this call wants to claim, usually 1)
-- ARGV[5] = ttl_seconds (key expiration)
--
-- Returns: { allowed(0/1), remaining, reset_time_ms, limit }
--
-- Implementation: sorted set where score = timestamp(ms), member = unique id
-- (timestamp:random) so concurrent requests at the same ms don't collide.
-- ZREMRANGEBYSCORE evicts anything older than (now - window), all inside one
-- EVAL so the count-then-add is atomic across concurrent nodes.

local key        = KEYS[1]
local window_ms  = tonumber(ARGV[1])
local limit      = tonumber(ARGV[2])
local now        = tonumber(ARGV[3])
local requested  = tonumber(ARGV[4]) or 1
local ttl_seconds = tonumber(ARGV[5])

local window_start = now - window_ms

-- Evict expired entries
redis.call("ZREMRANGEBYSCORE", key, "-inf", window_start)

local current = redis.call("ZCARD", key)

local allowed = 0
local remaining = math.max(0, limit - current)

if current + requested <= limit then
  for i = 1, requested do
    local member = tostring(now) .. ":" .. tostring(math.random(1, 1000000000)) .. ":" .. tostring(i)
    redis.call("ZADD", key, now, member)
  end
  allowed = 1
  remaining = math.max(0, limit - current - requested)
end

redis.call("EXPIRE", key, ttl_seconds)

-- reset_time: when the oldest entry currently in the window will fall out
local oldest = redis.call("ZRANGE", key, 0, 0, "WITHSCORES")
local reset_time = now + window_ms
if oldest and oldest[2] then
  reset_time = tonumber(oldest[2]) + window_ms
end

return { allowed, remaining, reset_time, limit }
