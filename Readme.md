# Golang thread-safe, lock-free file & stdout

## Import
```go
import "github.com/leprosus/golang-log"
```

## Settings

```go
// Set directory path to save log files
log.Path("./log")

// Set log level by constant
log.Level(log.DebugLevel)

// Set log level by string
log.LevelAsString("debug")

// Set function to prepare format log line
log.Format(func(level int, line string, message string) string{
    return fmt.Sprintf("Only message: %s", message)
})

// Set log file size limit
log.SizeLimit(2 * log.megaByte)

// Set log output to stdout 
log.Stdout(true)
```

## Usage
```go
// Write debug line
// Usually use to log debug data
log.Debug("debug line")

// Write information
// Usually use to log some state
log.Info("info line")

// Write warning
// Usually use to log warnings about unexpected application state (ex.: brudforce, incorrect request, bad loging&password authorization) 
log.Warn("warn line")

// Write error
// Use only in a case of a return error what doesn't effect application run
log.Error("error line")
```

## List all methods

* log.Path("./log") - sets directory path to save log files
* log.Level(log.Debug) - sets log level by constant
* log.LevelAsString("debug") - sets log level by string
* log.Format(func(level SeverityLevel, line string, message string) string) - sets function to prepare format log line
* log.SizeLimit(2 * log.megaByte) - sets log file size limit (if value is less zero then size limit is off)
* log.Stdout(true) - sets log output to stdout
* log.TTL(3600) - sets time-to-live of log files (all old files will be removed)
* log.Extension("txt") = sets another log files extension (.log is default)
* log.Emergency("emergency line") - writes a message when system is unusable
* log.EmergencyFmt("emergency line %d", 1) - writes a message when system is unusable
* log.Alert("alert line") - writes a message when action must be taken immediately
* log.AlertFmt("alert line %d", 1) - writes a message when action must be taken immediately
* log.Critical("critical line") - writes a message with critical conditions
* log.CriticalFmt("critical line %d", 1) - writes a message with critical conditions
* log.Error("error line") - writes a message with error conditions
* log.ErrorFmt("error line %d", 1) - writes a message with error conditions
* log.Warn("warning line") - writes a message with warning conditions
* log.WarnFmt("warning line %d", 1) - writes a message with warning conditions
* log.Notice("notice line") - writes a message when everything is normal but significant condition
* log.NoticeFmt("notice line %d", 1) - writes a message when everything is normal but significant condition
* log.Info("information line") - writes a informational message
* log.Info("information line %d", 1) - writes a informational message
* log.Debug("debug line") - writes message with debug data
* log.Debug("debug line %d", 1) - writes message with debug data
* log.Flush() - finish all process
* log.Hook(func(message string){ /* do something */ }, "warn") - sets callback on all message with level more or equal "warn" ("warn", "error", "fatal")