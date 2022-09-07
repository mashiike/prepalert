SELECT
    path, count(*) as cnt
FROM access_log
WHERE access_at
    BETWEEN 'epoch'::TIMESTAMP + interval '{{ .Alert.OpenedAt }} seconds'
    AND 'epoch'::TIMESTAMP + interval '{{ .Alert.ClosedAt }} seconds'
GROUP BY 1
