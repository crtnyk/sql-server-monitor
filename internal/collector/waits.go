package collector

import "database/sql"

func CollectWaitStats(db *sql.DB) ([]map[string]interface{}, error) {
	query := `
SELECT
    wait_type,
    waiting_tasks_count,
    wait_time_ms / 1000.0 AS wait_time_seconds,
    (wait_time_ms - signal_wait_time_ms) / 1000.0 AS resource_wait_seconds,
    signal_wait_time_ms / 1000.0 AS signal_wait_seconds,
    CASE WHEN waiting_tasks_count > 0
         THEN wait_time_ms / waiting_tasks_count
         ELSE 0 END AS avg_wait_time_ms
FROM sys.dm_os_wait_stats
WHERE wait_time_ms > 0
    AND wait_type NOT IN (
        'CLR_SEMAPHORE', 'LAZYWRITER_SLEEP', 'RESOURCE_QUEUE', 'SLEEP_TASK',
        'SLEEP_SYSTEMTASK', 'SQLTRACE_BUFFER_FLUSH', 'WAITFOR', 'LOGMGR_QUEUE',
        'CHECKPOINT_QUEUE', 'REQUEST_FOR_DEADLOCK_SEARCH', 'XE_TIMER_EVENT',
        'BROKER_TO_FLUSH', 'BROKER_TASK_STOP', 'CLR_MANUAL_EVENT',
        'CLR_AUTO_EVENT', 'DISPATCHER_QUEUE_SEMAPHORE', 'FT_IFTS_SCHEDULER_IDLE_WAIT',
        'XE_DISPATCHER_WAIT', 'XE_DISPATCHER_JOIN', 'SQLTRACE_INCREMENTAL_FLUSH_SLEEP',
        'ONDEMAND_TASK_QUEUE', 'BROKER_EVENTHANDLER', 'SLEEP_BPOOL_FLUSH',
        'DIRTY_PAGE_POLL', 'HADR_FILESTREAM_IOMGR_IOCOMPLETION'
    )
ORDER BY wait_time_ms DESC;
`

	return executeQuery(db, query)
}
