SELECT
    path, count(*) as cnt
FROM access_log
WHERE access_at
    BETWEEN 'epoch'::TIMESTAMP + interval '${runtime.event.alert.opened_at} seconds'
    AND 'epoch'::TIMESTAMP + interval '${runtime.event.alert.closed_at} seconds'
GROUP BY 1
