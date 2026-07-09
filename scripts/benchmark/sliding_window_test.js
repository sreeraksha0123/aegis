// sliding_window_test.js — isolates sliding_window algorithm under load,
// separate from the mixed workload in mixed_workload.js.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const client = new grpc.Client();
client.load(['../../api/proto'], 'ratelimit.proto');

const TARGET = __ENV.K6_TARGET || 'localhost:50051';

export const options = {
  scenarios: {
    sliding_window_load: {
      executor: 'constant-arrival-rate',
      rate: Number(__ENV.K6_RATE || 12000),
      timeUnit: '1s',
      duration: __ENV.K6_DURATION || '30s',
      preAllocatedVUs: Number(__ENV.K6_VUS || 500),
      maxVUs: Number(__ENV.K6_MAX_VUS || 2000),
    },
  },
  thresholds: {
    'grpc_req_duration': ['p(99)<2'],
  },
};

let connected = false;

export default () => {
  if (!connected) {
    client.connect(TARGET, { plaintext: true });
    connected = true;
  }

  const response = client.invoke('ratelimit.RateLimiter/CheckLimit', {
    key: `user_${__VU}`,
    algorithm: 'sliding_window',
    tenant: 'tenant_load_test',
    requests: 1,
  });

  check(response, { 'status is OK': (r) => r && r.status === grpc.StatusOK });
};
