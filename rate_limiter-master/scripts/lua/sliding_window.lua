--- Sliding window algorithm lua
local key = KEYS[1]
local window = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local request_id = ARGV[4]

-- Remove old timestamps
redis.call('ZREMRANGEBYSCORE', key, 0, now - window)

-- Count current window
local count = redis.call('ZCARD', key)

-- Get oldest timestamp to calculate reset time
local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')

local reset_time = 0
if #oldest >= 2 then
    reset_time = math.ceil((tonumber(oldest[2]) + window) / 1000)
end

if count < limit then
    redis.call('ZADD', key, now, request_id)
    redis.call('EXPIRE', key, math.ceil(window / 1000))
    -- If this is first request, reset_time is now + window
    if count == 0 then
        reset_time = math.ceil((now + window) / 1000)
    end
    return {1, count + 1, limit - count - 1, reset_time}
else
    return {0, count, 0, reset_time}
end