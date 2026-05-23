import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Gauge, Rate, Trend } from 'k6/metrics';
import {
	WRITE_EXPECTED_STATUSES,
	formatStatusList,
	isBusinessRejectedStatus,
	isExpectedWriteStatus,
	isProtectedStatus,
	writeResponseCallback,
} from './status.js';

// =========================
// Custom Metrics
// =========================
export const successRate = new Rate('success_rate');
export const failedRequests = new Counter('failed_requests');
export const transactionLatency = new Trend('transaction_latency');
export const protectedRequests = new Counter('protected_requests');
export const businessRejectedRequests = new Counter('business_rejected_requests');
export const sustainedIntervalRequests = new Counter('sustained_interval_requests');
export const sustainedIntervalTarget = new Gauge('sustained_interval_target');

const SUSTAINED_RATE = Number(__ENV.SUSTAINED_RATE || 800);
const SUSTAINED_DURATION = __ENV.SUSTAINED_DURATION || '30m';
const INTERVAL_SECONDS = Number(__ENV.INTERVAL_SECONDS || 60);
const PRE_ALLOCATED_VUS = Number(__ENV.PRE_ALLOCATED_VUS || 500);
const MAX_VUS = Number(__ENV.MAX_VUS || 10000);

const TEST_START_MS = Date.now();

function getCurrentInterval() {
	return Math.floor((Date.now() - TEST_START_MS) / (INTERVAL_SECONDS * 1000)) + 1;
}

// =========================
// Test Config
// Mempertahankan beban tinggi dalam durasi panjang untuk menguji
// stabilitas Connection Pooler (PgBouncer), Read Replica, dan
// Asynchronous Queue (RabbitMQ).
// =========================
export const options = {
	scenarios: {
		sustained_load_test: {
			executor: 'constant-arrival-rate',
			rate: SUSTAINED_RATE,
			timeUnit: '1s',
			duration: SUSTAINED_DURATION,
			preAllocatedVUs: PRE_ALLOCATED_VUS,
			maxVUs: MAX_VUS,
		},
	},

	thresholds: {
		http_req_failed: ['rate<0.01'],
		http_req_duration: ['p(95)<1500'],
		success_rate: ['rate>0.99'],
	},
};

function randomAccount() {
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
	const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
	const interval = getCurrentInterval();

	const payload = buildTransactionPayload();

	const params = {
		headers: { 'Content-Type': 'application/json' },
		timeout: '5s',
		responseCallback: writeResponseCallback,
	};

	const res = http.post(`${BASE_URL}/api/v1/transactions`, payload, params);

	sustainedIntervalRequests.add(1, { interval: String(interval) });
	sustainedIntervalTarget.add(SUSTAINED_RATE, { interval: String(interval) });

	const expectedStatus = isExpectedWriteStatus(res.status);
	check(res, {
		[`status is ${formatStatusList(WRITE_EXPECTED_STATUSES)}`]: (r) => isExpectedWriteStatus(r.status),
		'response time < 2s': (r) => r.timings.duration < 2000,
	});

	successRate.add(expectedStatus);
	transactionLatency.add(res.timings.duration);

	if (isProtectedStatus(res.status)) {
		protectedRequests.add(1);
	}

	if (isBusinessRejectedStatus(res.status)) {
		businessRejectedRequests.add(1);
	}

	if (!expectedStatus) {
		failedRequests.add(1);
		console.error(`UNEXPECTED STATUS | status=${res.status} body=${res.body}`);
	}

	sleep(Math.random() * 0.05);
}
