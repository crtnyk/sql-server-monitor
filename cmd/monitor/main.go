package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"

	"sql-server-monitor/internal/collector"
	"sql-server-monitor/internal/config"
	"sql-server-monitor/internal/severity"
	"sql-server-monitor/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
		os.Exit(1)
	}

	basePath, err := storage.EnsureDirectoryStructure(cfg.LogDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create directory structure: %v\n", err)
		os.Exit(1)
	}

	setupLogging(basePath)

	slog.Info("SQL Server Monitoring Application Started")
	slog.Info(fmt.Sprintf("Server: %s, Database: %s", cfg.SQLServer, cfg.SQLDatabase))
	slog.Info(fmt.Sprintf("Log directory: %s", cfg.LogDir))
	slog.Info("Running single collection cycle (scheduled externally)")

	db, err := connectToSQL(cfg)
	if err != nil {
		slog.Error(fmt.Sprintf("Initial connection test failed: %v", err))
		os.Exit(1)
	}
	db.Close()
	slog.Info("Initial connection test successful")

	if err := performCollectionCycle(cfg, basePath); err != nil {
		slog.Error(fmt.Sprintf("Collection cycle failed: %v", err))
		os.Exit(1)
	}

	slog.Info("Application completed successfully")
}

func performCollectionCycle(cfg *config.Config, basePath string) error {
	slog.Info("============================================================")
	slog.Info("Starting collection cycle")

	db, err := connectToSQL(cfg)
	if err != nil {
		slog.Error("Skipping collection cycle - no connection")
		return err
	}

	var procStats, activeQueries, serverResources []map[string]interface{}

	procStats, err = collector.CollectProcStats(db, cfg.SQLDatabase)
	if err != nil {
		slog.Error(fmt.Sprintf("Proc stats query failed: %v", err))
	} else {
		slog.Info(fmt.Sprintf("Proc stats query executed successfully - %d rows", len(procStats)))
	}

	activeQueries, err = collector.CollectActiveQueries(db)
	if err != nil {
		slog.Error(fmt.Sprintf("Active queries query failed: %v", err))
	} else {
		if len(activeQueries) == 0 {
			slog.Info("No active queries detected")
		} else {
			slog.Info(fmt.Sprintf("Active queries query executed successfully - %d rows", len(activeQueries)))
		}
	}

	serverResources, err = collector.CollectServerResources(db)
	if err != nil {
		slog.Error(fmt.Sprintf("Server resources query failed: %v", err))
	} else {
		slog.Info(fmt.Sprintf("Server resources query executed successfully - %d rows", len(serverResources)))
	}

	db.Close()

	result := severity.Calculate(procStats, activeQueries, serverResources)

	slog.Info(fmt.Sprintf("Severity: %s", result.Severity))
	if len(result.Reasons) > 0 {
		for _, reason := range result.Reasons {
			slog.Info(fmt.Sprintf("  - %s", reason))
		}
	}

	if err := storage.AppendToDailyCSV(procStats, "proc_stats_daily.csv", result.Severity, basePath); err != nil {
		slog.Error(fmt.Sprintf("Failed to write proc stats: %v", err))
	}

	if err := storage.AppendToDailyCSV(serverResources, "server_resources_daily.csv", result.Severity, basePath); err != nil {
		slog.Error(fmt.Sprintf("Failed to write server resources: %v", err))
	}

	if err := storage.WriteSnapshotCSV(activeQueries, "active_queries", basePath, result.Severity); err != nil {
		slog.Error(fmt.Sprintf("Failed to write active queries snapshot: %v", err))
	}

	if result.Severity == "YELLOW" || result.Severity == "RED" {
		slog.Info("Triggering detailed capture")

		db, err := connectToSQL(cfg)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to reconnect for detailed capture: %v", err))
		} else {
			blockingChain, err := collector.CollectBlockingChain(db)
			if err != nil {
				slog.Error(fmt.Sprintf("Blocking chain query failed: %v", err))
			} else {
				slog.Info(fmt.Sprintf("Blocking chain query executed successfully - %d rows", len(blockingChain)))
			}

			waitStats, err := collector.CollectWaitStats(db)
			if err != nil {
				slog.Error(fmt.Sprintf("Wait stats query failed: %v", err))
			} else {
				slog.Info(fmt.Sprintf("Wait stats query executed successfully - %d rows", len(waitStats)))
			}

			db.Close()

			triggerReason := strings.Join(result.Reasons, "; ")

			if err := storage.WriteDetailedCapture(blockingChain, "blocking", triggerReason, basePath); err != nil {
				slog.Error(fmt.Sprintf("Failed to write blocking capture: %v", err))
			}

			if err := storage.WriteDetailedCapture(waitStats, "waits", triggerReason, basePath); err != nil {
				slog.Error(fmt.Sprintf("Failed to write wait stats capture: %v", err))
			}
		}
	}

	now := time.Now()
	if now.Hour() == 0 && now.Minute() < 5 {
		slog.Info("Running daily cleanup")

		if err := storage.CleanupOldLogs(cfg.LogDir, cfg.RetentionDays); err != nil {
			slog.Error(fmt.Sprintf("Cleanup failed: %v", err))
		}

		s3Client := storage.NewS3Client(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Region, cfg.S3Bucket)
		if err := s3Client.CleanupOldFolders(cfg.S3RetentionDays); err != nil {
			slog.Error(fmt.Sprintf("S3 cleanup failed: %v", err))
		}
	}

	slog.Info("Starting S3 sync")
	s3Client := storage.NewS3Client(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Region, cfg.S3Bucket)
	if err := s3Client.SyncTodayFolder(cfg.LogDir); err != nil {
		slog.Error(fmt.Sprintf("S3 sync failed: %v", err))
	}

	slog.Info("Collection cycle completed")
	return nil
}

func connectToSQL(cfg *config.Config) (*sql.DB, error) {
	connString := fmt.Sprintf("server=%s;user id=%s;password=%s;database=%s;connection timeout=30",
		cfg.SQLServer, cfg.SQLUser, cfg.SQLPassword, cfg.SQLDatabase)

	db, err := sql.Open("sqlserver", connString)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	slog.Info("SQL connection established")
	return db, nil
}

func setupLogging(basePath string) {
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(basePath, fmt.Sprintf("app_%s.log", today))

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		os.Exit(1)
	}

	handler := slog.NewTextHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)

	slog.Info(fmt.Sprintf("Logging initialized for %s", today))
}
