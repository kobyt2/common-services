package logger

import (
	"fmt"
	"github.com/spf13/viper"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"path/filepath"
	"time"
)

// Define global variables
var (
	Logger        *zap.Logger
	SugaredLogger *zap.SugaredLogger
)

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

// InitLogger initializes the logger based on the configuration file path
func InitLogger(configFile string) error {
	viper.SetConfigFile(configFile)

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	var zapConfig ZapConfig
	if err := viper.UnmarshalKey("zap", &zapConfig); err != nil {
		return fmt.Errorf("error unmarshalling config to struct: %v", err)
	}

	fmt.Printf("Loaded config: %+v\n", zapConfig) // 输出加载的配置

	if ok, _ := PathExists(zapConfig.Director); !ok {
		fmt.Printf("create %v directory\n", zapConfig.Director)
		if err := os.Mkdir(zapConfig.Director, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", zapConfig.Director, err)
		}
	}

	cores, err := setupCores(&zapConfig)
	if err != nil {
		return err
	}

	Logger = zap.New(zapcore.NewTee(cores...), zap.AddCaller())
	SugaredLogger = Logger.Sugar()
	fmt.Println("Logger initialized successfully") // 确认初始化成功
	return nil
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
