package main

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Log struct {
	lg zerolog.Logger
}

var logs = "logs"

func NewLogger() *Log {
	os.MkdirAll(logs, 0755)
	fileName := filepath.Join(logs, "gateway.log")
	var logger *Log = &Log{}
	logFile := &lumberjack.Logger{
		Filename:   fileName,
		MaxSize:    100,
		MaxAge:     30,
		MaxBackups: 2,
		Compress:   true,
	}
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}
	log := zerolog.New(multiWriter).Level(zerolog.DebugLevel).With().Caller().Logger()
	logger.lg = log
	return logger
}

func (l *Log) Info(key, msg string) {
	l.lg.Info().Str(key, msg).Msg("")
}

func (l *Log) Error(key string, err error) {
	l.lg.Err(err).AnErr(key, err).Msg("")
}

func (l *Log) Warning(key string, msg string) {
	l.lg.Warn().Str(key, msg).Msg("")
}

func (l *Log) Debug(key string, msg string) {
	l.lg.Debug().Str(key, msg).Msg("")
}

func (l *Log) Fatal(key string, err error) {
	l.lg.Fatal().AnErr(key, err)
}
