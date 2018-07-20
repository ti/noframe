package log

import (
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	"github.com/onrik/logrus/filename"
	"github.com/sirupsen/logrus"
)

func init() {
	filenameHook := filename.NewHook()
	filenameHook.Field = "source"
	logrus.AddHook(filenameHook)
	grpc_logrus.ReplaceGrpcLogger(logrus.NewEntry(logrus.StandardLogger()))
}
