package log

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/op/go-logging"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

type formatter struct {
}

func (*formatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(fmt.Sprintf("[%s] [%d] [%s] %s:%d: %s\n", time.Now().Format("2006-01-02 15:04:05.000000"), os.Getpid(), strings.ToLower(entry.Level.String()), entry.Caller.Function, entry.Caller.Line, entry.Message)), nil
}

// return a new customized logrus instance
// 1. customized format
// 2. rotate
func NewLogrus(logPath string, logLevel string) *logrus.Logger {
	f := &lumberjack.Logger{
		Filename: logPath,
		// MaxSize:    100, // megabytes
		// MaxBackups: 30,
		// MaxAge:     30,     //days
		// Compress:   false, // disabled by default
	}

	level := logrus.DebugLevel
	if logLevel == "fatal" {
		level = logrus.FatalLevel
	} else if logLevel == "error" {
		level = logrus.ErrorLevel
	} else if logLevel == "info" {
		level = logrus.InfoLevel
	}

	logger := &logrus.Logger{
		Out:          f,
		Level:        level,
		Formatter:    &formatter{},
		ReportCaller: true,
	}

	go func() {
		c := cron.New()
		err := c.AddFunc("@daily", func() {
			f.Rotate()
		})
		if err != nil {
			logger.Fatal("fail to add log rotate cron job", err)
		}
		c.Start()
	}()

	return logger
}

// return a new customized go-logging instance
// 1. customized format
// 2. rotate
func New(logPath string, logLevel string) *logging.Logger {
	f := &lumberjack.Logger{
		Filename: logPath,
		// MaxSize:    100, // megabytes
		// MaxBackups: 30,
		// MaxAge:     30,     //days
		// Compress:   false, // disabled by default
	}

	logger := logging.MustGetLogger("")
	backend := logging.NewLogBackend(f, "", 0)
	format := logging.MustStringFormatter(
		// `%{color}%{time:2006-01-02 15:04:05.000} %{pid} %{shortfile} %{shortfunc} %{level:.4s} %{color:reset} | %{message}`,
		`[%{time:2006-01-02 15:04:05.000000}] [%{pid}] [%{level:.5s}]%{shortfile}(%{shortfunc}): %{message}`,
	)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	level, _ := logging.LogLevel(logLevel)
	backendLeveled.SetLevel(level, "")
	logger.SetBackend(backendLeveled)

	if logPath != "" {
		go func() {
			c := cron.New()
			err := c.AddFunc("@daily", func() {
				if err2 := f.Rotate(); err2 != nil {
					logger.Errorf("log rotate err.%v file.%s", err2, f.Filename)
				}
			})
			if err != nil {
				logger.Fatal("fail to add log rotate cron job", err)
			}
			c.Start()
		}()
	}

	return logger
}
