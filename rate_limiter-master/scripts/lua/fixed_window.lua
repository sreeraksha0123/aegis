-- LUA SCRIPT FOR FIXED WINDOW REDIS
local key = KEYS[1]
local window = tonumber(ARGV[1]) -- window time
local limit = tonumber(ARGV[2]) -- limit count

-- increment counter
local current = redis.call("INCR", key)

-- get ttl of key
local ttl = redis.call("TTL", key)
if current == 1 then
    redis.call("EXPIRE", key, window)
    ttl = window
end

-- check for limit
if current > limit then
    return {0, current, 0, ttl} 
else
    return {1, current, limit - current, ttl} 
end
