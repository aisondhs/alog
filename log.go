package alog

import (
	"time"
)

var aLogger *ALogger

// init global logger container
func Init(dir string, rotateMode int, compress bool) {
	logger, err := New(dir, rotateMode, compress)
	if err != nil {
		panic(err)
	}
	aLogger = logger
}

// close the global logger container
func Close() {
	aLogger.Close()
}

// close the global logger container
func Maxsize(size int64) {
	aLogger.Maxsize(size)
}

// write global logger container info msg
func Info(msg string, data ...interface{}) {
	aLogger.Info(msg, data...)
}

// write global logger container warn msg
func Warn(msg string, data ...interface{}) {
	aLogger.Warn(msg, data...)
}

// write global logger container error msg
func Error(msg string, data ...interface{}) {
	aLogger.Error(msg, data...)
}

// write global logger container debug msg
func Debug(msg string, data ...interface{}) {
	aLogger.Debug(msg, data...)
}

// open or close the global logger debug msg output
func SetDebug(debug bool) {
	aLogger.SetDebug(debug)
}

// Logger container
type ALogger struct {
	logger *Logger
	debug  bool
}

// create a logger container
func New(dir string, rotateMode int, compress bool) (*ALogger, error) {
	logger, err := Create(dir, rotateMode, ".log", compress)
	if err != nil {
		return nil, err
	}
	return &ALogger{logger, true}, nil
}

// set the log file rotate size
func (alogger *ALogger) Maxsize(size int64) {
	alogger.logger.jsonfile.Maxsize(size)
}

// close the log container
func (alogger *ALogger) Close() {
	alogger.logger.Close()
}

func (alogger *ALogger) Log(msg string, logType string, data ...interface{}) {
	rec := Mrecord{
		"Time":    time.Now(),
		"LogType": logType,
		"Message": msg,
	}
	if data != nil {
		rec["Data"] = data
	}
	alogger.logger.Log(rec)
}

// log the info msg
func (alogger *ALogger) Info(msg string, data ...interface{}) {
	alogger.Log(msg, "info", data...)
}

// log the warn msg
func (alogger *ALogger) Warn(msg string, data ...interface{}) {
	alogger.Log(msg, "warn", data...)
}

// log the error msg
func (alogger *ALogger) Error(msg string, data ...interface{}) {
	alogger.Log(msg, "error", data...)
}

// log the debug msg
func (alogger *ALogger) Debug(msg string, data ...interface{}) {
	if alogger.debug {
		alogger.Log(msg, "debug", data...)
	}
}

// open or close the debug msg output
func (alogger *ALogger) SetDebug(debug bool) {
	alogger.debug = debug
}
