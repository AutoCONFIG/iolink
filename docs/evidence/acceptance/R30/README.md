# R30 software evidence

The production Compose file binds HTTP and MQTT to loopback and does not publish PostgreSQL. `deploy/nginx.conf.example` exposes only 443 and 8883 with explicit certificate paths and denies `/metrics`. TLS certificate and firewall validation on a real deployment host remains `external_blocked`.
