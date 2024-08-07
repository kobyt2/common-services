package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"github.com/spf13/viper"
)

var (
	sugaredLogger *zap.SugaredLogger
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
	CustomLevelEncoder bool   `mapstructure:"custom-level-encoder" json:"custom-level-encoder" yaml:"custom-level-encoder"`
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
		EncodeLevel:    c.LevelEncoder(),
		EncodeTime:     c.TimeEncoder(),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
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

// Init initializes the logger by loading the configuration file
func Init(configFile string) error {
	viper.SetConfigFile(configFile)

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	var zapConfig ZapConfig
	if err := viper.UnmarshalKey("zap", &zapConfig); err != nil {
		return fmt.Errorf("error unmarshalling config to struct: %v", err)
	}

	return initLogger(&zapConfig)
}

// initLogger initializes the logger based on the given configuration
func initLogger(cfg *ZapConfig) error {
	if ok, _ := pathExists(cfg.Director); !ok {
		fmt.Printf("create %v directory\n", cfg.Director)
		err := os.Mkdir(cfg.Director, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %v", cfg.Director, err)
		}
	}

	cores, err := setupCores(cfg)
	if err != nil {
		return err
	}

	logger := zap.New(zapcore.NewTee(cores...), zap.AddCaller())
	sugaredLogger = logger.Sugar()
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
		writer, err := getLogWriter(cfg, level.String()+".log")
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
func getLogWriter(cfg *ZapConfig, filename string) (zapcore.WriteSyncer, error) {
	filepath := filepath.Join(cfg.Director, filename)
	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return zapcore.AddSync(file), nil
}

// pathExists checks if a path exists
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Info logs an info message
func Info(args ...interface{}) {
	sugaredLogger.Info(args...)
}

// Infof logs an info message with formatting
func Infof(template string, args ...interface{}) {
	sugaredLogger.Infof(template, args...)
}

// Error logs an error message
func Error(args ...interface{}) {
	sugaredLogger.Error(args...)
}

// Errorf logs an error message with formatting
func Errorf(template string, args ...interface{}) {
	sugaredLogger.Errorf(template, args...)
}

// Debug logs a debug message
func Debug(args ...interface{}) {
	sugaredLogger.Debug(args...)
}

// Debugf logs a debug message with formatting
func Debugf(template string, args ...interface{}) {
	sugaredLogger.Debugf(template, args...)
}

// Warn logs a warn message
func Warn(args ...interface{}) {
	sugaredLogger.Warn(args...)
}

// Warnf logs a warn message with formatting
func Warnf(template string, args ...interface{}) {
	sugaredLogger.Warnf(template, args...)
}

// Fatal logs a fatal message and exits
func Fatal(args ...interface{}) {
	sugaredLogger.Fatal(args...)
}

// Fatalf logs a fatal message with formatting and exits
func Fatalf(template string, args ...interface{}) {
	sugaredLogger.Fatalf(template, args...)
}
