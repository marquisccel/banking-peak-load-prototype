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

export const rampUpIntervalRequests = new Counter('ramp_up_interval_requests');
export const rampUpIntervalTarget = new Gauge('ramp_up_interval_target');
export const spikeIntervalRequests = new Counter('spike_interval_requests');
export const spikeIntervalTarget = new Gauge('spike_interval_target');
export const sustainedIntervalRequests = new Counter('sustained_interval_requests');
export const sustainedIntervalTarget = new Gauge('sustained_interval_target');

const RAMP_UP_START_RATE = Number(__ENV.RAMP_UP_START_RATE || 500);
const RAMP_UP_RATE_STEP = Number(__ENV.RAMP_UP_RATE_STEP || 10);
const RAMP_UP_STAGE_DURATION = __ENV.RAMP_UP_STAGE_DURATION || '1s';
const RAMP_UP_STAGES = Number(__ENV.RAMP_UP_STAGES || 60);

const SPIKE_START_RATE = Number(__ENV.SPIKE_START_RATE || 200);
const SPIKE_RATE_STEP = Number(__ENV.SPIKE_RATE_STEP || 200);
const SPIKE_INTERVAL_SECONDS = Number(__ENV.SPIKE_INTERVAL_SECONDS || 5);
const SPIKE_INTERVALS = Number(__ENV.SPIKE_INTERVALS || 12);

const SUSTAINED_RATE = Number(__ENV.SUSTAINED_RATE || 800);
const SUSTAINED_DURATION = __ENV.SUSTAINED_DURATION || '30m';
const SUSTAINED_INTERVAL_SECONDS = Number(__ENV.SUSTAINED_INTERVAL_SECONDS || 60);

const PRE_ALLOCATED_VUS = Number(__ENV.PRE_ALLOCATED_VUS || 1000);
const MAX_VUS = Number(__ENV.MAX_VUS || 15000);
const TEST_START_MS = Date.now();

function durationToSeconds(duration) {
	const match = String(duration).trim().match(/^(\d+)(ms|s|m|h)$/);

	if (!match) {
		throw new Error(`Unsupported duration format: ${duration}`);
	}

	const value = Number(match[1]);
	const unit = match[2];

	if (unit === 'ms') {
		return value / 1000;
	}

	if (unit === 's') {
		return value;
	}

	if (unit === 'm') {
		return value * 60;
	}

	return value * 3600;
}

const RAMP_UP_STAGE_SECONDS = durationToSeconds(RAMP_UP_STAGE_DURATION);
const RAMP_UP_TOTAL_SECONDS = RAMP_UP_STAGES * RAMP_UP_STAGE_SECONDS;
const SPIKE_TOTAL_SECONDS = SPIKE_INTERVALS * SPIKE_INTERVAL_SECONDS;
const SPIKE_START_SECONDS = RAMP_UP_TOTAL_SECONDS;
const SUSTAINED_START_SECONDS = RAMP_UP_TOTAL_SECONDS + SPIKE_TOTAL_SECONDS;

function buildRampUpStages() {
	const stages = [];

	for (let index = 0; index < RAMP_UP_STAGES; index += 1) {
		stages.push({
			target: RAMP_UP_START_RATE + (index * RAMP_UP_RATE_STEP),
			duration: RAMP_UP_STAGE_DURATION,
		});
	}

	return stages;
}

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

function getScenarioInterval(startMs, intervalSeconds) {
	return Math.floor((Date.now() - startMs) / (intervalSeconds * 1000)) + 1;
}

// =========================
// Test Config
// Combines ramp-up, spike, and sustained load in a single run.
// Ramp-up warms the system, spike stresses protection layers,
// and sustained load checks long-duration stability.
// =========================
export const options = {
	scenarios: {
		ramp_up_test: {
			executor: 'ramping-arrival-rate',
				exec: 'ramp_up_test',
			startRate: RAMP_UP_START_RATE,
			timeUnit: '1s',
			preAllocatedVUs: PRE_ALLOCATED_VUS,
			maxVUs: MAX_VUS,
			stages: buildRampUpStages(),
			startTime: '0s',
		},
		spike_test: {
			executor: 'ramping-arrival-rate',
				exec: 'spike_test',
			startRate: SPIKE_START_RATE,
			timeUnit: '1s',
			preAllocatedVUs: PRE_ALLOCATED_VUS,
			maxVUs: MAX_VUS,
			stages: buildSpikeStages(),
				startTime: `${SPIKE_START_SECONDS}s`,
		},
		sustained_load_test: {
			executor: 'constant-arrival-rate',
				exec: 'sustained_load_test',
			rate: SUSTAINED_RATE,
			timeUnit: '1s',
			duration: SUSTAINED_DURATION,
			preAllocatedVUs: PRE_ALLOCATED_VUS,
			maxVUs: MAX_VUS,
				startTime: `${SUSTAINED_START_SECONDS}s`,
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

function recordBaseMetrics(res, expectedStatus) {
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
}

// =========================
// Scenario Runners
// =========================
export function ramp_up_test() {
	const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
	const interval = getScenarioInterval(TEST_START_MS, RAMP_UP_STAGE_SECONDS);
	const target = RAMP_UP_START_RATE + ((interval - 1) * RAMP_UP_RATE_STEP);

	const payload = buildTransactionPayload();
	const params = {
		headers: { 'Content-Type': 'application/json' },
		timeout: '5s',
		responseCallback: writeResponseCallback,
	};

	const res = http.post(`${BASE_URL}/api/v1/transactions`, payload, params);

	rampUpIntervalRequests.add(1, { interval: String(interval) });
	rampUpIntervalTarget.add(target, { interval: String(interval) });

	const expectedStatus = isExpectedWriteStatus(res.status);
	check(res, {
		[`status is ${formatStatusList(WRITE_EXPECTED_STATUSES)}`]: (r) => isExpectedWriteStatus(r.status),
		'response time < 2s': (r) => r.timings.duration < 2000,
	});

	recordBaseMetrics(res, expectedStatus);
	sleep(Math.random() * 0.05);
}

export function spike_test() {
	const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
	const interval = getScenarioInterval(TEST_START_MS + (SPIKE_START_SECONDS * 1000), SPIKE_INTERVAL_SECONDS);
	const target = SPIKE_START_RATE + ((interval - 1) * SPIKE_RATE_STEP);

	const payload = buildTransactionPayload();
	const params = {
		headers: { 'Content-Type': 'application/json' },
		timeout: '5s',
		responseCallback: writeResponseCallback,
	};

	const res = http.post(`${BASE_URL}/api/v1/transactions`, payload, params);

	spikeIntervalRequests.add(1, { interval: String(interval) });
	spikeIntervalTarget.add(target, { interval: String(interval) });

	const expectedStatus = isExpectedWriteStatus(res.status);
	check(res, {
		[`status is ${formatStatusList(WRITE_EXPECTED_STATUSES)}`]: (r) => isExpectedWriteStatus(r.status),
		'response time < 2s': (r) => r.timings.duration < 2000,
	});

	recordBaseMetrics(res, expectedStatus);
	sleep(Math.random() * 0.05);
}

export function sustained_load_test() {
	const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
	const interval = getScenarioInterval(TEST_START_MS + (SUSTAINED_START_SECONDS * 1000), SUSTAINED_INTERVAL_SECONDS);

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

	recordBaseMetrics(res, expectedStatus);
	sleep(Math.random() * 0.05);
}
