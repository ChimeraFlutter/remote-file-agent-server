package objects

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// DownloadHandler handles file downloads with Range support
type DownloadHandler struct {
	storage *Storage
	logger  *zap.Logger
}

// NewDownloadHandler creates a new download handler
func NewDownloadHandler(storage *Storage, logger *zap.Logger) *DownloadHandler {
	return &DownloadHandler{
		storage: storage,
		logger:  logger,
	}
}

// ServeFile serves a file with support for HTTP Range requests (resumable downloads)
func (dh *DownloadHandler) ServeFile(w http.ResponseWriter, r *http.Request, obj *UploadedObject) error {
	// Get file path
	filePath := dh.storage.GetStoragePath(obj.ObjectID)

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		dh.logger.Error("Failed to open file", zap.String("path", filePath), zap.Error(err))
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		dh.logger.Error("Failed to stat file", zap.Error(err))
		return fmt.Errorf("failed to stat file: %w", err)
	}

	fileSize := fileInfo.Size()

	// Set common headers
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", obj.FileName))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Accept-Ranges", "bytes")

	// Check for Range header
	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		// No range request, serve entire file
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileSize))
		w.WriteHeader(http.StatusOK)

		if _, err := io.Copy(w, file); err != nil {
			dh.logger.Error("Failed to stream file", zap.Error(err))
			return fmt.Errorf("failed to stream file: %w", err)
		}

		dh.logger.Debug("File downloaded",
			zap.String("object_id", obj.ObjectID),
			zap.String("file_name", obj.FileName),
			zap.Int64("size", fileSize))

		return nil
	}

	// Parse Range header
	ranges, err := parseRange(rangeHeader, fileSize)
	if err != nil {
		http.Error(w, "Invalid Range header", http.StatusRequestedRangeNotSatisfiable)
		return fmt.Errorf("invalid range header: %w", err)
	}

	// Only support single range for now
	if len(ranges) != 1 {
		http.Error(w, "Multiple ranges not supported", http.StatusRequestedRangeNotSatisfiable)
		return fmt.Errorf("multiple ranges not supported")
	}

	start := ranges[0].start
	end := ranges[0].end

	// Seek to start position
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		dh.logger.Error("Failed to seek file", zap.Error(err))
		return fmt.Errorf("failed to seek file: %w", err)
	}

	// Set headers for partial content
	contentLength := end - start + 1
	w.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
	w.WriteHeader(http.StatusPartialContent)

	// Stream the requested range
	if _, err := io.CopyN(w, file, contentLength); err != nil && err != io.EOF {
		dh.logger.Error("Failed to stream file range", zap.Error(err))
		return fmt.Errorf("failed to stream file range: %w", err)
	}

	dh.logger.Debug("File range downloaded",
		zap.String("object_id", obj.ObjectID),
		zap.String("file_name", obj.FileName),
		zap.Int64("start", start),
		zap.Int64("end", end),
		zap.Int64("content_length", contentLength))

	return nil
}

// byteRange represents a byte range
type byteRange struct {
	start int64
	end   int64
}

// parseRange parses HTTP Range header
// Format: "bytes=start-end" or "bytes=start-" or "bytes=-suffix"
func parseRange(rangeHeader string, fileSize int64) ([]byteRange, error) {
	// Remove "bytes=" prefix
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return nil, fmt.Errorf("invalid range header format")
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")

	// Split by comma for multiple ranges
	rangeParts := strings.Split(rangeSpec, ",")

	var ranges []byteRange

	for _, part := range rangeParts {
		part = strings.TrimSpace(part)

		// Parse "start-end"
		if strings.Contains(part, "-") {
			parts := strings.Split(part, "-")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid range format")
			}

			startStr := strings.TrimSpace(parts[0])
			endStr := strings.TrimSpace(parts[1])

			var start, end int64

			if startStr == "" {
				// Suffix range: "-500" means last 500 bytes
				suffix, err := strconv.ParseInt(endStr, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("invalid suffix range: %w", err)
				}
				start = fileSize - suffix
				if start < 0 {
					start = 0
				}
				end = fileSize - 1
			} else if endStr == "" {
				// Open-ended range: "500-" means from byte 500 to end
				var err error
				start, err = strconv.ParseInt(startStr, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("invalid start position: %w", err)
				}
				end = fileSize - 1
			} else {
				// Full range: "500-999"
				var err error
				start, err = strconv.ParseInt(startStr, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("invalid start position: %w", err)
				}
				end, err = strconv.ParseInt(endStr, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("invalid end position: %w", err)
				}
			}

			// Validate range
			if start < 0 || end >= fileSize || start > end {
				return nil, fmt.Errorf("range out of bounds")
			}

			ranges = append(ranges, byteRange{start: start, end: end})
		} else {
			return nil, fmt.Errorf("invalid range format")
		}
	}

	if len(ranges) == 0 {
		return nil, fmt.Errorf("no valid ranges")
	}

	return ranges, nil
}
