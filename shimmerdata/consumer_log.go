package shimmerdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

type RotateMode int32

const (
	DefaultChannelSize            = 1000 // channel size
	RotateDaily        RotateMode = 0    // by the day
	RotateHourly       RotateMode = 1    // by the hour
)

// SDLogConsumer write data to file, it works with LogBus
type SDLogConsumer struct {
	directory      string   // directory of log file
	dateFormat     string   // name format of log file
	fileSize       int64    // max size of single log file (MByte)
	fileNamePrefix string   // prefix of log file
	currentFile    *os.File // current file handler
	wg             sync.WaitGroup
	ch             chan []byte
	mutex          *sync.RWMutex
	sdkClose       bool
}

type SDLogConsumerConfig struct {
	Directory      string     // directory of log file
	RotateMode     RotateMode // rotate mode of log file
	FileSize       int        // max size of single log file (MByte)
	FileNamePrefix string     // prefix of log file
	ChannelSize    int
}

func NewLogConsumer(directory string, r RotateMode) (SDConsumer, error) {
	return NewLogConsumerWithFileSize(directory, r, 0)
}

// NewLogConsumerWithFileSize init SDLogConsumer
// directory: directory of log file
// r: rotate mode of log file. (in days / hours)
// size: max size of single log file (MByte)
func NewLogConsumerWithFileSize(directory string, r RotateMode, size int) (SDConsumer, error) {
	config := SDLogConsumerConfig{
		Directory:  directory,
		RotateMode: r,
		FileSize:   size,
	}
	return NewLogConsumerWithConfig(config)
}

func NewLogConsumerWithConfig(config SDLogConsumerConfig) (SDConsumer, error) {
	var df string
	switch config.RotateMode {
	case RotateDaily:
		df = "2006-01-02"
	case RotateHourly:
		df = "2006-01-02-15"
	default:
		errStr := "unknown rotate mode"
		sdLogInfo(errStr)
		return nil, errors.New(errStr)
	}

	chanSize := DefaultChannelSize
	if config.ChannelSize > 0 {
		chanSize = config.ChannelSize
	}

	c := &SDLogConsumer{
		directory:      config.Directory,
		dateFormat:     df,
		fileSize:       int64(config.FileSize * 1024 * 1024),
		fileNamePrefix: config.FileNamePrefix,
		wg:             sync.WaitGroup{},
		ch:             make(chan []byte, chanSize),
		mutex:          new(sync.RWMutex),
		sdkClose:       false,
	}

	return c, c.init()
}

func (c *SDLogConsumer) Add(d Data) error {
	var err error = nil
	c.mutex.Lock()
	defer func() {
		c.mutex.Unlock()
	}()
	if c.sdkClose {
		err = errors.New("add event failed, SDK has been closed")
		sdLogError(err.Error())
	} else {
		jsonBytes, jsonErr := json.Marshal(d)
		if jsonErr != nil {
			err = jsonErr
		} else {
			c.ch <- jsonBytes
		}
	}
	return err
}

func (c *SDLogConsumer) Flush() error {
	sdLogInfo("flush data")
	var err error = nil
	c.mutex.Lock()
	if c.currentFile != nil {
		err = c.currentFile.Sync()
	}
	c.mutex.Unlock()
	return err
}

func (c *SDLogConsumer) Close() error {
	sdLogInfo("log consumer close")

	var err error = nil
	c.mutex.Lock()
	if c.sdkClose {
		err = errors.New("[ShimmerData][error]: SDK has been closed")
	} else {
		close(c.ch)
		c.wg.Wait()
		if c.currentFile != nil {
			_ = c.currentFile.Sync()
			err = c.currentFile.Close()
			c.currentFile = nil
		}
	}
	c.sdkClose = true
	c.mutex.Unlock()
	return err
}

func (c *SDLogConsumer) IsStringent() bool {
	return false
}

func (c *SDLogConsumer) constructFileName(timeStr string, i int) string {
	fileNamePrefix := ""
	if len(c.fileNamePrefix) != 0 {
		fileNamePrefix = c.fileNamePrefix + "."
	}
	// is need paging
	if c.fileSize > 0 {
		return fmt.Sprintf("%s/%slog.%s_%d", c.directory, fileNamePrefix, timeStr, i)
	} else {
		return fmt.Sprintf("%s/%slog.%s", c.directory, fileNamePrefix, timeStr)
	}
}

func (c *SDLogConsumer) init() error {
	fd, err := c.initLogFile()
	if err != nil {
		sdLogError("init log file failed: %s", err.Error())
		return err
	}
	c.currentFile = fd

	c.wg.Add(1)

	go func() {
		defer func() {
			c.wg.Done()
		}()
		for {
			select {
			case rec, ok := <-c.ch:
				if !ok {
					return
				}
				jsonStr := parseTime(rec)
				sdLogInfo("write event data: %s", jsonStr)
				c.writeToFile(string(jsonStr))
			}
		}
	}()

	sdLogInfo("Mode: log consumer, log path: " + c.directory)

	return nil
}

func (c *SDLogConsumer) initLogFile() (*os.File, error) {
	_, err := os.Stat(c.directory)
	if err != nil && os.IsNotExist(err) {
		e := os.MkdirAll(c.directory, os.ModePerm)
		if e != nil {
			return nil, e
		}
	}
	timeStr := time.Now().UTC().Format(c.dateFormat)
	return os.OpenFile(c.constructFileName(timeStr, 0), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
}

var logFileIndex = 0

func (c *SDLogConsumer) writeToFile(str string) {
	timeStr := time.Now().UTC().Format(c.dateFormat)
	// paging by Rotate Mode and current file size
	var newName string
	fName := c.constructFileName(timeStr, logFileIndex)

	if c.currentFile == nil {
		var openFileErr error
		c.currentFile, openFileErr = os.OpenFile(fName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
		if openFileErr != nil {
			sdLogInfo("open log file failed: %s\n", openFileErr)
			return
		}
	}

	if c.currentFile.Name() != fName {
		newName = fName
	} else if c.fileSize > 0 {
		stat, _ := c.currentFile.Stat()
		if stat.Size() > c.fileSize {
			logFileIndex++
			newName = c.constructFileName(timeStr, logFileIndex)
		}
	}
	if newName != "" {
		err := c.currentFile.Close()
		if err != nil {
			sdLogInfo("close file failed: %s", err.Error())
			return
		}
		c.currentFile, err = os.OpenFile(fName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
		if err != nil {
			sdLogInfo("rotate log file failed: %s", err.Error())
			return
		}
	}
	_, err := fmt.Fprintln(c.currentFile, str)
	if err != nil {
		sdLogInfo("LoggerWriter(%q): %s", c.currentFile.Name(), err.Error())
		return
	}
}
