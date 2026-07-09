-- hash_operations.lua
-- Implements the "grouped hash" memory optimization (Claim 3).
--
-- Baseline approach: one Redis STRING key per rate-limit identifier, e.g.
--   SET ratelimit:token_bucket:user_12345 "<json>"
-- Every key pays fixed per-key overhead: dict entry (~56-96 bytes on 64-bit
-- Redis depending on build), key name bytes, plus the expire-table entry if
-- TTL is set. At high cardinality (100k+ users/IPs) that overhead dominates.
--
-- Grouped-hash approach: identifiers are sharded into N buckets by a stable
-- hash of the identifier, and all identifiers in the same bucket live as
-- fields inside ONE Redis Hash key:
--   HSET ratelimit:token_bucket:shard:{bucket} {identifier} "<packed counter>"
-- This trades per-key overhead for per-field overhead inside a single
-- ziplist/listpack-encoded hash (Redis keeps small hashes in a compact
-- listpack encoding below hash-max-listpack-entries/-value, default 128
-- entries / 64 bytes), which is much cheaper than a full key per identifier.
--
-- Because real per-field TTL (HEXPIRE) requires Redis 7.4+, and this
-- environment runs Redis 7.0.15, this script stores an explicit expiry
-- timestamp packed into the field value and treats a field as logically
-- expired (and clears it) if read after that timestamp, giving equivalent
-- behavior without requiring HEXPIRE. If you deploy on 7.4+, HEXPIRE can be
-- substituted directly and this script is unnecessary.
--
-- KEYS[1] = hash key, e.g. ratelimit:token_bucket:shard:{bucket}
-- ARGV[1] = field (identifier)
-- ARGV[2] = op ("incr" or "get")
-- ARGV[3] = now (ms)
-- ARGV[4] = ttl_ms (for "incr")
-- ARGV[5] = increment amount (for "incr")
--
-- Packed value format: "<counter>|<expires_at_ms>"

local hkey = KEYS[1]
local field = ARGV[1]
local op = ARGV[2]
local now = tonumber(ARGV[3])

local function parse(v)
  if not v then return nil, nil end
  local counter, expires_at = v:match("^(%-?%d+)|(%-?%d+)$")
  if not counter then return nil, nil end
  return tonumber(counter), tonumber(expires_at)
end

if op == "get" then
  local raw = redis.call("HGET", hkey, field)
  local counter, expires_at = parse(raw)
  if counter == nil or (expires_at and now > expires_at) then
    if raw then redis.call("HDEL", hkey, field) end
    return { 0, 0 }
  end
  return { counter, expires_at - now }
end

if op == "incr" then
  local ttl_ms = tonumber(ARGV[4])
  local amount = tonumber(ARGV[5]) or 1
  local raw = redis.call("HGET", hkey, field)
  local counter, expires_at = parse(raw)
  if counter == nil or (expires_at and now > expires_at) then
    counter = 0
    expires_at = now + ttl_ms
  end
  counter = counter + amount
  redis.call("HSET", hkey, field, tostring(counter) .. "|" .. tostring(expires_at))
  -- Set a coarse TTL on the whole hash key so an abandoned shard eventually
  -- disappears too, even if individual field cleanup lags.
  redis.call("EXPIRE", hkey, math.ceil(ttl_ms / 1000) + 60)
  return { counter, expires_at - now }
end

return redis.error_reply("unknown op: " .. tostring(op))
