BEGIN;
INSERT INTO users(id,open_id,authority) VALUES (711,'m7a-port-pairs','USER');
INSERT INTO tenants(id,name) VALUES (711,'m7a-port-pairs');
INSERT INTO farms(id,owner_id,name,tenant_id) VALUES (711,711,'Ports',711);
INSERT INTO ponds(id,farm_id,name) VALUES (711,711,'Ports');
INSERT INTO video_cameras(id,tenant_id,farm_id,pond_id,name,source_kind,rtsp_uri)
VALUES (711,711,711,711,'Ports','rtsp','rtsp://192.168.10.20:554/live');
INSERT INTO video_streams(id,camera_id,tenant_id,source_version,rtp_port,ssrc,call_id,cseq)
SELECT ('00000000-0000-4000-8000-' || lpad(n::text,12,'0'))::uuid,
       711,711,n,30000+(n-1)*2,lpad(n::text,10,'0'),
       ('00000000-0000-4000-9000-' || lpad(n::text,12,'0'))::uuid,1
FROM generate_series(1,20) AS n;
DO $$
DECLARE n INTEGER;
BEGIN
    SELECT count(*) INTO n FROM video_streams WHERE rtp_port IS NOT NULL;
    IF n<>20 THEN RAISE EXCEPTION 'twenty port pairs not allocated'; END IF;
    IF EXISTS(SELECT 1 FROM video_streams a JOIN video_streams b
              ON a.rtp_port+1=b.rtp_port AND a.id<>b.id) THEN
        RAISE EXCEPTION 'RTP collides with another RTCP';
    END IF;
    BEGIN
        UPDATE video_streams SET rtp_port=30001 WHERE source_version=1;
        RAISE EXCEPTION 'odd RTP port accepted';
    EXCEPTION WHEN check_violation THEN NULL;
    END;
    BEGIN
        UPDATE video_streams SET rtp_port=30040 WHERE source_version=1;
        RAISE EXCEPTION 'out of range RTP port accepted';
    EXCEPTION WHEN check_violation THEN NULL;
    END;
    BEGIN
        UPDATE video_streams SET rtp_port=30002 WHERE source_version=1;
        RAISE EXCEPTION 'duplicate active RTP pair accepted';
    EXCEPTION WHEN unique_violation THEN NULL;
    END;
END $$;
SELECT 'twenty disjoint RTP/RTCP pairs; odd, out of range and duplicate ports rejected PASS' AS result;
ROLLBACK;
