package logger

import (
	"os"
	"path/filepath"

	"gocerery/internal/config"

	"github.com/zeromicro/go-zero/core/logx"
)

// InitLogger 初始化日志系统
func InitLogger(cfg *config.LogConfig) error {
	// 设置默认值
	if cfg.ServiceName == "" {
		cfg.ServiceName = "gocerery"
	}
	if cfg.Mode == "" {
		cfg.Mode = "file"
	}
	if cfg.Path == "" {
		cfg.Path = "logs"
	}
	if cfg.Level == "" {
		cfg.Level = "info"
	}
	if cfg.KeepDays == 0 {
		cfg.KeepDays = 7
	}
	if cfg.StackCooldownMillis == 0 {
		cfg.StackCooldownMillis = 100
	}

	// 创建日志目录
	if cfg.Mode == "file" {
		if err := os.MkdirAll(cfg.Path, 0755); err != nil {
			return err
		}
	}

	// 配置 logx
	logx.DisableStat()

	// 设置日志配置
	logConf := logx.LogConf{
		ServiceName:         cfg.ServiceName,
		Mode:                cfg.Mode,
		Path:                cfg.Path,
		Level:               cfg.Level,
		Compress:            cfg.Compress,
		KeepDays:            cfg.KeepDays,
		StackCooldownMillis: cfg.StackCooldownMillis,
	}

	// 应用配置
	if err := logx.SetUp(logConf); err != nil {
		return err
	}

	return nil
}

// GetLogFile 获取日志文件路径
func GetLogFile(cfg *config.LogConfig, logType string) string {
	if cfg.Mode != "file" {
		return ""
	}

	fileName := cfg.ServiceName
	if logType != "" {
		fileName = cfg.ServiceName + "-" + logType
	}

	return filepath.Join(cfg.Path, fileName+".log")
}
