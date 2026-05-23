import http from 'k6/http';

// Statuses intentionally returned by the API/middleware during load tests.
export const READ_EXPECTED_STATUSES = [200, 429, 503];
export const WRITE_EXPECTED_STATUSES = [201, 202, 422, 429, 503];

export const readResponseCallback = http.expectedStatuses(200, 429, 503);
export const writeResponseCallback = http.expectedStatuses(201, 202, 422, 429, 503);

export function formatStatusList(statuses) {
	return statuses.join(', ');
}

export function isExpectedReadStatus(status) {
	return READ_EXPECTED_STATUSES.includes(status);
}

export function isExpectedWriteStatus(status) {
	return WRITE_EXPECTED_STATUSES.includes(status);
}

export function isProtectedStatus(status) {
	return status === 429 || status === 503;
}

export function isBusinessRejectedStatus(status) {
	return status === 422;
}
