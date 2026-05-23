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
export const spikeIntervalRequests = new Counter('spike_interval_requests');
export const spikeIntervalTarget = new Gauge('spike_interval_target');

const SPIKE_START_RATE = Number(__ENV.SPIKE_START_RATE || 200);
const SPIKE_RATE_STEP = Number(__ENV.SPIKE_RATE_STEP || 200);
const SPIKE_INTERVAL_SECONDS = Number(__ENV.SPIKE_INTERVAL_SECONDS || 5);
const SPIKE_INTERVALS = Number(__ENV.SPIKE_INTERVALS || 12); // 1 minute total
const PRE_ALLOCATED_VUS = Number(__ENV.PRE_ALLOCATED_VUS || 500);
const MAX_VUS = Number(__ENV.MAX_VUS || 10000);

const TEST_START_MS = Date.now();

function buildSpikeStages() {
	const stages = [];

	for (let index = 0; index < SPIKE_INTERVALS; index += 1) {
		stages.push({
			target: SPIKE_START_RATE + (index * SPIKE_RATE_STEP),
			duration: `${SPIKE_INTERVAL_SECONDS}s`,
		});
	}

	return stages;
}

function getSpikeInterval() {
	return Math.floor((Date.now() - TEST_START_MS) / (SPIKE_INTERVAL_SECONDS * 1000)) + 1;
}

function getSpikeTargetRate(interval) {
	return SPIKE_START_RATE + ((interval - 1) * SPIKE_RATE_STEP);
}

// =========================
// Test Config
// Menembakkan request masif dalam waktu singkat untuk memverifikasi
// proteksi Rate Limiter dan Circuit Breaker (HTTP 429 dan HTTP 503).
// =========================
export const options = {
	scenarios: {
		spike_test: {
			executor: 'ramping-arrival-rate',
			startRate: SPIKE_START_RATE,
			timeUnit: '1s',
			preAllocatedVUs: PRE_ALLOCATED_VUS,
			maxVUs: MAX_VUS,
			stages: buildSpikeStages(),
		},
	},

	thresholds: {
		http_req_failed: ['rate<0.01'],
		http_req_duration: ['p(95)<1000'],
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
	const spikeInterval = getSpikeInterval();
	const spikeTarget = getSpikeTargetRate(spikeInterval);

	const payload = buildTransactionPayload();

	const params = {
		headers: { 'Content-Type': 'application/json' },
		timeout: '5s',
		responseCallback: writeResponseCallback,
	};

	const res = http.post(`${BASE_URL}/api/v1/transactions`, payload, params);

	spikeIntervalRequests.add(1, { interval: String(spikeInterval) });
	spikeIntervalTarget.add(spikeTarget, { interval: String(spikeInterval) });

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
