package shimmerdata

import (
	"fmt"
	"time"
)

// SdkLogPrefix const
const SdkLogPrefix = "[ShimmerData]"

var logInstance SDLogger

type SDLogLevel int32

const (
	SDLogLevelOff     SDLogLevel = 1
	SDLogLevelError   SDLogLevel = 2
	SDLogLevelWarning SDLogLevel = 3
	SDLogLevelInfo    SDLogLevel = 4
	SDLogLevelDebug   SDLogLevel = 5
)

// default is SDLogLevelOff
var currentLogLevel = SDLogLevelOff

// SDLogger User-defined log classes must comply with interface
type SDLogger interface {
	Print(message string)
}

// SetLogLevel Set the log output level
func SetLogLevel(level SDLogLevel) {
	if level < SDLogLevelOff || level > SDLogLevelDebug {
		fmt.Println(SdkLogPrefix + "log type error")
		return
	} else {
		currentLogLevel = level
	}
}

// SetCustomLogger Set a custom log input class, usually you don't need to set it up.
func SetCustomLogger(logger SDLogger) {
	if logger != nil {
		logInstance = logger
	}
}

func sdLog(level SDLogLevel, format string, v ...interface{}) {
	if level > currentLogLevel {
		return
	}

	var modeStr string
	switch level {
	case SDLogLevelError:
		modeStr = "[Error] "
		break
	case SDLogLevelWarning:
		modeStr = "[Warning] "
		break
	case SDLogLevelInfo:
		modeStr = "[Info] "
		break
	case SDLogLevelDebug:
		modeStr = "[Debug] "
		break
	default:
		modeStr = "[Info] "
		break
	}

	if logInstance != nil {
		msg := fmt.Sprintf(SdkLogPrefix+modeStr+format+"\n", v...)
		logInstance.Print(msg)
	} else {
		logTime := fmt.Sprintf("[%v]", time.Now().Format("2006-01-02 15:04:05.000"))
		fmt.Printf(logTime+SdkLogPrefix+modeStr+format+"\n", v...)
	}
}

func sdLogDebug(format string, v ...interface{}) {
	sdLog(SDLogLevelDebug, format, v...)
}

func sdLogInfo(format string, v ...interface{}) {
	sdLog(SDLogLevelInfo, format, v...)
}

func sdLogError(format string, v ...interface{}) {
	sdLog(SDLogLevelError, format, v...)
}

func sdLogWarning(format string, v ...interface{}) {
	sdLog(SDLogLevelWarning, format, v...)
}
