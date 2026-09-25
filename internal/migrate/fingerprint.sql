WITH business AS (
 SELECT oid, relname FROM pg_class
 WHERE relnamespace='public'::regnamespace AND relkind IN ('r','p')
 AND relname IN ('users','farms','ponds','devices','device_shadows','alarm_rules','alarms','sensor_data')
), definitions AS (
 SELECT 'column|'||b.relname||'|'||a.attname||'|'||format_type(a.atttypid,a.atttypmod)||'|'||a.attnotnull||'|'||coalesce(pg_get_expr(d.adbin,d.adrelid),'') AS value
 FROM business b JOIN pg_attribute a ON a.attrelid=b.oid AND a.attnum>0 AND NOT a.attisdropped
 LEFT JOIN pg_attrdef d ON d.adrelid=b.oid AND d.adnum=a.attnum
 UNION ALL
 SELECT 'constraint|'||b.relname||'|'||pg_get_constraintdef(c.oid)
 FROM business b JOIN pg_constraint c ON c.conrelid=b.oid
 UNION ALL
 SELECT 'index|'||b.relname||'|'||pg_get_indexdef(i.indexrelid)
 FROM business b JOIN pg_index i ON i.indrelid=b.oid
)
SELECT coalesce(json_agg(value ORDER BY value),'[]'::json)::text FROM definitions;
