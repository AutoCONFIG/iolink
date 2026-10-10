BEGIN;
INSERT INTO users(id,open_id,authority) VALUES (721,'m7a-hls-paths','USER');
INSERT INTO tenants(id,name) VALUES (721,'m7a-hls-paths');
INSERT INTO farms(id,owner_id,name,tenant_id) VALUES (721,721,'HLS',721);
INSERT INTO ponds(id,farm_id,name) VALUES (721,721,'HLS');
INSERT INTO video_cameras(id,tenant_id,farm_id,pond_id,name,source_kind,rtsp_uri)
VALUES (721,721,721,721,'HLS','rtsp','rtsp://192.168.10.20:554/live');
INSERT INTO video_streams(id,camera_id,tenant_id,source_version)
VALUES ('00000000-0000-4000-8000-000000000721',721,721,1);
INSERT INTO video_sessions(id,stream_id,camera_id,tenant_id,source_version,user_id,user_token_version,
tenant_permission_version,member_permission_version,token_hash,created_at,expires_at)
VALUES ('00000000-0000-4000-8000-000000000722','00000000-0000-4000-8000-000000000721',721,721,1,721,
0,0,0,decode(repeat('cc',32),'hex'),now(),now()+interval '5 minutes');
INSERT INTO video_segments(session_id,segment_id,provider_name,expires_at)
VALUES ('00000000-0000-4000-8000-000000000722','00000000-0000-4000-8000-000000000723',
        '2026-10-11/23/59-59_123.ts',now()+interval '1 minute');
DO $$
DECLARE bad TEXT;
BEGIN
    FOREACH bad IN ARRAY ARRAY['../segment.ts','/2026-10-11/23/59-59_1.ts',
        'https://example.test/segment.ts','2026-10-11/23/%2e%2e/segment.ts',
        '2026-10-11/23/59-59_1.ts?secret=x','2026-10-11/23/59-59_1.ts#x',
        '2026-10-11/24/59-59_1.ts','2026-13-11/23/59-59_1.ts','segment.ts'] LOOP
        BEGIN
            UPDATE video_segments SET provider_name=bad;
            RAISE EXCEPTION 'unsafe or unsupported segment path accepted';
        EXCEPTION WHEN check_violation THEN NULL;
        END;
    END LOOP;
END $$;
SELECT 'native ZLM dated TS path accepted; traversal, external, encoded and malformed paths rejected PASS' AS result;
ROLLBACK;
