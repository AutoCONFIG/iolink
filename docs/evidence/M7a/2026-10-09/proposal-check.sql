BEGIN;
INSERT INTO users(id,open_id,authority) VALUES (701,'m7a-fixture-a','USER'),(702,'m7a-fixture-b','USER');
INSERT INTO tenants(id,name) VALUES (701,'m7a-fixture-a'),(702,'m7a-fixture-b');
INSERT INTO farms(id,owner_id,name,tenant_id) VALUES (701,701,'Farm A',701),(702,702,'Farm B',702);
INSERT INTO ponds(id,farm_id,name) VALUES (701,701,'Pond A'),(702,702,'Pond B');
INSERT INTO video_gb_devices(id,tenant_id,device_id,name,credential_cipher)
VALUES (701,701,'34020000001320000001','GB A',decode(repeat('aa',29),'hex')),
       (702,702,'34020000001320000002','GB B',decode(repeat('bb',29),'hex'));
INSERT INTO video_gb_channels(device_id,tenant_id,channel_id,name,catalog_sn)
VALUES (701,701,'34020000001320000011','Channel A',1),(702,702,'34020000001320000012','Channel B',1);
INSERT INTO video_cameras(id,tenant_id,farm_id,pond_id,name,source_kind,rtsp_uri)
VALUES (701,701,701,701,'RTSP A','rtsp','rtsp://192.168.10.20:554/live');
INSERT INTO video_cameras(id,tenant_id,farm_id,pond_id,name,source_kind,gb_device_id,gb_channel_id)
VALUES (702,701,701,701,'GB A','gb28181',701,'34020000001320000011');
INSERT INTO video_streams(id,camera_id,tenant_id,source_version)
VALUES ('00000000-0000-4000-8000-000000000701',701,701,1);
INSERT INTO video_sessions(id,stream_id,camera_id,tenant_id,source_version,user_id,user_token_version,
tenant_permission_version,member_permission_version,token_hash,created_at,expires_at)
VALUES ('00000000-0000-4000-8000-000000000702','00000000-0000-4000-8000-000000000701',701,701,1,701,0,
0,0,decode(repeat('aa',32),'hex'),now(),now()+interval '5 minutes');
DO $$
DECLARE n INTEGER;
BEGIN
    SELECT count(*) INTO n FROM video_cameras WHERE tenant_id=701;
    IF n<>2 THEN RAISE EXCEPTION 'valid RTSP/GB fixture failed'; END IF;
    BEGIN
        INSERT INTO video_cameras(tenant_id,farm_id,pond_id,name,source_kind,rtsp_uri)
        VALUES (702,701,701,'Cross tenant','rtsp','rtsp://host/live');
        RAISE EXCEPTION 'cross tenant farm accepted';
    EXCEPTION WHEN foreign_key_violation THEN NULL;
    END;
    BEGIN
        INSERT INTO video_cameras(tenant_id,farm_id,pond_id,name,source_kind,rtsp_uri)
        VALUES (701,701,702,'Cross pond','rtsp','rtsp://host/live');
        RAISE EXCEPTION 'cross farm pond accepted';
    EXCEPTION WHEN foreign_key_violation THEN NULL;
    END;
    BEGIN
        INSERT INTO video_cameras(tenant_id,farm_id,pond_id,name,source_kind,gb_device_id,gb_channel_id)
        VALUES (701,701,701,'Cross GB','gb28181',702,'34020000001320000012');
        RAISE EXCEPTION 'cross tenant channel accepted';
    EXCEPTION WHEN foreign_key_violation THEN NULL;
    END;
    BEGIN
        INSERT INTO video_cameras(tenant_id,farm_id,pond_id,name,source_kind,rtsp_uri,gb_device_id,gb_channel_id)
        VALUES (701,701,701,'Mixed source','rtsp','rtsp://host/live',701,'34020000001320000011');
        RAISE EXCEPTION 'mixed source accepted';
    EXCEPTION WHEN check_violation THEN NULL;
    END;
    BEGIN
        UPDATE video_sessions SET expires_at=created_at+interval '301 seconds';
        RAISE EXCEPTION 'overlong session accepted';
    EXCEPTION WHEN check_violation THEN NULL;
    END;
    BEGIN
        UPDATE video_sessions SET tenant_id=702;
        RAISE EXCEPTION 'session cross tenant stream accepted';
    EXCEPTION WHEN foreign_key_violation THEN NULL;
    END;
    BEGIN
        UPDATE video_sessions SET state='revoked';
        RAISE EXCEPTION 'revocation without timestamp accepted';
    EXCEPTION WHEN check_violation THEN NULL;
    END;
    BEGIN
        UPDATE video_sessions SET token_hash=decode('aa','hex');
        RAISE EXCEPTION 'short token hash accepted';
    EXCEPTION WHEN check_violation THEN NULL;
    END;
    BEGIN
        UPDATE video_streams SET rtp_port=30000;
        RAISE EXCEPTION 'partial RTP allocation accepted';
    EXCEPTION WHEN check_violation THEN NULL;
    END;
    BEGIN
        UPDATE video_gb_devices SET registration_state='registered';
        RAISE EXCEPTION 'registration without peer accepted';
    EXCEPTION WHEN check_violation THEN NULL;
    END;
END $$;
SELECT 'valid sources, tenant/farm/channel/session FKs, source exclusivity, TTL, revocation, hash, RTP, registration PASS' AS result;
ROLLBACK;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM video_cameras) OR EXISTS(SELECT 1 FROM video_sessions) THEN
        RAISE EXCEPTION 'fixture rollback failed';
    END IF;
END $$;
SELECT 'fixture transaction rollback PASS' AS result;
