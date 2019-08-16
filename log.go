package log

import (
	"fmt"
	"log/syslog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DEBUG = iota
	INFO
	WARN
	ERROR
	FATAL
)

var (
	once          = &sync.Once{}
	wg            = &sync.WaitGroup{}
	logChan       = make(chan log, 1024)
	cfgLevel      = &atomic.Value{}
	cfgPath       = &atomic.Value{}
	cfgSyslog     = &atomic.Value{}
	cfgSyslogFlag = &atomic.Value{}
	cfgTTL        = &atomic.Value{}
	cfgFormat     = &atomic.Value{}
	cfgExtension  = &atomic.Value{}
	cfgSize       = &atomic.Value{}
	cfgStdOut     = &atomic.Value{}
	cfgHook       = &atomic.Value{}
)

type hook struct {
	callback func(level int, message string)
	level    int
}

type log struct {
	level   int
	message string
}

func init() {
	cfgLevel.Store(DEBUG)
	cfgSyslog.Store(&syslog.Writer{})
	cfgSyslogFlag.Store(false)
	cfgTTL.Store(int64(0))
	cfgFormat.Store(func(level int, line string, message string) string {
		levelStr := "DEBUG"

		switch level {
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
			time.Now().Format("2006-01-02 15:04:05"),
			levelStr,
			line,
			message}

		return strings.Join(data, "\t")
	})
	cfgExtension.Store("log")
	cfgSize.Store(int64(-1))
	cfgStdOut.Store(false)
	cfgHook.Store(hook{
		callback: func(level int, message string) {},
		level:    DEBUG})
}

func Path(path string) (err error) {
	cfgPath.Store(path)

	err = os.MkdirAll(path, 0755)

	return
}

func Syslog(tag string) {
	sl, err := syslog.New(syslog.LOG_DEBUG|syslog.LOG_USER, tag)
	if err != nil {
		fmt.Printf("Can't init syslog with tag %s. Catch error %s\n", tag, err.Error())

		return
	}

	cfgSyslog.Store(sl)
	cfgSyslogFlag.Store(true)
}

func Level(level int) {
	if level >= 0 && level < 5 {
		cfgLevel.Store(level)
	}
}

func LevelAsString(level string) {
	Level(getLevelFromString(level))
}

func Format(format func(level int, line string, message string) string) {
	cfgFormat.Store(format)
}

func SizeLimit(size int64) {
	cfgSize.Store(size)
}

func Stdout(state bool) {
	cfgStdOut.Store(state)
}

func TTL(ttl int64) {
	cfgTTL.Store(ttl)
}

func Extension(extension string) {
	cfgExtension.Store(extension)
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

func getFilePath(appendLength int) (path string, err error) {
	timestamp := time.Now().Format("2006-01-02")
	path = filepath.Join(cfgPath.Load().(string), timestamp+"."+cfgExtension.Load().(string))

	path, err = filepath.Abs(path)
	if err != nil {
		return
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return path, nil
	} else if cfgSize.Load().(int64) < 0 ||
		info.Size()+int64(appendLength) <= cfgSize.Load().(int64) {
		return path, nil
	} else {
		var increment int
		increment, err = getMaxIncrement(path)
		if err != nil {
			return
		}

		err = moveFile(path, fmt.Sprintf("%s.%d", path, increment+1))
		if err != nil {
			return
		}
	}

	return
}

func getMaxIncrement(path string) (incr int, err error) {
	path, err = filepath.Abs(path)
	if err != nil {
		return
	}

	matches, err := filepath.Glob(path + ".*")
	if os.IsNotExist(err) {
		return
	} else if err != nil {
		return
	}

	if len(matches) > 0 {
		for _, match := range matches {
			match = strings.Replace(match, path+".", "", -1)
			var i64 int64
			i64, err = strconv.ParseInt(match, 10, 32)

			if err == nil {
				i := int(i64)

				if incr < i {
					incr = i
				}
			} else {
				return
			}
		}

		return
	}

	return
}

func moveFile(sourceFilePath string, destinationFilePath string) error {
	return os.Rename(sourceFilePath, destinationFilePath)
}

func handle(l log) {
	if cfgLevel.Load().(int) <= l.level {
		wg.Add(1)
		l.message = cfgFormat.Load().(func(level int, line, message string) string)(l.level, getFuncName(), l.message)
		logChan <- l
	}

	once.Do(func() {
		if cfgTTL.Load().(int64) > 0 {
			go watchOld()
		}

		go func(logChan chan log) {
			var h hook
			for log := range logChan {
				h = cfgHook.Load().(hook)
				if h.level <= log.level {
					h.callback(log.level, log.message)
				}

				printToStdout(log)
				writeToFile(log)
				writeToSyslog(log)

				wg.Done()
			}
		}(logChan)
	})
}

func printToStdout(l log) {
	if cfgStdOut.Load().(bool) {
		var err error

		if l.level < WARN {
			_, err = fmt.Fprintln(os.Stdout, l.message)
			if err != nil {
				fmt.Printf("Can't write to stdout. Catch error %s\n", err.Error())
			}
		} else {
			_, err = fmt.Fprintln(os.Stderr, l.message)
			if err != nil {
				fmt.Printf("Can't write to stderr. Catch error %s\n", err.Error())
			}
		}
	}
}

func writeToFile(l log) {
	if len(cfgPath.Load().(string)) > 0 {
		filePath, err := getFilePath(len(l.message))
		if err != nil {
			fmt.Printf("Can't access to log file %s. Catch error %s\n", cfgPath.Load().(string), err.Error())

			return
		}

		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			fmt.Printf("Can't write log to file %s. Catch error: %s\n", filePath, err.Error())

			return
		}

		defer func() {
			err = file.Sync()
			if err != nil {
				fmt.Printf("Can't sync log file %s. Catch error: %s\n", filePath, err.Error())
			}

			err = file.Close()
			if err != nil {
				fmt.Printf("Can't sync log file %s. Catch error: %s\n", filePath, err.Error())
			}
		}()

		_, err = file.WriteString(l.message + "\n")
		if err != nil {
			fmt.Printf("Can't write log to file %s. Catch error: %s\n", filePath, err.Error())
		}
	}
}

func writeToSyslog(l log) {
	var err error

	if cfgSyslogFlag.Load().(bool) {
		sl := cfgSyslog.Load().(*syslog.Writer)

		switch l.level {
		case FATAL:
			err = sl.Emerg(l.message)
		case ERROR:
			err = sl.Err(l.message)
		case WARN:
			err = sl.Warning(l.message)
		case INFO:
			err = sl.Info(l.message)
		case DEBUG:
			err = sl.Debug(l.message)
		}
	}

	if err != nil {
		fmt.Printf("Can't write log to syslog. Catch error: %s\n", err.Error())
	}
}

func watchOld() {
	for {
		deleteOld()

		time.Sleep(time.Hour)
	}
}

func deleteOld() {
	paths, err := filepath.Glob(cfgPath.Load().(string) + string(filepath.Separator) + "*")
	if err != nil {
		fmt.Printf("Can't access to log file %s. Catch error %s\n", cfgPath.Load().(string), err.Error())

		return
	} else {
		ttl := float64(cfgTTL.Load().(int64))

		for _, path := range paths {
			file, err := os.Stat(path)
			if err != nil {
				fmt.Printf("Can't access to log file %s. Catch error %s\n", cfgPath.Load().(string), err.Error())

				return
			} else if !file.IsDir() {
				if time.Now().Sub(file.ModTime()).Seconds() > ttl {
					err = os.Remove(path)
					if err != nil {
						fmt.Printf("Can't remove old log file %s. Catch error %s\n", path, err.Error())

						return
					}
				}
			}
		}
	}
}

func Flush() {
	wg.Wait()
}

func Hook(callback func(level int, message string), level string) {
	cfgHook.Store(hook{
		callback: callback,
		level:    getLevelFromString(level)})
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
