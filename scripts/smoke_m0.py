#!/usr/bin/env python3
"""M0 CLI smoke on a disposable Docker Postgres instance. Creates/drops only its UUID database.
Set IOLINK_TEST_PG_DSN, IOLINK_TEST_CONTAINER; build /tmp/iolink-m0 and /tmp/iolink-mqtt-once.
Requires Docker exec access. Secrets are generated in memory and never written to output.
This checks current wiring, NOT all target HTTP contracts (R02.c).
"""
import json
import os
import secrets
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

container = os.environ['IOLINK_TEST_CONTAINER']
base = urllib.parse.urlsplit(os.environ['IOLINK_TEST_PG_DSN'])
assert base.scheme in ('postgres', 'postgresql')
db = 'iolink_smoke_' + uuid.uuid4().hex
binary = os.environ.get('IOLINK_TEST_BINARY', '/tmp/iolink-m0')
mqtt = os.environ.get('IOLINK_TEST_MQTT_BINARY', '/tmp/iolink-mqtt-once')

def sql(statement, database='postgres'):
    p = subprocess.run(['docker', 'exec', '-i', container, 'psql', '-X', '-v', 'ON_ERROR_STOP=1', '-U', base.username, '-d', database, '-At'], input=statement, text=True, capture_output=True, timeout=20)
    assert p.returncode == 0, 'test database SQL failed'
    return p.stdout.strip()

def port():
    with socket.socket() as s:
        s.bind(('127.0.0.1', 0))
        return s.getsockname()[1]

http_port, mqtt_port = port(), port()
env = dict(os.environ, IOLINK_PG_DSN=urllib.parse.urlunsplit(base._replace(path='/' + db)), IOLINK_SECRET_KEY=secrets.token_hex(32), IOLINK_HTTP_ADDR=f'127.0.0.1:{http_port}', IOLINK_MQTT_ADDR=f'127.0.0.1:{mqtt_port}', GIN_MODE='release')
password = secrets.token_urlsafe(24)
server = None

def cli(*args, data=None, success=True):
    p = subprocess.run([binary, *args], env=env, input=data, capture_output=True, text=True, timeout=20)
    assert (p.returncode == 0) == success, f'CLI unexpected exit for {args[0]} {p.returncode}'
    assert password not in p.stdout + p.stderr, 'secret exposed'
    return p.stdout

def http(method, path, body=None, token=None, expect=200):
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    req = urllib.request.Request(f'http://127.0.0.1:{http_port}' + path, data=None if body is None else json.dumps(body).encode(), headers=headers, method=method)
    try:
        response = urllib.request.urlopen(req, timeout=3)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        raw = response.read()
        assert response.status == expect, f'HTTP {path}: {response.status} != {expect}'
        return json.loads(raw) if raw and response.headers.get_content_type() == 'application/json' else raw

def start(log):
    global server
    server = subprocess.Popen([binary, 'serve'], env=env, stdout=log, stderr=log)
    for _ in range(100):
        assert server.poll() is None, 'server exited before ready'
        try:
            if http('GET', '/healthz') == b'ok':
                return
        except (OSError, urllib.error.URLError):
            pass
        time.sleep(.05)
    raise AssertionError('startup timeout')

def stop():
    global server
    if server is not None:
        server.terminate()
        try:
            server.wait(timeout=15)
        except subprocess.TimeoutExpired:
            server.kill()
            server.wait()
            raise AssertionError('shutdown deadline exceeded')
        assert server.returncode == 0, 'unclean shutdown'
        server = None

sql('CREATE DATABASE ' + db)
try:
    cli('serve', success=False)
    cli('migrate', 'status')
    cli('migrate', 'up')
    cli('migrate', 'up')
    cli('serve', success=False)
    cli('admin', 'init', 'operator', data='A' * 256 + '\r\ntrailing', success=False)
    cli('admin', 'init', 'operator', data=password + '\n')
    cli('admin', 'init', 'another', data=password + '\n', success=False)
    print('PASS CLI: pending schema/no admin rejected; migration idempotent; long stdin and repeat bootstrap rejected')
    with tempfile.TemporaryFile(mode='w+t') as log:
        start(log)
        http('POST', '/admin/v1/login', {'username': 'operator', 'password': 'wrong'}, expect=401)
        token = http('POST', '/admin/v1/login', {'username': 'operator', 'password': password})['token']
        http('GET', '/admin/v1/stats', token=token)
        http('GET', '/api/v9/missing', expect=404)
        farm = http('POST', '/admin/v1/farms', {'name': 'audit farm'}, token)
        farm_id = farm.get('id', farm.get('ID'))
        pond = http('POST', '/admin/v1/ponds', {'farm_id': farm_id, 'name': 'audit pond'}, token)
        pond_id = pond.get('id', pond.get('ID'))
        device = http('POST', '/admin/v1/devices', {'pond_id': pond_id, 'model': 'audit'}, token)
        p = subprocess.run([mqtt, '-addr', f'127.0.0.1:{mqtt_port}', '-device', device['device_no'], '-secret', device['secret'], '-payload', '{"temperature":26,"signal":-65}'], capture_output=True, timeout=15)
        assert p.returncode == 0, 'MQTT publish failed'
        assert sql('SELECT count(*) FROM sensor_data WHERE temperature=26 AND signal=-65', db) == '1'
        ponds = http('GET', '/admin/v1/ponds', token=token)
        assert ponds[0]['latest']['temperature'] == 26, 'admin telemetry dependency missing'
        stop()
        start(log)
        assert http('GET', '/admin/v1/ponds', token=token)[0]['latest']['temperature'] == 26
        stop()
        log.seek(0)
        output = log.read()
        assert password not in output and device['secret'] not in output and env['IOLINK_SECRET_KEY'] not in output
    print('PASS process: login/health/JSON404; real MQTT -> Timescale signal -> admin latest; restart persists; SIGTERM exits zero; no tested secret in logs')
    print('Environment: ' + sql("SELECT current_setting('server_version') || '; Timescale ' || extversion FROM pg_extension WHERE extname='timescaledb'", db))
finally:
    try:
        stop()
    finally:
        sql('DROP DATABASE ' + db + ' WITH (FORCE)')
        print('CLEANUP: removed this run UUID test database')
