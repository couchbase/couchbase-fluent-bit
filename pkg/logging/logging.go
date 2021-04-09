package logging

import "go.uber.org/zap"

var (
	logger, _ = zap.NewProduction()
	Log       = logger.Sugar().Named("couchbase-watcher")
)

func GetLogger(name string) *zap.SugaredLogger {
	return logger.Sugar().Named(name)
}
