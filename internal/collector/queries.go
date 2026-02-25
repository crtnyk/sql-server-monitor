package collector

import "database/sql"

func CollectActiveQueries(db *sql.DB) ([]map[string]interface{}, error) {
	query := `
SELECT
    s.session_id,
    r.status AS task_state,
    r.command,
    r.wait_type,
    OBJECT_NAME(st.objectid, st.dbid) AS sp_name,
    r.wait_time / 1000.0 AS wait_time_seconds,
    r.blocking_session_id,
    r.cpu_time / 1000.0 AS cpu_time_seconds,
    r.total_elapsed_time / 1000.0 AS total_elapsed_time_seconds,
    r.reads,
    r.writes,
    r.logical_reads,
    r.granted_query_memory * 8 / 1024.0 AS granted_memory_mb,
    r.open_transaction_count,
    r.percent_complete,
    CASE WHEN r.estimated_completion_time > 0
         THEN r.estimated_completion_time / 1000.0 / 60.0
         ELSE NULL END AS est_minutes_remaining,
    SUBSTRING(
        st.text,
        (r.statement_start_offset/2)+1,
        ((CASE r.statement_end_offset
            WHEN -1 THEN DATALENGTH(st.text)
            ELSE r.statement_end_offset
        END - r.statement_start_offset)/2) + 1
    ) AS query,
    s.login_name,
    s.host_name,
    s.program_name,
    DB_NAME(r.database_id) AS database_name
FROM
    sys.dm_exec_sessions s
INNER JOIN
    sys.dm_exec_requests r ON s.session_id = r.session_id
CROSS APPLY
    sys.dm_exec_sql_text(r.sql_handle) st
WHERE
    s.is_user_process = 1
ORDER BY
    r.cpu_time DESC;
`

	return executeQuery(db, query)
}
