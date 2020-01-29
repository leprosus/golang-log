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

type SeverityLevel uint

const (
	EmergencyLevel SeverityLevel = iota // system is unusable
	AlertLevel                          // action must be taken immediately
	CriticalLevel                       // critical conditions
	ErrorLevel                          // error conditions
	WarnLevel                           // warning conditions
	NoticeLevel                         // normal but significant condition
	InfoLevel                           // informational messages
	DebugLevel                          // debug-level messages
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
	callback func(level SeverityLevel, message string)
	level    SeverityLevel
}

type log struct {
	level   SeverityLevel
	message string
}

func init() {
	cfgLevel.Store(DebugLevel)
	cfgSyslog.Store(&syslog.Writer{})
	cfgSyslogFlag.Store(false)
	cfgTTL.Store(int64(0))
	cfgFormat.Store(func(level SeverityLevel, line string, message string) string {
		levelStr := "DEBUG"

		switch level {
		case InfoLevel:
			levelStr = "INFO"
		case NoticeLevel:
			levelStr = "NOTICE"
		case WarnLevel:
			levelStr = "WARN"
		case ErrorLevel:
			levelStr = "ERROR"
		case CriticalLevel:
			levelStr = "CRITICAL"
		case AlertLevel:
			levelStr = "ALERT"
		case EmergencyLevel:
			levelStr = "EMERGENCY"
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
		callback: func(level SeverityLevel, message string) {},
		level:    DebugLevel})
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

func Level(level SeverityLevel) {
	cfgLevel.Store(level)
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

func getLevelFromString(level string) SeverityLevel {
	switch strings.ToLower(level) {
	case "info":
		return InfoLevel
	case "notice":
		return NoticeLevel
	case "warn":
		return WarnLevel
	case "error":
		return ErrorLevel
	case "critical":
		return CriticalLevel
	case "alert":
		return AlertLevel
	case "emergency":
		return EmergencyLevel
	default:
		return DebugLevel
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
	if cfgLevel.Load().(SeverityLevel) <= l.level {
		wg.Add(1)
		l.message = cfgFormat.Load().(func(level SeverityLevel, line, message string) string)(l.level, getFuncName(), l.message)
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

		if l.level < WarnLevel {
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
	if cfgPath.Load() != nil && len(cfgPath.Load().(string)) > 0 {
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
		case EmergencyLevel:
			err = sl.Emerg(l.message)
		case AlertLevel:
			err = sl.Alert(l.message)
		case CriticalLevel:
			err = sl.Crit(l.message)
		case ErrorLevel:
			err = sl.Err(l.message)
		case WarnLevel:
			err = sl.Warning(l.message)
		case NoticeLevel:
			err = sl.Notice(l.message)
		case InfoLevel:
			err = sl.Info(l.message)
		case DebugLevel:
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

func Hook(callback func(level SeverityLevel, message string), level string) {
	cfgHook.Store(hook{
		callback: callback,
		level:    getLevelFromString(level)})
}

func Emergency(message string, args ...interface{}) {
	handle(log{level: EmergencyLevel, message: message})
}

func Alert(message string) {
	handle(log{level: AlertLevel, message: message})
}

func Critical(message string) {
	handle(log{level: CriticalLevel, message: message})
}

func Error(message string) {
	handle(log{level: ErrorLevel, message: message})
}

func Warn(message string) {
	handle(log{level: WarnLevel, message: message})
}

func Notice(message string) {
	handle(log{level: NoticeLevel, message: message})
}

func Info(message string) {
	handle(log{level: InfoLevel, message: message})
}

func Debug(message string) {
	handle(log{level: DebugLevel, message: message})
}

func EmergencyFmt(message string, args ...interface{}) {
	handle(log{level: EmergencyLevel, message: fmt.Sprintf(message, args...)})
}

func AlertFmt(message string, args ...interface{}) {
	handle(log{level: AlertLevel, message: fmt.Sprintf(message, args...)})
}

func CriticalFmt(message string, args ...interface{}) {
	handle(log{level: CriticalLevel, message: fmt.Sprintf(message, args...)})
}

func ErrorFmt(message string, args ...interface{}) {
	handle(log{level: ErrorLevel, message: fmt.Sprintf(message, args...)})
}

func WarnFmt(message string, args ...interface{}) {
	handle(log{level: WarnLevel, message: fmt.Sprintf(message, args...)})
}

func NoticeFmt(message string, args ...interface{}) {
	handle(log{level: NoticeLevel, message: fmt.Sprintf(message, args...)})
}

func InfoFmt(message string, args ...interface{}) {
	handle(log{level: InfoLevel, message: fmt.Sprintf(message, args...)})
}

func DebugFmt(message string, args ...interface{}) {
	handle(log{level: DebugLevel, message: fmt.Sprintf(message, args...)})
}
