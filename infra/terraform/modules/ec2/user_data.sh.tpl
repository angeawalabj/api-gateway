#!/usr/bin/env bash
set -euo pipefail
exec > /var/log/user-data.log 2>&1

echo "[$(date)] Starting user_data"

apt-get update -y
apt-get install -y curl awscli

mkdir -p /opt/gateway
aws s3 cp s3://${s3_bucket}/gateway/${gateway_version}/gateway-linux-amd64 /opt/gateway/gateway
chmod +x /opt/gateway/gateway

cat > /etc/gateway.env << EOF
GATEWAY_ADDR=:8080
METRICS_ADDR=:9090
POSTGRES_DSN=${postgres_dsn}
REDIS_ADDR=${redis_addr}
LOG_LEVEL=info
DEV_MODE=false
JWT_ISSUER=api-gateway
EOF
chmod 600 /etc/gateway.env

cat > /etc/systemd/system/gateway.service << 'UNIT'
[Unit]
Description=API Gateway
After=network.target

[Service]
Type=simple
User=nobody
EnvironmentFile=/etc/gateway.env
ExecStart=/opt/gateway/gateway
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload
systemctl enable gateway
systemctl start gateway

sleep 5
curl -sf http://localhost:8080/health && echo "[$(date)] Health check OK" || echo "[$(date)] Health check FAILED"
echo "[$(date)] Done — env=${environment}"
