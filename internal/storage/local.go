package storage

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func EnsureDirectoryStructure(baseDir string) (string, error) {
	today := time.Now().Format("2006-01-02")
	basePath := filepath.Join(baseDir, today)

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Join(basePath, "active_queries"), 0755); err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Join(basePath, "detailed_captures"), 0755); err != nil {
		return "", err
	}

	return basePath, nil
}

func AppendToDailyCSV(data []map[string]interface{}, filename, severity, basePath string) error {
	if data == nil || len(data) == 0 {
		return nil
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	for i := range data {
		data[i]["collection_timestamp"] = timestamp
		data[i]["severity"] = severity
	}

	filePath := filepath.Join(basePath, filename)

	fileExists := false
	if _, err := os.Stat(filePath); err == nil {
		fileExists = true
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	keys := getSortedKeys(data[0])

	if !fileExists {
		if err := writer.Write(keys); err != nil {
			return err
		}
	}

	for _, row := range data {
		record := make([]string, len(keys))
		for i, key := range keys {
			record[i] = formatValue(row[key])
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	slog.Info(fmt.Sprintf("Appended %d rows to %s", len(data), filename))
	return nil
}

func WriteSnapshotCSV(data []map[string]interface{}, subfolder, basePath, severity string) error {
	if data == nil {
		return nil
	}

	timestamp := time.Now().Format("20060102_1504")
	severitySuffix := ""
	if severity != "GREEN" {
		severitySuffix = fmt.Sprintf("_%s", severity)
	}
	filename := fmt.Sprintf("%s%s.csv", timestamp, severitySuffix)

	filePath := filepath.Join(basePath, subfolder, filename)

	timestampStr := time.Now().Format("2006-01-02 15:04:05")
	for i := range data {
		data[i]["collection_timestamp"] = timestampStr
		data[i]["severity"] = severity
	}

	return writeCSV(filePath, data, filename)
}

func WriteDetailedCapture(data []map[string]interface{}, captureType, triggerReason, basePath string) error {
	if data == nil || len(data) == 0 {
		slog.Info(fmt.Sprintf("No data for %s detailed capture", captureType))
		return nil
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.csv", captureType, timestamp)
	filePath := filepath.Join(basePath, "detailed_captures", filename)

	captureTimestamp := time.Now().Format("2006-01-02 15:04:05")
	for i := range data {
		data[i]["trigger_reason"] = triggerReason
		data[i]["capture_timestamp"] = captureTimestamp
	}

	if err := writeCSV(filePath, data, filename); err != nil {
		return err
	}

	slog.Info(fmt.Sprintf("Detailed capture saved: %s - Reason: %s", filename, triggerReason))
	return nil
}

func CleanupOldLogs(baseDir string, retentionDays int) error {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		folderName := entry.Name()
		if len(folderName) != 10 {
			continue
		}

		folderDate, err := time.Parse("2006-01-02", folderName)
		if err != nil {
			continue
		}

		if folderDate.Before(cutoffDate) {
			folderPath := filepath.Join(baseDir, folderName)
			if err := os.RemoveAll(folderPath); err != nil {
				slog.Warn(fmt.Sprintf("Failed to delete folder %s: %v", folderName, err))
			} else {
				slog.Info(fmt.Sprintf("Deleted old folder: %s", folderName))
			}
		}
	}

	return nil
}

func writeCSV(filePath string, data []map[string]interface{}, logName string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if len(data) == 0 {
		return nil
	}

	keys := getSortedKeys(data[0])

	if err := writer.Write(keys); err != nil {
		return err
	}

	for _, row := range data {
		record := make([]string, len(keys))
		for i, key := range keys {
			record[i] = formatValue(row[key])
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	slog.Info(fmt.Sprintf("Created snapshot: %s", logName))
	return nil
}

func getSortedKeys(row map[string]interface{}) []string {
	keys := make([]string, 0, len(row))
	for k := range row {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatValue(val interface{}) string {
	if val == nil {
		return ""
	}
	return fmt.Sprintf("%v", val)
}
