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
	logger *gslog.Logger
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

func getLogger() *gslog.Logger {
	timestamp := time.Now().Format("2006-01-02")
	writer := gslog.WriterNew(path + "/" + timestamp + ".log")

	logger = gslog.GetLogger(getFuncName())
	logger.SetLevel(level)
	logger.SetWriter(writer)

	return logger
}

func getFuncName() string {
	_, scriptName, line, _ := runtime.Caller(3)

	appPath, _ := os.Getwd()
	appPath += string(os.PathSeparator)

	return fmt.Sprintf("%s:%d", strings.Replace(scriptName, appPath, "", -1), line)
}

func Debug(line string) {
	getLogger().Debug(line)
}

func Info(line string) {
	getLogger().Info(line)
}

func Warn(line string) {
	getLogger().Warn(line)
}

func Error(line string) {
	getLogger().Error(line)
}

func Fatal(line string) {
	getLogger().Fatal(line)
}
