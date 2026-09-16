# 部署指南(iolinkd)

> 单二进制 + TimescaleDB。原型期:docker-compose 单机;TLS 公网部署加前置反代。

## 1. 构建与启动(生产)

```bash
# 1) 构建镜像(前端 dist 需先就位: web/admin/dist/,占位页可构建)
docker build -t iolinkd:latest .

# 2) 环境变量(deploy/.env)
cat > deploy/.env <<EOF
IOLINK_PG_PASSWORD=<强密码>
IOLINK_SECRET_KEY=<>=32字节随机串>
IOLINK_WX_APPID=<可选,小程序appid>
IOLINK_WX_SECRET=<可选>
IOLINK_WX_TEMPLATE_ID=<可选,报警订阅消息模板>
EOF

# 3) 启动
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d

# 4) 验证
curl http://<host>:8080/healthz        # ok
curl -s http://<host>:8080/metrics | head   # iolink_* 指标
# 管理后台: http://<host>:8080/  (admin / admin123 —— 首次登录后立即改密!)
```

## 2. 端口与 TLS

| 端口 | 用途 | 公网暴露建议 |
|---|---|---|
| 8080 | HTTP(API+后台+metrics) | 经 nginx/caddy 反代 + TLS(443) |
| 1883 | MQTT(设备) | 公网 4G 设备需 TLS:反代 8883 → 1883(tcp),固件侧 8883 |

nginx 最小配置(示意):

```nginx
server {
  listen 443 ssl;
  server_name iolink.example.com;
  ssl_certificate     /etc/letsencrypt/live/iolink.example.com/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/iolink.example.com/privkey.pem;
  location / { proxy_pass http://127.0.0.1:8080; }
}
server { listen 8883 ssl; proxy_pass 127.0.0.1:1883; }   # stream 块,需 nginx stream 模块
```

## 3. 备份

```bash
deploy/backup.sh     # 业务表每日全量 + 时序表归档,保留 14 天
# crontab -e
30 2 * * * /opt/iolink/deploy/backup.sh >> /var/log/iolink-backup.log 2>&1
```

恢复:`gunzip -c business-XXX.sql.gz | docker exec -i <pg容器> psql -U iolink -d iolink`

## 4. 微信订阅消息启用(M5)

1. 小程序后台申请「订阅消息」模板(报警类:内容项=告警描述/数值/时间)
2. env 配 `IOLINK_WX_APPID/SECRET/TEMPLATE_ID`,重启 iolinkd(日志出现 "wechat notifier enabled")
3. 小程序端在关键操作处调 `wx.requestSubscribeMessage` 攒授权;报警触发时 BFF 自动下发

## 5. 升级

```bash
git pull && docker build -t iolinkd:latest . && \
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d iolinkd
# 数据库 schema 变更随迁移文件自动执行(当前为启动时 schema.sql 幂等)
```

## 6. 监控要点

- `/healthz`:存活 + DB 连通
- `/metrics`:`iolink_devices_online`(在线数)、`iolink_telemetry_total`(上报量)、
  `iolink_alarms_total`(报警量)、`iolink_notifications_total`(通知量)
- 告警建议:devices_online 突降 50%、telemetry_total 停止增长 10 分钟
