package logger

import "go.uber.org/zap"

type ZapLoggerAdapter struct {
	*zap.Logger
}

func (l *ZapLoggerAdapter) Fatalf(format string, args ...interface{}) {
	l.Sugar().Fatalf(format, args...)
}
func (l *ZapLoggerAdapter) Errorf(format string, args ...interface{}) {
	l.Sugar().Errorf(format, args...)
}
func (l *ZapLoggerAdapter) Warnf(format string, args ...interface{}) {
	l.Sugar().Warnf(format, args...)
}
func (l *ZapLoggerAdapter) Infof(format string, args ...interface{}) {
	l.Sugar().Infof(format, args...)
}
func (l *ZapLoggerAdapter) Debugf(format string, args ...interface{}) {
	l.Sugar().Debugf(format, args...)
}

func (l *ZapLoggerAdapter) Printf(format string, args ...interface{}) {
	l.Sugar().Infof(format, args...)
}
