package log

import (
	"time"
	"github.com/pastebt/gslog"
	"runtime"
	"os"
	"strings"
	"fmt"
)

var (
	logger map[string]*gslog.Logger = map[string]*gslog.Logger{}
	level int = gslog.DEBUG
	path = "./log"
)

func SetLevel(cusLevel string) {
	switch cusLevel {
	case "debug":
		level = gslog.DEBUG
	case "info":
		level = gslog.INFO
	case "warn":
		level = gslog.WARNING
	case "error":
		level = gslog.ERROR
	case "fatal":
		level = gslog.FATAL
	default:
		level = gslog.DEBUG
	}
}

func SetPath(newPath string) {
	path = newPath
}

func getLogger(fullFuncName string) *gslog.Logger {
	if logger[fullFuncName] == nil {
		timestamp := time.Now().Format("2006-01-02")
		writer := gslog.WriterNew(path + "/" + timestamp + ".log")

		logger[fullFuncName] = gslog.GetLogger(fullFuncName)
		logger[fullFuncName].SetLevel(level)
		logger[fullFuncName].SetWriter(writer)
	}

	return logger[fullFuncName]
}

func GetFuncName() string {
	_, scriptName, line, _ := runtime.Caller(1)

	appPath, _ := os.Getwd()
	appPath += string(os.PathSeparator)

	return fmt.Sprintf("%s:%d", strings.Replace(scriptName, appPath, "", -1), line)
}

func Debug(fullFuncName string, line string) {
	getLogger(fullFuncName).Debug(line)
}

func Info(fullFuncName string, line string) {
	getLogger(fullFuncName).Info(line)
}

func Warn(fullFuncName string, line string) {
	getLogger(fullFuncName).Warn(line)
}

func Error(fullFuncName string, line string) {
	getLogger(fullFuncName).Error(line)
}

func Fatal(fullFuncName string, line string) {
	getLogger(fullFuncName).Fatal(line)
}
