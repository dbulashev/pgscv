-- {{ if .Config.serverVersionNum < .Const.PostgresV96 }}
SELECT coalesce(usename, 'system')                                     AS user,
       datname                                                         AS database,
       state,
       waiting,
       coalesce(extract(epoch FROM clock_timestamp() - xact_start), 0) AS active_seconds,
       CASE
           WHEN waiting = 't' THEN extract(epoch FROM clock_timestamp() - state_change)
           ELSE 0
           END                                                         AS waiting_seconds,
       left(query, 32)                                                 AS query
FROM pg_stat_activity;

-- {{ else if .Config.serverVersionNum < .Const.PostgresV10 }}
-- Postgres 9.6 doesn't have 'backend_type' attribute.
SELECT coalesce(usename, 'system')                                     AS user,
       datname                                                         AS database,
       state,
       wait_event_type,
       wait_event,
       coalesce(extract(epoch FROM clock_timestamp() - xact_start), 0) AS active_seconds,
       CASE
           WHEN wait_event_type = 'Lock' THEN extract(epoch FROM clock_timestamp() - state_change)
           ELSE 0 END                                                  AS waiting_seconds,
       left(query, 32)                                                 AS query
FROM pg_stat_activity;

-- {{ else if .Config.serverVersionNum < .Const.PostgresV14 }}
-- query for versions from 10 to 13.
SELECT coalesce(usename, backend_type)                                 AS user,
       datname                                                         AS database,
       state,
       wait_event_type,
       wait_event,
       coalesce(extract(epoch FROM clock_timestamp() - xact_start), 0) AS active_seconds,
       CASE
           WHEN wait_event_type = 'Lock'
               THEN extract(epoch FROM clock_timestamp() - state_change)
           ELSE 0 END                                                  AS waiting_seconds,
       left(query, 32)                                                 AS query
FROM pg_stat_activity;

-- {{ else }}
-- query for recent versions.
-- Postgres 14 has pg_locks.waitstart which is better for taking sessions waiting time.
SELECT coalesce(usename, backend_type)                                 AS user,
       datname                                                         AS database,
       state,
       wait_event_type,
       wait_event,
       coalesce(extract(epoch FROM clock_timestamp() - xact_start), 0) AS active_seconds,
       CASE
           WHEN wait_event_type = 'Lock'
               THEN (
                        SELECT extract(epoch FROM clock_timestamp() - max(waitstart))
                        FROM pg_locks l
                        WHERE l.pid = a.pid
                    )
           ELSE 0 END                                                  AS waiting_seconds,
       left(query, 32)                                                 AS query
FROM pg_stat_activity a;
-- {{ end }}
