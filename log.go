package log

import (
	"fmt"
	"io"
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
	once         = &sync.Once{}
	wg           = &sync.WaitGroup{}
	mx           = &sync.Mutex{}
	logChan      = make(chan Log, 1024)
	cfgLevel     = &atomic.Value{}
	cfgPath      = &atomic.Value{}
	cfgTTL       = &atomic.Value{}
	cfgFormat    = &atomic.Value{}
	cfgExtension = &atomic.Value{}
	cfgSize      = &atomic.Value{}
	cfgStdOut    = &atomic.Value{}
	cfgHook      = &atomic.Value{}
)

type hook struct {
	callback func(log Log)
	level    SeverityLevel
}

type Log struct {
	Level     SeverityLevel
	Message   string
	TimeStamp time.Time
	Path      string
	Line      int
	Full      string
}

func init() {
	cfgLevel.Store(DebugLevel)
	cfgTTL.Store(int64(0))
	cfgFormat.Store(func(log Log) string {
		levelStr := "DEBUG"

		switch log.Level {
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
			log.Path,
			fmt.Sprint(log.Line),
			log.Message}

		return strings.Join(data, "\t")
	})
	cfgExtension.Store("log")
	cfgSize.Store(int64(-1))
	cfgStdOut.Store(false)
	cfgHook.Store(hook{
		callback: func(log Log) {},
		level:    DebugLevel})
}

func Path(path string) (err error) {
	path, err = filepath.Abs(path)
	if err != nil {
		return
	}

	cfgPath.Store(path)

	err = os.MkdirAll(path, 0755)

	return
}

func Level(level SeverityLevel) {
	cfgLevel.Store(level)
}

func LevelAsString(level string) {
	Level(getLevelFromString(level))
}

func Format(format func(level SeverityLevel, line int, message string) string) {
	cfgFormat.Store(func(log Log) string {
		return format(log.Level, log.Line, log.Message)
	})
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
	case "warn", "warning":
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

var (
	srcMark    = filepath.Join("go", "src")
	modMark    = filepath.Join("go", "pkg", "mod")
	goPathMark = filepath.Join("gopath", "src")
)

func getFuncName() (path string, line int) {
	_, path, line, _ = runtime.Caller(3)

	var ix = -1

	if ix = strings.Index(path, srcMark); ix > -1 {
		path = path[ix+len(srcMark)+1:]
	} else if ix = strings.Index(path, modMark); ix > -1 {
		path = path[ix+len(modMark)+1:]
	} else if ix = strings.Index(path, goPathMark); ix > -1 {
		path = path[ix+len(goPathMark)+1:]
	}

	return
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

func handle(l Log) {
	if cfgLevel.Load().(SeverityLevel) >= l.Level {
		mx.Lock()
		wg.Add(1)
		mx.Unlock()

		l.TimeStamp = time.Now()

		if len(l.Message) > 0 {
			l.Message = strings.ToUpper(l.Message[0:1]) + l.Message[1:]
		}

		l.Path, l.Line = getFuncName()
		l.Full = cfgFormat.Load().(func(log Log) string)(l)
		logChan <- l
	}

	once.Do(func() {
		if cfgTTL.Load().(int64) > 0 {
			go watchOld()
		}

		go func(logChan chan Log) {
			var h hook
			for log := range logChan {
				h = cfgHook.Load().(hook)
				if h.level >= log.Level {
					h.callback(log)
				}

				printToStdout(log)
				writeToFile(log)

				mx.Lock()
				wg.Done()
				mx.Unlock()
			}
		}(logChan)
	})
}

func printToStdout(l Log) {
	if cfgStdOut.Load().(bool) {
		var err error

		if l.Level < WarnLevel {
			_, err = fmt.Fprintln(os.Stdout, l.Full)
			if err != nil {
				io.WriteString(os.Stderr, fmt.Sprintf("Can't write to stdout. Catch error %s\n", err.Error()))
			}
		} else {
			_, err = fmt.Fprintln(os.Stderr, l.Full)
			if err != nil {
				io.WriteString(os.Stderr, fmt.Sprintf("Can't write to stderr. Catch error %s\n", err.Error()))
			}
		}
	}
}

func writeToFile(l Log) {
	path := cfgPath.Load()
	if path != nil && len(path.(string)) > 0 {
		filePath, err := getFilePath(len(l.Full))
		if err != nil {
			io.WriteString(os.Stderr, fmt.Sprintf("Can't access to log file %s. Catch error %s\n", cfgPath.Load().(string), err.Error()))

			return
		}

		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			io.WriteString(os.Stderr, fmt.Sprintf("Can't write log to file %s. Catch error: %s\n", filePath, err.Error()))

			return
		}

		defer func() {
			err = file.Sync()
			if err != nil {
				io.WriteString(os.Stderr, fmt.Sprintf("Can't sync log file %s. Catch error: %s\n", filePath, err.Error()))
			}

			err = file.Close()
			if err != nil {
				io.WriteString(os.Stderr, fmt.Sprintf("Can't sync log file %s. Catch error: %s\n", filePath, err.Error()))
			}
		}()

		_, err = file.WriteString(l.Full + "\n")
		if err != nil {
			io.WriteString(os.Stderr, fmt.Sprintf("Can't write log to file %s. Catch error: %s\n", filePath, err.Error()))
		}
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
		io.WriteString(os.Stderr, fmt.Sprintf("Can't access to log file %s. Catch error %s\n", cfgPath.Load().(string), err.Error()))

		return
	} else {
		ttl := float64(cfgTTL.Load().(int64))

		for _, path := range paths {
			file, err := os.Stat(path)
			if err != nil {
				io.WriteString(os.Stderr, fmt.Sprintf("Can't access to log file %s. Catch error %s\n", cfgPath.Load().(string), err.Error()))

				return
			} else if !file.IsDir() {
				if time.Now().Sub(file.ModTime()).Seconds() > ttl {
					err = os.Remove(path)
					if err != nil {
						io.WriteString(os.Stderr, fmt.Sprintf("Can't remove old log file %s. Catch error %s\n", path, err.Error()))

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

func Hook(callback func(log Log), level string) {
	cfgHook.Store(hook{
		callback: callback,
		level:    getLevelFromString(level),
	})
}

func Emergency(message string) {
	handle(Log{Level: EmergencyLevel, Message: message})
}

func Alert(message string) {
	handle(Log{Level: AlertLevel, Message: message})
}

func Critical(message string) {
	handle(Log{Level: CriticalLevel, Message: message})
}

func Error(message string) {
	handle(Log{Level: ErrorLevel, Message: message})
}

func Warn(message string) {
	handle(Log{Level: WarnLevel, Message: message})
}

func Notice(message string) {
	handle(Log{Level: NoticeLevel, Message: message})
}

func Info(message string) {
	handle(Log{Level: InfoLevel, Message: message})
}

func Debug(message string) {
	handle(Log{Level: DebugLevel, Message: message})
}

func EmergencyFmt(message string, args ...interface{}) {
	handle(Log{Level: EmergencyLevel, Message: fmt.Sprintf(message, args...)})
}

func AlertFmt(message string, args ...interface{}) {
	handle(Log{Level: AlertLevel, Message: fmt.Sprintf(message, args...)})
}

func CriticalFmt(message string, args ...interface{}) {
	handle(Log{Level: CriticalLevel, Message: fmt.Sprintf(message, args...)})
}

func ErrorFmt(message string, args ...interface{}) {
	handle(Log{Level: ErrorLevel, Message: fmt.Sprintf(message, args...)})
}

func WarnFmt(message string, args ...interface{}) {
	handle(Log{Level: WarnLevel, Message: fmt.Sprintf(message, args...)})
}

func NoticeFmt(message string, args ...interface{}) {
	handle(Log{Level: NoticeLevel, Message: fmt.Sprintf(message, args...)})
}

func InfoFmt(message string, args ...interface{}) {
	handle(Log{Level: InfoLevel, Message: fmt.Sprintf(message, args...)})
}

func DebugFmt(message string, args ...interface{}) {
	handle(Log{Level: DebugLevel, Message: fmt.Sprintf(message, args...)})
}
