package log

import (
	"time"
	"runtime"
	"os"
	"strings"
	"fmt"
	"sync"
	"path/filepath"
	"strconv"
)

const (
	DEBUG = 1
	INFO = 2
	WARN = 3
	ERROR = 4
	FATAL = 5
)

const (
	kiloByte = 1024
	megaByte = kiloByte * 1024
	gigaByte = megaByte * 1024
)

type logger struct {
	level  int
	path   string
	format func(level int, line string, message string) string
	size   int64
	stdout bool
}

var (
	log logger = logger{
		level: DEBUG,
		path: "./log",
		format: func(level int, line string, message string) string {
			now := time.Now().Format("2006-01-02 15:04:05")
			levelStr := "DEBUG"

			switch level {
			case DEBUG:
				levelStr = "DEBUG"
			case INFO:
				levelStr = "INFO"
			case WARN:
				levelStr = "WARN"
			case ERROR:
				levelStr = "ERROR"
			case FATAL:
				levelStr = "FATAL"
			}

			data := []string{
				now,
				levelStr,
				line,
				message}

			return strings.Join(data, "\t")
		},
		size: megaByte,
		stdout: false}

	mutex = &sync.Mutex{}
)

func Path(path string) {
	log.path = path
}

func Level(level int) {
	if level > 0 && level < 6 {
		log.level = level
	}
}

func LevelAsString(level string) {
	switch level {
	case "debug":
		Level(DEBUG)
	case "info":
		Level(INFO)
	case "warn":
		Level(WARN)
	case "error":
		Level(ERROR)
	case "fatal":
		Level(FATAL)
	default:
		Level(DEBUG)
	}
}

func Format(format func(level int, line string, message string) string) {
	log.format = format
}

func SizeLimit(size int64) {
	log.size = size
}

func Stdout(state bool) {
	log.stdout = state
}

func getFuncName() string {
	_, scriptName, line, _ := runtime.Caller(3)

	appPath, _ := os.Getwd()
	appPath += string(os.PathSeparator)

	return fmt.Sprintf("%s:%d", strings.Replace(scriptName, appPath, "", -1), line)
}

func getFilePath(appendLength int) (path string, err error) {
	timestamp := time.Now().Format("2006-01-02")
	basePath := log.path + string(os.PathSeparator) + timestamp + ".log"

	increment, err := getMaxIncrement(path)
	if err != nil {
		return basePath, err
	}

	if increment > 0 {
		path = fmt.Sprintf("%s.%d", basePath, increment)
	} else {
		path = basePath
	}

	isFull := true

	for isFull {
		info, err := os.Stat(path)
		if err != nil {
			return path, err
		}

		if info.Size() + int64(appendLength) > log.size {
			increment++

			path = fmt.Sprintf("%s.%d", basePath, increment)
		} else {
			isFull = false
		}
	}

	return path, nil
}

func getMaxIncrement(path string) (int, error) {
	matches, err := filepath.Glob(path + ".*")
	if err != nil {
		return 0, err
	}

	if len(matches) > 0 {
		max := 0

		for _, match := range matches {
			match = strings.Replace(match, path, "", -1)
			i32, err := strconv.ParseInt(match, 10, 32)

			if err == nil {
				i := int(i32)

				if max < i {
					max = i
				}
			}
		}

		return max, nil
	}

	return 0, nil
}

func write(level int, message string) {
	if log.level >= level {
		logLine := log.format(level, getFuncName(), message)
		filePath, err := getFilePath(len(logLine))
		if err != nil {
			fmt.Printf("Can't scan log directory %s. Catch error %s", log.path, err.Error())

			return
		}

		mutex.Lock()

		file, err := os.OpenFile(filePath, os.O_APPEND | os.O_CREATE | os.O_WRONLY, 0600)

		defer file.Sync()
		defer file.Close()
		defer mutex.Unlock()

		if err != nil {
			fmt.Printf("Can't write log to file %s. Catch error %s", filePath, err.Error())

			return
		}

		_, err = file.WriteString(logLine)
		if err != nil {
			fmt.Printf("Can't write log to file %s. Catch error %s", filePath, err.Error())

			return
		}

		if log.stdout {
			fmt.Println(logLine)
		}
	}
}

func Debug(message string) {
	write(DEBUG, message)
}

func Info(message string) {
	write(INFO, message)
}

func Warn(message string) {
	write(WARN, message)
}

func Error(message string) {
	write(ERROR, message)
}

func Fatal(message string) {
	write(FATAL, message)
}
