// mixed_workload.js
//
// Uses k6's built-in gRPC client (k6/net/grpc) to call Aegis's real
// CheckLimit RPC directly — not HTTP against the gRPC port, which
// wouldn't work against a real gRPC server. Point K6_TARGET at your
// deployment; defaults to localhost:50051 for local testing.
import grpc from 'k6/net/grpc';
import { check } from 'k6';

const client = new grpc.Client();
client.load(['../../api/proto'], 'ratelimit.proto');

const TARGET = __ENV.K6_TARGET || 'localhost:50051';
const RATE = Number(__ENV.K6_RATE || 12000); // requests/sec

export const options = {
  scenarios: {
    high_throughput: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: __ENV.K6_DURATION || '30s',
      preAllocatedVUs: Number(__ENV.K6_VUS || 500),
      maxVUs: Number(__ENV.K6_MAX_VUS || 2000),
    },
  },
  thresholds: {
    'grpc_req_duration': ['p(99)<2'], // ms; the Claim 2 target
  },
};

let connected = false;

export default () => {
  if (!connected) {
    client.connect(TARGET, { plaintext: true });
    connected = true;
  }

  const algorithm = __ITER % 2 === 0 ? 'token_bucket' : 'sliding_window';
  const response = client.invoke('ratelimit.RateLimiter/CheckLimit', {
    key: `user_${__VU}`,
    algorithm,
    tenant: 'tenant_load_test',
    requests: 1,
  });

  check(response, {
    'status is OK': (r) => r && r.status === grpc.StatusOK,
  });
};
