package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/remote-file-manager/server/pkg/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitLogger initializes the logger based on configuration
func InitLogger(cfg *config.LoggingConfig) (*zap.Logger, error) {
	// Parse log level
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	// Create encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// Determine output path
	var outputPath string
	if cfg.UseRotate && cfg.LogDir != "" {
		// Create log directory with date
		now := time.Now()
		dateDir := now.Format("2006-1-2")
		logDir := filepath.Join(cfg.LogDir, dateDir)

		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// Generate log filename with hour
		hour := now.Format("15-00")
		outputPath = filepath.Join(logDir, fmt.Sprintf("Golang-%s.log", hour))
	} else {
		outputPath = cfg.Output
	}

	// Create core
	var core zapcore.Core
	if outputPath == "stdout" {
		core = zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			level,
		)
	} else {
		// File output
		file, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		core = zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(file),
			level,
		)
	}

	// Create logger
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}
