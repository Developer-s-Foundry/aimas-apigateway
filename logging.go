package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Log struct {
	lg zerolog.Logger
}

var logs = "logs"

type TextWriter struct {
	io.Writer
}

func (tw TextWriter) Write(p []byte) (n int, err error) {
	var logEntry map[string]interface{}
	if err := json.Unmarshal(p, &logEntry); err != nil {
		line := strings.TrimSpace(string(p))

		level := "info"
		if strings.Contains(strings.ToLower(line), "error") {
			level = "error"
		} else if strings.Contains(strings.ToLower(line), "warn") {
			level = "warn"
		} else if strings.Contains(strings.ToLower(line), "debug") {
			level = "debug"
		}
		formatted := fmt.Sprintf("level=%s message=%s\n", level, line)
		_, err = tw.Writer.Write([]byte(formatted))
		return len(p), err
	}

	var parts []string
	keys := []string{
		"level", "method", "path", "status_code", "latency",
		"service_target", "user_agent", "request_id", "caller", "message",
	}
	for _, k := range keys {
		if val, ok := logEntry[k]; ok {
			parts = append(parts, fmt.Sprintf("%s=%v", k, val))
		}
	}

	line := strings.Join(parts, " ") + "\n"
	_, err = tw.Writer.Write([]byte(line))
	return len(p), err
}

func NewLogger() *Log {
	mode := os.Getenv("debug")
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

	var writer io.Writer

	if mode != "test" {
		writer = io.MultiWriter(TextWriter{Writer: os.Stdout}, logFile)
	} else {
		writer = TextWriter{Writer: os.Stdout}
	}
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		const maxDepth = 15
		prefix := "aimas-apigateway"
		for i := 2; i < maxDepth; i++ {
			_, file, line, ok := runtime.Caller(i)
			if !ok {
				break
			}
			if strings.Contains(file, "/rs/zerolog/") ||
				strings.Contains(file, "/runtime/") ||
				strings.Contains(file, "/testing/") ||
				strings.Contains(file, "logging.go") {
				continue
			}

			if strings.Contains(file, prefix) {
				return fmt.Sprintf("%s:%d", filepath.Base(file), line)
			}
		}
		_, f, l, _ := runtime.Caller(0)
		caller := fmt.Sprintf("%s:%d", filepath.Base(f), l)
		return caller
	}
	log := zerolog.New(writer).Level(zerolog.DebugLevel).With().CallerWithSkipFrameCount(2).Logger()
	logger.lg = log
	return logger
}

func (l *Log) Info(key, msg string) {
	l.lg.Info().Str("Context", key).Msg(msg)
}

func (l *Log) Error(key, message string, err error) {
	l.lg.Error().AnErr(key, err).Msg(message)
}

func (l *Log) Warning(key string, msg string) {
	l.lg.Warn().Str("context", key).Msg(msg)
}

func (l *Log) Debug(key string, msg string) {
	l.lg.Debug().Str("context", key).Msg(msg)
}

func (l *Log) Fatal(key string, message string, err error) {
	l.lg.Fatal().AnErr(key, err).Msg(message)
}
