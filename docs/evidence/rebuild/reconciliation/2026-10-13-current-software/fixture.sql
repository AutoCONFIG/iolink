
BEGIN;
INSERT INTO tenants(id,name) VALUES(700001,'ops-qa-tenant');
INSERT INTO users(id,open_id,authority) VALUES(700001,'ops-qa-user','USER');
INSERT INTO farms(id,owner_id,name,tenant_id) VALUES(700001,700001,'ops-qa-farm',700001);
INSERT INTO ponds(id,farm_id,name) VALUES(700001,700001,'ops-qa-pond');
INSERT INTO devices(id,pond_id,device_no,secret_hash,mqtt_username) VALUES(700001,700001,'ops-qa-device','test-only-unusable-hash','ops-qa-issued-user');
INSERT INTO sensor_data(ts,device_no,pond_id,tenant_id,temperature,signal) VALUES('2026-10-13T01:00:00Z','ops-qa-device',700001,700001,24.5,-62),('2026-10-13T01:01:00Z','ops-qa-device',700001,700001,25,-61);
INSERT INTO device_shadows(device_no,pond_id,tenant_id,last,ts,model_version,product_id) VALUES('ops-qa-device',700001,700001,'{"temperature":25}','2026-10-13T01:01:00Z',1,iolink_default_product());
INSERT INTO alarms(id,device_no,pond_id,metric,current_value,threshold,level,message) VALUES(700001,'ops-qa-device',700001,'temperature',25,24,'warning','ops fixture');
INSERT INTO telemetry(ts,device_no,pond_id,tenant_id,model_version,product_id,properties) VALUES('2026-10-13T01:01:00Z','ops-qa-device',700001,700001,1,iolink_default_product(),'{"temperature":25}');
INSERT INTO jobs(tenant_id,kind,idempotency_key,status,payload) VALUES(700001,'ops-qa-job','ops-qa-job-1','succeeded','{"qa":true}');
INSERT INTO outbox_events(tenant_id,topic,idempotency_key,status,payload) VALUES(700001,'ops-qa-topic','ops-qa-event-1','sent','{"qa":true}');
INSERT INTO notification_outbox(alarm_id,status,attempts) VALUES(700001,'disabled',0);
COMMIT;
