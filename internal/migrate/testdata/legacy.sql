-- IoLink 数据库 schema v0.1 (PostgreSQL 15+ / TimescaleDB)
-- 领域链: User -> Farm -> Pond -> Device -> Sensor(物模型固定,暂不建 sensor 表)
-- 状态: 草案,评审后冻结;迁移文件由 core 负责人维护

CREATE EXTENSION IF NOT EXISTS timescaledb;

-- ---------- 业务表 ----------

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    open_id       VARCHAR(64)  NOT NULL UNIQUE,        -- 微信 openid
    username      VARCHAR(64)  UNIQUE,                 -- 管理后台登录名
    password_hash VARCHAR(64),                         -- 管理员口令 sha256(iolink-admin:pw)
    authority     VARCHAR(16)  NOT NULL DEFAULT 'USER',-- USER | ADMIN
    nickname      VARCHAR(64),
    phone         VARCHAR(20),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- 设备影子: 每设备最新物模型值(含 battery/signal), /water/latest 读这里
CREATE TABLE device_shadows (
    device_no VARCHAR(64) PRIMARY KEY,
    last      JSONB        NOT NULL,
    signal    INT,
    ts        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE farms (
    id          BIGSERIAL PRIMARY KEY,
    owner_id    BIGINT       NOT NULL REFERENCES users(id),
    name        VARCHAR(128) NOT NULL,
    location    VARCHAR(256),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE ponds (
    id          BIGSERIAL PRIMARY KEY,
    farm_id     BIGINT       NOT NULL REFERENCES farms(id),
    name        VARCHAR(128) NOT NULL,
    area_mu     NUMERIC(10,2),                          -- 面积(亩)
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE devices (
    id          BIGSERIAL PRIMARY KEY,
    pond_id     BIGINT       NOT NULL REFERENCES ponds(id),
    device_no   VARCHAR(64)  NOT NULL UNIQUE,          -- 硬件身份,MQTT username
    secret_hash VARCHAR(128) NOT NULL,                -- 设备密钥哈希(access 鉴权用)
    model       VARCHAR(64),
    status      VARCHAR(16)  NOT NULL DEFAULT 'offline',  -- online | offline
    last_seen_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_devices_pond ON devices(pond_id);

-- 报警规则: 按池塘配置阈值(如 池塘A DO下限 4.0, 池塘B 5.0)
CREATE TABLE alarm_rules (
    id          BIGSERIAL PRIMARY KEY,
    pond_id     BIGINT       NOT NULL REFERENCES ponds(id),
    metric      VARCHAR(32)  NOT NULL,   -- dissolved_oxygen | ph | temperature | ...
    min_value   DOUBLE PRECISION,        -- NULL = 不设下限
    max_value   DOUBLE PRECISION,        -- NULL = 不设上限
    level       VARCHAR(16)  NOT NULL DEFAULT 'warning',  -- critical | warning
    enabled     BOOLEAN      NOT NULL DEFAULT TRUE,
    UNIQUE (pond_id, metric)
);

CREATE TABLE alarms (
    id            BIGSERIAL PRIMARY KEY,
    device_no     VARCHAR(64)  NOT NULL,
    pond_id       BIGINT       NOT NULL REFERENCES ponds(id),
    metric        VARCHAR(32)  NOT NULL,
    current_value DOUBLE PRECISION NOT NULL,
    threshold     DOUBLE PRECISION NOT NULL,
    level         VARCHAR(16)  NOT NULL,
    message       VARCHAR(256),
    confirmed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_alarms_pond_time ON alarms(pond_id, created_at DESC);
CREATE INDEX idx_alarms_open ON alarms(confirmed_at) WHERE confirmed_at IS NULL;

-- ---------- 时序表 (hypertable) ----------

-- 宽表:一台上报一行,字段与物模型对应;<100 台 × 1min ≈ 14 万行/天,毫无压力
CREATE TABLE sensor_data (
    ts              TIMESTAMPTZ NOT NULL,
    device_no       VARCHAR(64) NOT NULL,
    temperature     DOUBLE PRECISION,
    dissolved_oxygen DOUBLE PRECISION,
    ph              DOUBLE PRECISION,
    turbidity       DOUBLE PRECISION,
    salinity        DOUBLE PRECISION,
    battery         DOUBLE PRECISION
);
SELECT create_hypertable('sensor_data', 'ts');
SELECT add_retention_policy('sensor_data', INTERVAL '13 months');  -- 归档策略由 core 决定
CREATE INDEX idx_sensor_data_device_ts ON sensor_data(device_no, ts DESC);

-- 种子管理员(原型期): 用户名 admin / 口令 admin123 —— 首次部署后必须修改
INSERT INTO users (open_id, username, password_hash, authority, nickname)
VALUES ('internal-admin', 'admin', '1cd663ce3300b9f52a357c4ae4e114064b0fa066071728aca1d7a98f5916f2e0',
        'ADMIN', '管理员')
ON CONFLICT (open_id) DO NOTHING;
