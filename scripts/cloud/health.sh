#!/bin/bash

set -e

: "${SERVER_IP:?SERVER_IP is required. Copy .env.cloud.example to .env.cloud and set SERVER_IP.}"

echo "Checking cloud endpoints on $SERVER_IP ..."

echo ""
echo "Health:"
curl -s "http://$SERVER_IP:8080/health" || true

echo ""
echo ""
echo "Metrics:"
curl -s "http://$SERVER_IP:8080/metrics" | head -n 10 || true

echo ""
echo ""
echo "Grafana:"
curl -Is "http://$SERVER_IP:3000" | head -n 1 || true

echo ""
echo "Prometheus:"
curl -Is "http://$SERVER_IP:9090" | head -n 1 || true
