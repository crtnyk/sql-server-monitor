package collector

import (
	"database/sql"
	"fmt"
)

func CollectProcStats(db *sql.DB, database string) ([]map[string]interface{}, error) {
	query := fmt.Sprintf(`
SELECT TOP 100
    DB_NAME(database_id) AS DatabaseName,
    OBJECT_NAME(object_id, database_id) AS StoredProcedure,
    execution_count,
    CAST(total_worker_time/1000.0/1000.0 AS DECIMAL(10,2)) AS total_cpu_seconds,
    CAST((total_worker_time/execution_count)/1000.0 AS DECIMAL(10,2)) AS avg_cpu_ms,
    CAST(total_logical_reads/execution_count AS BIGINT) AS avg_reads,
    CAST(total_elapsed_time/1000.0/1000.0 AS DECIMAL(10,2)) AS total_elapsed_seconds,
    CAST((total_elapsed_time/execution_count)/1000.0 AS DECIMAL(10,2)) AS avg_elapsed_ms,
    cached_time,
    last_execution_time
FROM sys.dm_exec_procedure_stats
WHERE database_id = DB_ID('%s')
ORDER BY total_worker_time DESC;
`, database)

	return executeQuery(db, query)
}

func executeQuery(db *sql.DB, query string) ([]map[string]interface{}, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range cols {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}
