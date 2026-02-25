package collector

import "database/sql"

func CollectBlockingChain(db *sql.DB) ([]map[string]interface{}, error) {
	query := `
WITH BlockingChain AS (
    SELECT
        r.session_id,
        r.blocking_session_id,
        s.login_name,
        s.host_name,
        s.program_name,
        r.wait_type,
        r.wait_time / 1000.0 AS wait_time_seconds,
        r.wait_resource,
        OBJECT_NAME(st.objectid, st.dbid) AS blocking_object,
        DB_NAME(r.database_id) AS database_name,
        SUBSTRING(st.text, (r.statement_start_offset/2)+1,
            ((CASE r.statement_end_offset
                WHEN -1 THEN DATALENGTH(st.text)
                ELSE r.statement_end_offset
            END - r.statement_start_offset)/2) + 1) AS query_text,
        r.cpu_time / 1000.0 AS cpu_time_seconds,
        r.logical_reads,
        r.transaction_isolation_level,
        0 AS blocking_level
    FROM sys.dm_exec_requests r
    INNER JOIN sys.dm_exec_sessions s ON r.session_id = s.session_id
    CROSS APPLY sys.dm_exec_sql_text(r.sql_handle) st
    WHERE r.blocking_session_id = 0
        AND EXISTS (
            SELECT 1 FROM sys.dm_exec_requests r2
            WHERE r2.blocking_session_id = r.session_id
        )

    UNION ALL

    SELECT
        r.session_id,
        r.blocking_session_id,
        s.login_name,
        s.host_name,
        s.program_name,
        r.wait_type,
        r.wait_time / 1000.0 AS wait_time_seconds,
        r.wait_resource,
        OBJECT_NAME(st.objectid, st.dbid) AS blocking_object,
        DB_NAME(r.database_id) AS database_name,
        SUBSTRING(st.text, (r.statement_start_offset/2)+1,
            ((CASE r.statement_end_offset
                WHEN -1 THEN DATALENGTH(st.text)
                ELSE r.statement_end_offset
            END - r.statement_start_offset)/2) + 1) AS query_text,
        r.cpu_time / 1000.0 AS cpu_time_seconds,
        r.logical_reads,
        r.transaction_isolation_level,
        bc.blocking_level + 1
    FROM sys.dm_exec_requests r
    INNER JOIN sys.dm_exec_sessions s ON r.session_id = s.session_id
    CROSS APPLY sys.dm_exec_sql_text(r.sql_handle) st
    INNER JOIN BlockingChain bc ON r.blocking_session_id = bc.session_id
)
SELECT * FROM BlockingChain
ORDER BY blocking_level, wait_time_seconds DESC;
`

	return executeQuery(db, query)
}
