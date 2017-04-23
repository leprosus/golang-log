# Golang file logger

## Settings

```go
// Set directory path to save log files
log.Path("./log")

// Set log level by constant
log.Level(log.DEBUG)

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

// Write fatal error
// Use only in a case of a return error what do effect application run
log.Fatal("fatal line")
```

## List all methods

* log.Path("./log") - sets directory path to save log files
* log.Level(log.DEBUG) - sets log level by constant
* log.LevelAsString("debug") - sets log level by string
* log.Format(func(level int, line string, message string) string) - sets function to prepare format log line
* log.SizeLimit(2 * log.megaByte) - sets log file size limit
* log.Stdout(true) - sets log output to stdout
* log.Debug("debug line") - writes message with debug data
* log.DebugFmt("debug line %d", 1) - writes message with debug data
* log.Info("info line") - writes message with information about state or similar
* log.InfoFmt("info line %d", 1) - writes message with information about state or similar
* log.Warn("warn line") - usually use to write warning message about unexpected application state (ex.: brudforce, incorrect request, bad loging&password authorization) 
* log.WarnFmt("warn line %d", 1) - usually use to write warning message about unexpected application state (ex.: brudforce, incorrect request, bad loging&password authorization) 
* log.Error("error line") - use only in a case of a return error what doesn't effect application run
* log.ErrorFmt("error line %d", 1) - use only in a case of a return error what doesn't effect application run
* log.Fatal("fatal line") - use only in a case of a return error what do effect application run
* log.FatalFmt("fatal line %d", 1) - use only in a case of a return error what do effect application run