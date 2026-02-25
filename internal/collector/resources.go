package collector

import "database/sql"

func CollectServerResources(db *sql.DB) ([]map[string]interface{}, error) {
	query := `
SELECT
    GETDATE() AS collection_timestamp,
    (SELECT TOP 1
        100 - SystemIdle AS cpu_percent
     FROM (
        SELECT
            record.value('(./Record/SchedulerMonitorEvent/SystemHealth/SystemIdle)[1]', 'int') AS SystemIdle
        FROM (
            SELECT CAST(record AS XML) AS record
            FROM sys.dm_os_ring_buffers
            WHERE ring_buffer_type = N'RING_BUFFER_SCHEDULER_MONITOR'
              AND record LIKE '%<SystemHealth>%'
        ) AS x
     ) AS y
     ORDER BY SystemIdle DESC
    ) AS cpu_percent,
    CAST((total_physical_memory_kb - available_physical_memory_kb) * 1.0 / 1024 / 1024 AS DECIMAL(10,2)) AS memory_used_gb,
    CAST(available_physical_memory_kb * 1.0 / 1024 / 1024 AS DECIMAL(10,2)) AS memory_available_gb,
    CAST(total_physical_memory_kb * 1.0 / 1024 / 1024 AS DECIMAL(10,2)) AS total_physical_memory_gb,
    CAST((total_physical_memory_kb - available_physical_memory_kb) * 100.0 / total_physical_memory_kb AS DECIMAL(5,2)) AS memory_percent_used,
    (SELECT cntr_value
     FROM sys.dm_os_performance_counters
     WHERE counter_name = 'Page life expectancy'
       AND object_name LIKE '%Buffer Manager%') AS page_life_expectancy,
    (SELECT CAST(
        (a.cntr_value * 1.0 / NULLIF(b.cntr_value, 0)) * 100.0 AS DECIMAL(5,2))
     FROM sys.dm_os_performance_counters a
     CROSS JOIN sys.dm_os_performance_counters b
     WHERE a.counter_name = 'Buffer cache hit ratio'
       AND b.counter_name = 'Buffer cache hit ratio base'
       AND a.object_name LIKE '%Buffer Manager%'
       AND b.object_name LIKE '%Buffer Manager%') AS buffer_cache_hit_ratio_percent,
    (SELECT cntr_value
     FROM sys.dm_os_performance_counters
     WHERE counter_name = 'Batch Requests/sec'
       AND object_name LIKE '%SQL Statistics%') AS batch_requests_per_sec,
    (SELECT cntr_value
     FROM sys.dm_os_performance_counters
     WHERE counter_name = 'SQL Compilations/sec'
       AND object_name LIKE '%SQL Statistics%') AS sql_compilations_per_sec,
    (SELECT cntr_value
     FROM sys.dm_os_performance_counters
     WHERE counter_name = 'SQL Re-Compilations/sec'
       AND object_name LIKE '%SQL Statistics%') AS sql_recompilations_per_sec,
    (SELECT cntr_value / 1024
     FROM sys.dm_os_performance_counters
     WHERE counter_name = 'Target Server Memory (KB)'
       AND object_name LIKE '%Memory Manager%') AS target_server_memory_mb,
    (SELECT cntr_value / 1024
     FROM sys.dm_os_performance_counters
     WHERE counter_name = 'Total Server Memory (KB)'
       AND object_name LIKE '%Memory Manager%') AS total_server_memory_mb,
    (SELECT COUNT(*)
     FROM sys.dm_exec_sessions
     WHERE is_user_process = 1) AS active_user_connections,
    (SELECT COUNT(DISTINCT blocking_session_id)
     FROM sys.dm_exec_requests
     WHERE blocking_session_id > 0) AS blocking_sessions_count,
    (SELECT
        CAST(SUM(io_stall_read_ms) * 1.0 / NULLIF(SUM(num_of_reads), 0) AS DECIMAL(10,2))
     FROM sys.dm_io_virtual_file_stats(NULL, NULL)) AS avg_disk_read_latency_ms,
    (SELECT
        CAST(SUM(io_stall_write_ms) * 1.0 / NULLIF(SUM(num_of_writes), 0) AS DECIMAL(10,2))
     FROM sys.dm_io_virtual_file_stats(NULL, NULL)) AS avg_disk_write_latency_ms,
    (SELECT CAST(SUM(wait_time) / 1000.0 AS DECIMAL(10,2))
     FROM sys.dm_exec_requests
     WHERE wait_time > 0) AS total_wait_time_seconds
FROM sys.dm_os_sys_memory;
`

	return executeQuery(db, query)
}
