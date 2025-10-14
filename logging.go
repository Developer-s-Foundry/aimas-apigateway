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
		formatted := fmt.Sprintf("[%s] INFO  --- %s\n", time.Now().Format(time.RFC3339), line)
		_, err = tw.Writer.Write([]byte(formatted))
		return len(p), err
	}

	timestamp := fmt.Sprintf("%v", logEntry["time"])
	if timestamp == "" || timestamp == "<nil>" {
		timestamp = time.Now().Format(time.RFC3339)
	}

	level := strings.ToUpper(fmt.Sprintf("%v", logEntry["level"]))
	if level == "" || level == "<nil>" {
		level = "INFO"
	}

	message := fmt.Sprintf("%v", logEntry["message"])

	method := safeString(logEntry["method"])
	path := safeString(logEntry["path"])
	status := safeString(logEntry["status_code"])
	latency := safeString(logEntry["latency"])
	service := safeString(logEntry["service_target"])
	requestID := safeString(logEntry["request_id"])

	if method != "" && path != "" {
		formatted := fmt.Sprintf("[%s] %-5s %s %s %s (%s) [%s] request_id=%s msg=\"%s\"\n",
			timestamp, level, method, path, status, latency, service, requestID, message)
		_, err = tw.Writer.Write([]byte(formatted))
		return len(p), err
	}

	formatted := fmt.Sprintf("[%s] %-5s --- %s\n", timestamp, level, message)
	_, err = tw.Writer.Write([]byte(formatted))
	return len(p), err
}

func safeString(v interface{}) string {
	if v == nil {
		return ""
	}
	s := fmt.Sprintf("%v", v)
	if s == "<nil>" {
		return ""
	}
	return s
}

func NewLogger() *Log {
	mode := os.Getenv("MODE")
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

	if strings.ToLower(mode) != "debug" {
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
	zerolog.TimeFieldFormat = time.RFC3339
	log := zerolog.New(writer).Level(zerolog.DebugLevel).With().Timestamp().CallerWithSkipFrameCount(2).Logger()
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
