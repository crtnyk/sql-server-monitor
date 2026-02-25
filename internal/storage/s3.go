package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	MultipartThreshold = 5 * 1024 * 1024
	MaxWorkers         = 5
)

type S3Client struct {
	client *s3.Client
	bucket string
}

func NewS3Client(endpoint, accessKey, secretKey, region, bucket string) *S3Client {
	cfg := aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		BaseEndpoint: aws.String(endpoint),
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &S3Client{
		client: client,
		bucket: bucket,
	}
}

func (s *S3Client) SyncTodayFolder(baseDir string) error {
	today := time.Now().Format("2006-01-02")
	folderPath := filepath.Join(baseDir, today)

	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		slog.Warn(fmt.Sprintf("Folder %s does not exist, skipping sync", folderPath))
		return nil
	}

	localFiles, err := getLocalFiles(folderPath)
	if err != nil {
		return err
	}

	if len(localFiles) == 0 {
		slog.Info("No files to sync")
		return nil
	}

	prefix := fmt.Sprintf("%s/", today)
	s3Files, err := s.listS3Files(prefix)
	if err != nil {
		return err
	}

	var missingFiles []string
	for relativePath := range localFiles {
		s3Key := strings.ReplaceAll(relativePath, "\\", "/")
		if !s3Files[s3Key] {
			missingFiles = append(missingFiles, relativePath)
		}
	}

	if len(missingFiles) == 0 {
		slog.Info("All files already synced to S3")
		return nil
	}

	uploadedCount, failedCount := s.uploadFilesParallel(localFiles, missingFiles)

	slog.Info(fmt.Sprintf("S3 sync completed: %d uploaded, %d failed", uploadedCount, failedCount))
	return nil
}

func (s *S3Client) CleanupOldFolders(retentionDays int) error {
	ctx := context.Background()
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Delimiter: aws.String("/"),
	}

	paginator := s3.NewListObjectsV2Paginator(s.client, input)

	var foldersToDelete []string

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, prefix := range page.CommonPrefixes {
			folderName := strings.TrimSuffix(*prefix.Prefix, "/")

			if len(folderName) == 10 {
				folderDate, err := time.Parse("2006-01-02", folderName)
				if err == nil && folderDate.Before(cutoffDate) {
					foldersToDelete = append(foldersToDelete, *prefix.Prefix)
				}
			}
		}
	}

	deletedCount := 0
	for _, prefix := range foldersToDelete {
		count, err := s.deletePrefix(prefix)
		if err != nil {
			slog.Warn(fmt.Sprintf("Failed to delete S3 folder %s: %v", prefix, err))
		} else {
			deletedCount += count
			slog.Info(fmt.Sprintf("Deleted S3 folder: %s", strings.TrimSuffix(prefix, "/")))
		}
	}

	slog.Info(fmt.Sprintf("S3 cleanup completed: %d objects deleted", deletedCount))
	return nil
}

func (s *S3Client) uploadFilesParallel(localFiles map[string]string, filesToUpload []string) (int, int) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, MaxWorkers)
	results := make(chan bool, len(filesToUpload))

	for _, relativePath := range filesToUpload {
		wg.Add(1)
		go func(relPath string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			key := strings.ReplaceAll(relPath, "\\", "/")
			filePath := localFiles[relPath]

			success := s.uploadFile(key, filePath)
			results <- success
		}(relativePath)
	}

	wg.Wait()
	close(results)

	uploadedCount := 0
	failedCount := 0
	for success := range results {
		if success {
			uploadedCount++
		} else {
			failedCount++
		}
	}

	return uploadedCount, failedCount
}

func (s *S3Client) uploadFile(key, filePath string) bool {
	ctx := context.Background()

	file, err := os.Open(filePath)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to open file %s: %v", filePath, err))
		return false
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to stat file %s: %v", filePath, err))
		return false
	}

	if stat.Size() > MultipartThreshold {
		uploader := manager.NewUploader(s.client, func(u *manager.Uploader) {
			u.PartSize = MultipartThreshold
		})

		_, err = uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
			Body:   file,
		})
	} else {
		_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
			Body:   file,
		})
	}

	if err != nil {
		slog.Error(fmt.Sprintf("Failed to upload %s to S3: %v", filePath, err))
		return false
	}

	return true
}

func (s *S3Client) listS3Files(prefix string) (map[string]bool, error) {
	ctx := context.Background()
	files := make(map[string]bool)

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	}

	paginator := s3.NewListObjectsV2Paginator(s.client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, obj := range page.Contents {
			key := *obj.Key
			if !strings.HasSuffix(key, "/") {
				relativeKey := strings.TrimPrefix(key, prefix)
				if relativeKey != "" {
					files[key] = true
				}
			}
		}
	}

	slog.Info(fmt.Sprintf("Found %d files in S3 under %s", len(files), prefix))
	return files, nil
}

func (s *S3Client) deletePrefix(prefix string) (int, error) {
	ctx := context.Background()

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	}

	paginator := s3.NewListObjectsV2Paginator(s.client, input)

	var objectsToDelete []types.ObjectIdentifier
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return 0, err
		}

		for _, obj := range page.Contents {
			objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}
	}

	if len(objectsToDelete) == 0 {
		return 0, nil
	}

	deletedCount := 0
	batchSize := 1000

	for i := 0; i < len(objectsToDelete); i += batchSize {
		end := i + batchSize
		if end > len(objectsToDelete) {
			end = len(objectsToDelete)
		}

		batch := objectsToDelete[i:end]

		deleteInput := &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &types.Delete{
				Objects: batch,
			},
		}

		output, err := s.client.DeleteObjects(ctx, deleteInput)
		if err != nil {
			return deletedCount, err
		}

		deletedCount += len(output.Deleted)
	}

	return deletedCount, nil
}

func getLocalFiles(folderPath string) (map[string]string, error) {
	files := make(map[string]string)

	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			relativePath, err := filepath.Rel(filepath.Dir(folderPath), path)
			if err != nil {
				return err
			}
			files[relativePath] = path
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	slog.Info(fmt.Sprintf("Found %d files in %s", len(files), folderPath))
	return files, nil
}

func readFile(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(file)
}
