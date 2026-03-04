package logger

import (
	"os"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Log *zap.Logger

func InitLogger(logPath string) {
	consoleEncoder, fileEncoder := getEncoder()

	// 文件日志 writer
	lumberJackLogger := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    100, // MB
		MaxBackups: 7,
		MaxAge:     0,    // days
		Compress:   true, // gzip
	}
	fileWriter := zapcore.AddSync(lumberJackLogger)

	// 控制台输出 writer
	consoleWriter := zapcore.AddSync(os.Stdout)

	// 设置日志级别
	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, fileWriter, zapcore.InfoLevel),
		zapcore.NewCore(consoleEncoder, consoleWriter, zapcore.DebugLevel),
	)

	// 构造 logger
	options := []zap.Option{zap.AddCaller(), zap.Development()}
	prefix, _ := gonanoid.New(4)
	Log = zap.New(core, options...).With(zap.String("prefix", prefix))
}

func getEncoder() (zapcore.Encoder, zapcore.Encoder) {
	consoleEncoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout(time.DateTime),
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
	}

	fileEncoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout(time.DateTime),
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
	}
	return zapcore.NewConsoleEncoder(consoleEncoderConfig), zapcore.NewConsoleEncoder(fileEncoderConfig)
}
