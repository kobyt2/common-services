package logger

import (
	"context"
	"fmt"
	"github.com/spf13/viper"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"path/filepath"
	"time"
	"gorm.io/gorm/logger"
)
// GormLogger 定义一个 GORM 自定义日志结构体
type GormLogger struct {
	zapLogger *zap.Logger
	config    logger.Config
}

// Define global variables
var (
	Logger        *zap.Logger
	SugaredLogger *zap.SugaredLogger
)

// NewGormLogger 创建一个新的 GormLogger 实例
func NewGormLogger(zapLogger *zap.Logger) GormLogger {
    if zapLogger == nil {
        panic("zapLogger is nil")
    }
    return GormLogger{
        zapLogger: zapLogger,
        config: logger.Config{
            SlowThreshold:             200 * time.Millisecond, // 慢查询的阈值
            LogLevel:                  logger.Warn,            // 默认日志级别
            IgnoreRecordNotFoundError: false,                  // 忽略没有找到记录的错误
            Colorful:                  false,                  // 禁用彩色打印
        },
    }
}

// LogMode 设置日志级别
func (l GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newlogger := l
	newlogger.config.LogLevel = level
	return newlogger
}

// Info 实现 gorm.Logger 的 Info 方法
func (l GormLogger) Info(ctx context.Context, s string, i ...interface{}) {
	if l.config.LogLevel >= logger.Info {
		l.zapLogger.Sugar().Infof(s, i...)
	}
}

// Warn 实现 gorm.Logger 的 Warn 方法
func (l GormLogger) Warn(ctx context.Context, s string, i ...interface{}) {
	if l.config.LogLevel >= logger.Warn {
		l.zapLogger.Sugar().Warnf(s, i...)
	}
}

// Error 实现 gorm.Logger 的 Error 方法
func (l GormLogger) Error(ctx context.Context, s string, i ...interface{}) {
	if l.config.LogLevel >= logger.Error {
		l.zapLogger.Sugar().Errorf(s, i...)
	}
}

// Trace 实现 gorm.Logger 的 Trace 方法
func (l GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.config.LogLevel > 0 {
		elapsed := time.Since(begin)
		sql, rows := fc()
		if err != nil {
			l.zapLogger.Sugar().Errorf("[%.3fms] [rows:%v] %s", float64(elapsed.Nanoseconds())/1e6, rows, sql)
		} else if elapsed > l.config.SlowThreshold && l.config.SlowThreshold != 0 {
			l.zapLogger.Sugar().Warnf("[%.3fms] [rows:%v] %s", float64(elapsed.Nanoseconds())/1e6, rows, sql)
		} else {
			l.zapLogger.Sugar().Debugf("[%.3fms] [rows:%v] %s", float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	}
}

// ZapConfig holds the configuration for the logger
type ZapConfig struct {
	Level              string `mapstructure:"level" json:"level" yaml:"level"`
	Prefix             string `mapstructure:"prefix" json:"prefix" yaml:"prefix"`
	Format             string `mapstructure:"format" json:"format" yaml:"format"`
	Director           string `mapstructure:"director" json:"director" yaml:"director"`
	EncodeLevel        string `mapstructure:"encode-level" json:"encode-level" yaml:"encode-level"`
	StacktraceKey      string `mapstructure:"stacktrace-key" json:"stacktrace-key" yaml:"stacktrace-key"`
	ShowLine           bool   `mapstructure:"show-line" json:"show-line" yaml:"show-line"`
	LogInConsole       bool   `mapstructure:"log-in-console" json:"log-in-console" yaml:"log-in-console"`
	RetentionDay       int    `mapstructure:"retention-day" json:"retention-day" yaml:"retention-day"`
	CustomLevelEncoder bool   `mapstructure:"custom-level-encoder" json:"custom-level-encoder"` // New field
}


// EncoderConfig returns the encoder configuration based on the ZapConfig
func (c *ZapConfig) EncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  c.StacktraceKey,
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    c.LevelEncoder(), // Use custom LevelEncoder
		EncodeTime:     c.TimeEncoder(),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   c.CallerEncoder(),
		EncodeName:     zapcore.FullNameEncoder,
	}
}

// LevelEncoder returns the level encoder based on the ZapConfig
func (c *ZapConfig) LevelEncoder() zapcore.LevelEncoder {
	if c.CustomLevelEncoder {
		return func(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(fmt.Sprintf("[%s]", level.CapitalString()))
		}
	}

	switch c.EncodeLevel {
	case "lowercase":
		return zapcore.LowercaseLevelEncoder
	case "capital":
		return zapcore.CapitalLevelEncoder
	case "lowercaseColor":
		return zapcore.LowercaseColorLevelEncoder
	case "capitalColor":
		return zapcore.CapitalColorLevelEncoder
	default:
		return zapcore.CapitalLevelEncoder
	}
}

// TimeEncoder returns the time encoder based on the ZapConfig
func (c *ZapConfig) TimeEncoder() zapcore.TimeEncoder {
	return func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}
}

// CallerEncoder returns the caller encoder based on the ZapConfig
func (c *ZapConfig) CallerEncoder() zapcore.CallerEncoder {
	return func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(fmt.Sprintf("%s", caller.TrimmedPath()))
	}
}

func InitLogger(configFile string) error {
	// 设置配置文件路径
	viper.SetConfigFile(configFile)

	// 尝试读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Config file not found, using default values.")
		// 使用默认值
		defaultConfig := getDefaultConfig()
		cores, err := setupCores(&defaultConfig)
		if err != nil {
			return fmt.Errorf("failed to set up cores with default config: %v", err)
		}

		Logger = zap.New(zapcore.NewTee(cores...), zap.AddCaller())
		SugaredLogger = Logger.Sugar()
		fmt.Println("Logger initialized successfully with default values")
		return nil
	}

	// 读取配置文件成功，解码配置
	var zapConfig ZapConfig
	if err := viper.UnmarshalKey("zap", &zapConfig); err != nil {
		return fmt.Errorf("error unmarshalling config to struct: %v", err)
	}

	fmt.Printf("Loaded config: %+v\n", zapConfig)

	// 确保日志目录存在
	if ok, _ := PathExists(zapConfig.Director); !ok {
		fmt.Printf("Creating %v directory\n", zapConfig.Director)
		if err := os.Mkdir(zapConfig.Director, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", zapConfig.Director, err)
		}
	}

	// 设置日志核心
	cores, err := setupCores(&zapConfig)
	if err != nil {
		return fmt.Errorf("failed to set up cores with provided config: %v", err)
	}

	// 初始化 Logger
	//Logger = zap.New(zapcore.NewTee(cores...), zap.AddCaller())
	//zap.AddCallerSkip(1) 会让 zap 在记录 caller 信息时跳过一层栈帧，从而显示出你业务代码中调用 logger.Debug() 或其他日志函数的正确位置
	Logger = zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddCallerSkip(1))
	SugaredLogger = Logger.Sugar()

	fmt.Println("Logger initialized successfully")
	return nil
}


// getDefaultConfig returns a ZapConfig with default values
func getDefaultConfig() ZapConfig {
	return ZapConfig{
		Level:        "info",
		Prefix:       "[LOGGER]",
		Format:       "json",
		Director:     "./logs",
		EncodeLevel:  "capital",
		StacktraceKey: "stacktrace",
		ShowLine:     true,
		LogInConsole: true,
		RetentionDay:  7,
	}
}


// setupCores sets up the cores for different log levels
func setupCores(cfg *ZapConfig) ([]zapcore.Core, error) {
	levels := []zapcore.Level{zapcore.DebugLevel, zapcore.InfoLevel, zapcore.WarnLevel, zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel}
	cores := make([]zapcore.Core, 0, len(levels))

	encoderConfig := cfg.EncoderConfig()
	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}
	for _, level := range levels {
		writer, err := getLogWriter(cfg, level.String())
		if err != nil {
			return nil, fmt.Errorf("failed to create log file for level %s: %v", level.String(), err)
		}
		core := zapcore.NewCore(encoder, writer, zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl == level
		}))
		cores = append(cores, core)
	}
	return cores, nil
}

// getLogWriter creates a WriteSyncer for the given file
func getLogWriter(cfg *ZapConfig, level string) (zapcore.WriteSyncer, error) {
	timestamp := time.Now().Format("2006010215")
	filepath := filepath.Join(cfg.Director, fmt.Sprintf("%s_%s.log", level, timestamp))
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filepath,
		MaxSize:    1, // 每个日志文件最大 1 MB
		MaxBackups: 24, // 最多保存 24 个备份文件
		MaxAge:     cfg.RetentionDay, // 最多保存 cfg.RetentionDay 天的日志文件
		Compress:   true, // 压缩旧日志文件
	}
	return zapcore.AddSync(lumberJackLogger), nil
}


// PathExists checks if a path exists
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func Debug(args ...interface{}) {
	SugaredLogger.Debug(args...)
}

func Debugf(template string, args ...interface{}) {
	SugaredLogger.Debugf(template, args...)
}

func Info(args ...interface{}) {
	SugaredLogger.Info(args...)
}

func Infof(template string, args ...interface{}) {
	SugaredLogger.Infof(template, args...)
}

func Warn(args ...interface{}) {
	SugaredLogger.Warn(args...)
}

func Warnf(template string, args ...interface{}) {
	SugaredLogger.Warnf(template, args...)
}

func Error(args ...interface{}) {
	SugaredLogger.Error(args...)
}

func Errorf(template string, args ...interface{}) {
	SugaredLogger.Errorf(template, args...)
}

func DPanic(args ...interface{}) {
	SugaredLogger.DPanic(args...)
}

func DPanicf(template string, args ...interface{}) {
	SugaredLogger.DPanicf(template, args...)
}

func Panic(args ...interface{}) {
	SugaredLogger.Panic(args...)
}

func Panicf(template string, args ...interface{}) {
	SugaredLogger.Panicf(template, args...)
}

func Fatal(args ...interface{}) {
	SugaredLogger.Fatal(args...)
}

func Fatalf(template string, args ...interface{}) {
	SugaredLogger.Fatalf(template, args...)
}
