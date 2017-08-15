package log

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DEBUG = 1
	INFO  = 2
	WARN  = 3
	ERROR = 4
	FATAL = 5
)

const (
	kiloByte = 1024
	megaByte = kiloByte * 1024
	gigaByte = megaByte * 1024
)

type config struct {
	level    int
	path     string
	format   func(level int, line string, message string) string
	size     int64
	logChan  chan log
	stdout   bool
	once     *sync.Once
	wg       *sync.WaitGroup
	notifier notifier
}
type notifier struct {
	callback func(message string)
	level    int
}

type log struct {
	level   int
	message string
}

var (
	cfg = config{
		level: DEBUG,
		path:  "./log",
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
		size:    megaByte,
		logChan: make(chan log, 100),
		stdout:  false,
		once:    &sync.Once{},
		wg:      &sync.WaitGroup{},
		notifier: notifier{
			callback: func(message string) {},
			level:    DEBUG}}
)

func Path(path string) {
	cfg.path = path

	os.MkdirAll(cfg.path, os.ModePerm)
}

func Level(level int) {
	if level > 0 && level < 6 {
		cfg.level = level
	}
}

func LevelAsString(level string) {
	Level(getLevelFromString(level))
}

func Format(format func(level int, line string, message string) string) {
	cfg.format = format
}

func SizeLimit(size int64) {
	cfg.size = size
}

func Stdout(state bool) {
	cfg.stdout = state
}

func getLevelFromString(level string) int {
	switch strings.ToLower(level) {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn":
		return WARN
	case "error":
		return ERROR
	case "fatal":
		return FATAL
	default:
		return DEBUG
	}
}

func getFuncName() string {
	_, scriptName, line, _ := runtime.Caller(3)

	appPath, _ := os.Getwd()
	appPath += string(os.PathSeparator)

	return fmt.Sprintf("%s:%d", strings.Replace(scriptName, appPath, "", -1), line)
}

func getFilePath(appendLength int) (string, error) {
	timestamp := time.Now().Format("2006-01-02")
	path := cfg.path + string(os.PathSeparator) + timestamp + ".log"

	info, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return path, nil
	} else if info.Size()+int64(appendLength) <= cfg.size {
		return path, nil
	} else {
		increment, err := getMaxIncrement(path)
		if err != nil {
			return path, err
		}

		err = moveFile(path, fmt.Sprintf("%s.%d", path, increment+1))
		if err != nil {
			return path, err
		}
	}

	return path, nil
}

func getMaxIncrement(path string) (int, error) {
	matches, err := filepath.Glob(path + ".*")
	if os.IsNotExist(err) {
		return 0, nil
	} else if err != nil {
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

func moveFile(sourceFilePath string, destinationFilePath string) error {
	return os.Rename(sourceFilePath, destinationFilePath)
}

func handle(l log) {
	cfg.wg.Add(1)
	l.message = cfg.format(l.level, getFuncName(), l.message)
	cfg.logChan <- l

	cfg.once.Do(func() {
		go func(logChan chan log) {
			for log := range logChan {
				if cfg.notifier.level <= log.level {
					cfg.notifier.callback(log.message)
				}

				write(log)

				cfg.wg.Done()
			}
		}(cfg.logChan)
	})
}

func write(l log) {
	if cfg.level <= l.level {
		filePath, err := getFilePath(len(l.message))
		if err != nil {
			fmt.Printf("Can't access to log file %s. Catch error %s\n", cfg.path, err.Error())

			return
		}

		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)

		defer file.Sync()
		defer file.Close()

		if err != nil {
			fmt.Printf("Can't write log to file %s. Catch error: %s\n", filePath, err.Error())

			return
		}

		_, err = file.WriteString(l.message + "\n")
		if err != nil {
			fmt.Printf("Can't write log to file %s. Catch error: %s\n", filePath, err.Error())

			return
		}

		if cfg.stdout {
			if l.level < WARN {
				fmt.Fprintln(os.Stdout, l.message)
			} else {
				fmt.Fprintln(os.Stderr, l.message)
			}
		}
	}
}

func Flush() {
	cfg.wg.Wait()
}

func Notifier(callback func(message string), level string) {
	cfg.notifier = notifier{
		callback: callback,
		level:    getLevelFromString(level)}
}

func Debug(message string) {
	handle(log{level: DEBUG, message: message})
}

func Info(message string) {
	handle(log{level: INFO, message: message})
}

func Warn(message string) {
	handle(log{level: WARN, message: message})
}

func Error(message string) {
	handle(log{level: ERROR, message: message})
}

func Fatal(message string) {
	handle(log{level: FATAL, message: message})
}

func DebugFmt(message string, args ...interface{}) {
	handle(log{level: DEBUG, message: fmt.Sprintf(message, args...)})
}

func InfoFmt(message string, args ...interface{}) {
	handle(log{level: INFO, message: fmt.Sprintf(message, args...)})
}

func WarnFmt(message string, args ...interface{}) {
	handle(log{level: WARN, message: fmt.Sprintf(message, args...)})
}

func ErrorFmt(message string, args ...interface{}) {
	handle(log{level: ERROR, message: fmt.Sprintf(message, args...)})
}

func FatalFmt(message string, args ...interface{}) {
	handle(log{level: FATAL, message: fmt.Sprintf(message, args...)})
}
