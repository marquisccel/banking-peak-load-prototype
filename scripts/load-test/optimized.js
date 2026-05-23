import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// =========================
// Custom Metrics
// =========================
export const successRate = new Rate('success_rate');
export const failedRequests = new Counter('failed_requests');
export const transactionLatency = new Trend('transaction_latency');

// =========================
// Test Config
// Target: ~500.000 requests
// Kalkulasi:
//   50 req/s x 30s  =  1.500
//  200 req/s x 60s  = 12.000
//  500 req/s x 120s = 60.000
//  800 req/s x 180s = 144.000
// 1000 req/s x 120s = 120.000
// 1000 req/s x 60s  =  60.000
//  500 req/s x 60s  =  30.000
//  200 req/s x 60s  =  12.000
//   50 req/s x 30s  =  1.500
// Total             = 441.000 + overhead ~500.000
// =========================
export const options = {
  scenarios: {
    peak_load_test: {
      executor: 'ramping-arrival-rate',
      startRate: 50,
      timeUnit: '1s',
      preAllocatedVUs: 200,
      maxVUs: 2000,
      stages: [
        { target: 50,   duration: '30s' },  // warm up
        { target: 200,  duration: '1m' },   // naik pelan
        { target: 500,  duration: '2m' },   // naik ke 500 req/s
        { target: 800,  duration: '3m' },   // naik ke 800 req/s
        { target: 1000, duration: '2m' },   // spike 1000 req/s
        { target: 1000, duration: '1m' },   // tahan puncak
        { target: 500,  duration: '1m' },   // turun
        { target: 200,  duration: '1m' },   // turun lagi
        { target: 0,    duration: '30s' },  // stop
      ],
    },
  },

  thresholds: {
    http_req_failed:   ['rate<0.01'],   // error < 1%
    http_req_duration: ['p(95)<1000'],  // p95 < 1s
    success_rate:      ['rate>0.99'],   // sukses > 99%
  },
};


function randomAccount() {
  // 1001 - 101000
  return Math.floor(Math.random() * 100000) + 1001;
}

function buildTransactionPayload() {
  let source = randomAccount();
  let dest = randomAccount();

  while (dest === source) {
    dest = randomAccount();
  }

  return JSON.stringify({
    source_account: source,
    dest_account: dest,
    amount: Math.floor(Math.random() * 100000) + 10000,
  });
}

// =========================
// Main Test
// =========================
export default function () {
  const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:41335';

  const payload = buildTransactionPayload();

  const params = {
    headers: { 'Content-Type': 'application/json' },
    timeout: '5s',
  };

  const res = http.post(`${BASE_URL}/api/v1/transactions`, payload, params);

  const ok = check(res, {
    'status is 201 or 202': (r) => r.status === 201 || r.status === 202,
    'response time < 2s':   (r) => r.timings.duration < 2000,
  });

  successRate.add(ok);
  transactionLatency.add(res.timings.duration);

  if (!ok) {
    failedRequests.add(1);
    console.error(`FAILED | status=${res.status} body=${res.body}`);
  }

  sleep(Math.random() * 0.1);
}
