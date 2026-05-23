import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
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

const INITIAL_RATE = Number(__ENV.INITIAL_RATE || 500);
const RATE_STEP = Number(__ENV.RATE_STEP || 10);
const STAGE_DURATION = __ENV.STAGE_DURATION || '1s';
const MAX_STAGES = Number(__ENV.MAX_STAGES || 60); // 1 minute
const PRE_ALLOCATED_VUS = Number(__ENV.PRE_ALLOCATED_VUS || 200);
const MAX_VUS = Number(__ENV.MAX_VUS || 5000);

function buildRampStages() {
	const stages = [];

	for (let index = 0; index < MAX_STAGES; index += 1) {
		stages.push({
			target: INITIAL_RATE + (index * RATE_STEP),
			duration: STAGE_DURATION,
		});
	}

	return stages;
}

// =========================
// Test Config
// Ramps up every 2s until the run is stopped or MAX_STAGES is reached.
// =========================
export const options = {
	scenarios: {
		continuous_ramp_up: {
			executor: 'ramping-arrival-rate',
			startRate: INITIAL_RATE,
			timeUnit: '1s',
			preAllocatedVUs: PRE_ALLOCATED_VUS,
			maxVUs: MAX_VUS,
			stages: buildRampStages(),
		},
	},

	thresholds: {
		http_req_failed: ['rate<0.01'],
		http_req_duration: ['p(95)<1000'],
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

	const payload = buildTransactionPayload();

	const params = {
		headers: { 'Content-Type': 'application/json' },
		timeout: '5s',
		responseCallback: writeResponseCallback,
	};

	const res = http.post(`${BASE_URL}/api/v1/transactions`, payload, params);

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

	sleep(Math.random() * 0.1);
}
