// 结构化日志引擎
// 基于 Zap + Lumberjack，统一 JSON 输出 + 文件自动轮转
// 标准字段常量规范全项目日志 key 命名
// 调用方统一使用 util.Info / Debug / Warn / Error / Fatal

package util

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// ============================================================
// 标准字段常量（全项目统一 key 命名）
// ============================================================

const (
	FieldModule   = "module"   // 模块标识：syncer / service / handler / core
	FieldAction   = "action"   // 操作：sync / query / download / upsert
	FieldElapsed  = "elapsed"  // 耗时（秒，float64）
	FieldSoftware = "software" // 软件 ID：vscode / obsidian
	FieldError    = "error"    // 错误信息
	FieldStatus   = "status"   // ok / degraded / failed
)

// ============================================================
// 便捷字段构造器
// ============================================================

// Module 模块字段
func Module(name string) zap.Field { return zap.String(FieldModule, name) }

// Elapsed 耗时字段（秒）
func Elapsed(d time.Duration) zap.Field { return zap.Float64(FieldElapsed, d.Seconds()) }

// Software 软件 ID 字段
func Software(id string) zap.Field { return zap.String(FieldSoftware, id) }

// Action 操作字段
func Action(name string) zap.Field { return zap.String(FieldAction, name) }

// Status 状态字段
func Status(s string) zap.Field { return zap.String(FieldStatus, s) }

// ============================================================
// 日志配置
// ============================================================

// LogConfig 日志配置（config.yaml 注入）
type LogConfig struct {
	Level string    `yaml:"level"` // debug / info / warn / error
	File  LogFileConfig `yaml:"file"`
}

// LogFileConfig 文件存储配置
type LogFileConfig struct {
	Enabled    bool   `yaml:"enabled"`      // 启用文件存储
	Path       string `yaml:"path"`         // "logs/app.log"
	MaxSizeMB  int    `yaml:"max_size_mb"`  // 单文件上限 MB
	MaxBackups int    `yaml:"max_backups"`  // 保留归档数
	MaxAgeDays int    `yaml:"max_age_days"` // 最大保留天数
	Compress   bool   `yaml:"compress"`     // gzip 归档
}

// levelMap 级别字符串映射
var levelMap = map[string]zapcore.Level{
	"debug": zapcore.DebugLevel,
	"info":  zapcore.InfoLevel,
	"warn":  zapcore.WarnLevel,
	"error": zapcore.ErrorLevel,
}

var logger *zap.Logger

// ============================================================
// 初始化
// ============================================================

// InitLogger 初始化日志引擎
// debug=true 时使用 debug 级别 + stdout 输出（向后兼容）
func InitLogger(debug bool) error {
	cfg := &LogConfig{Level: "info"}
	if debug {
		cfg.Level = "debug"
	}
	cfg.File.Enabled = !debug // 生产模式启用文件
	cfg.File.Path = "logs/app.log"
	cfg.File.MaxSizeMB = 100
	cfg.File.MaxBackups = 10
	cfg.File.MaxAgeDays = 30
	cfg.File.Compress = true
	return InitLoggerWithConfig(cfg)
}

// InitLoggerWithConfig 根据配置初始化日志
func InitLoggerWithConfig(cfg *LogConfig) error {
	// 级别
	level := zapcore.InfoLevel
	if lvl, ok := levelMap[cfg.Level]; ok {
		level = lvl
	}

	// JSON 编码器配置
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "time"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeDuration = zapcore.SecondsDurationEncoder
	encoderCfg.EncodeCaller = zapcore.ShortCallerEncoder
	encoderCfg.MessageKey = "msg"
	encoderCfg.LevelKey = "level"

	// 输出目标
	var writers []zapcore.WriteSyncer
	writers = append(writers, zapcore.AddSync(os.Stdout))

	if cfg.File.Enabled && cfg.File.Path != "" {
		lj := &lumberjack.Logger{
			Filename:   cfg.File.Path,
			MaxSize:    cfg.File.MaxSizeMB,
			MaxBackups: cfg.File.MaxBackups,
			MaxAge:     cfg.File.MaxAgeDays,
			Compress:   cfg.File.Compress,
		}
		if lj.MaxSize == 0 { lj.MaxSize = 100 }
		if lj.MaxBackups == 0 { lj.MaxBackups = 10 }
		if lj.MaxAge == 0 { lj.MaxAge = 30 }
		writers = append(writers, zapcore.AddSync(lj))
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.NewMultiWriteSyncer(writers...),
		level,
	)

	logger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	zap.ReplaceGlobals(logger)
	return nil
}

// ============================================================
// 获取 / 同步 / 关闭
// ============================================================

// GetLogger 获取日志实例
func GetLogger() *zap.Logger {
	if logger == nil {
		_ = InitLogger(true)
	}
	return logger
}

// SyncLogger 同步日志缓冲区
func SyncLogger() {
	if logger != nil {
		_ = logger.Sync()
	}
}

// ============================================================
// 日志级别方法（统一入口）
// ============================================================

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

// Fatal 致命日志（调用 os.Exit(1)）
func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
}
