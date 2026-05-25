#!/bin/bash

set -e

: "${LOADGEN_IP:?LOADGEN_IP is required. Copy .env.cloud.example to .env.cloud and set LOADGEN_IP.}"
: "${SSH_USER:=ubuntu}"
: "${SSH_KEY:=~/.ssh/id_rsa}"
: "${LOADTEST_COMMAND:=/home/ubuntu/run-mixed.sh}"

SSH_KEY="${SSH_KEY/#\~/$HOME}"

echo "Running cloud load test from $LOADGEN_IP ..."

ssh -i "$SSH_KEY" "$SSH_USER@$LOADGEN_IP" "$LOADTEST_COMMAND"
