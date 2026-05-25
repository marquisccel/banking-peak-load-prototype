#!/bin/bash

set -e

: "${SERVER_IP:?SERVER_IP is required. Copy .env.cloud.example to .env.cloud and set SERVER_IP.}"
: "${SSH_USER:=ubuntu}"
: "${SSH_KEY:=~/.ssh/id_rsa}"
: "${REMOTE_APP_DIR:=banking-peak-load-prototype}"

SSH_KEY="${SSH_KEY/#\~/$HOME}"

echo "Showing cloud logs from $SERVER_IP ..."

ssh -i "$SSH_KEY" "$SSH_USER@$SERVER_IP" "
cd $REMOTE_APP_DIR &&
docker compose logs --tail=120 app postgres pgbouncer prometheus grafana
"
