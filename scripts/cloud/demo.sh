#!/bin/bash

set -e

: "${SERVER_IP:?SERVER_IP is required. Copy .env.cloud.example to .env.cloud and set SERVER_IP.}"
: "${SSH_USER:=ubuntu}"
: "${SSH_KEY:=~/.ssh/id_rsa}"
: "${REMOTE_APP_DIR:=banking-peak-load-prototype}"

SSH_KEY="${SSH_KEY/#\~/$HOME}"

echo "Starting cloud stack on $SERVER_IP ..."

ssh -i "$SSH_KEY" "$SSH_USER@$SERVER_IP" "
cd $REMOTE_APP_DIR &&
docker compose up -d
"

echo ""
echo "Cloud services:"
echo "Application : http://$SERVER_IP:8080"
echo "Grafana     : http://$SERVER_IP:3000"
echo "Prometheus  : http://$SERVER_IP:9090"
