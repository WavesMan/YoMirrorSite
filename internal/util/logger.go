package util

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

// InitLogger 初始化日志
func InitLogger(debug bool) error {
	var config zap.Config
	if debug {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
		config.OutputPaths = []string{"stdout", "logs/app.log"}
	}

	var err error
	logger, err = config.Build()
	if err != nil {
		return err
	}

	zap.ReplaceGlobals(logger)
	return nil
}

// GetLogger 获取日志实例
func GetLogger() *zap.Logger {
	if logger == nil {
		// 如果未初始化，创建一个默认的日志实例
		_ = InitLogger(true)
	}
	return logger
}

// SyncLogger 同步日志
func SyncLogger() {
	if logger != nil {
		_ = logger.Sync()
	}
}

// Debug 调试日志
func Debug(msg string, fields ...zap.Field) {
	GetLogger().Debug(msg, fields...)
}

// Info 信息日志
func Info(msg string, fields ...zap.Field) {
	GetLogger().Info(msg, fields...)
}

// Warn 警告日志
func Warn(msg string, fields ...zap.Field) {
	GetLogger().Warn(msg, fields...)
}

// Error 错误日志
func Error(msg string, fields ...zap.Field) {
	GetLogger().Error(msg, fields...)
}

// DPanic 致命错误日志（开发环境panic，生产环境不panic）
func DPanic(msg string, fields ...zap.Field) {
	GetLogger().DPanic(msg, fields...)
}

// Panic 恐慌日志（会panic）
func Panic(msg string, fields ...zap.Field) {
	GetLogger().Panic(msg, fields...)
}

// Fatal 致命日志（会exit）
func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
}
