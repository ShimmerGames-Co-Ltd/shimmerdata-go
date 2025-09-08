package shimmerdata

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	shimmerdata_go "github.com/ShimmerGames-Co-Ltd/shimmerdata-go"
)

// SDBatchConsumer 通过HTTP协议上报日志
type SDBatchConsumer struct {
	conf            SDBatchConfig //启动配置
	logPrinter      *printer      //日志打印
	count           int64         //统计总数
	countSend       int64         //统计发送总数
	ticker          *time.Ticker  //定时器
	buffer          *SafeList     //日志缓存
	listener        chan *Data    //日志通道
	watchFlushForce atomic.Int64  //强制发送信号监听
	watchFlush      atomic.Int64  //非强制发送信号监听
	watchStop       chan struct{} //发送进程退出
	stopped         chan struct{} //关闭信号
	dirWatchStop    chan struct{} //文件监听关闭信号
	dirWatchStopped chan struct{} //文件监听关闭信号
}

// SDBatchConfig 启动配置参数
type SDBatchConfig struct {
	TempDir   string        // 用于日志无法正常发送到HTTP服务器时缓存日志的本地目录，如果为空则丢弃无法发送的日志
	ServerUrl string        // HTTP服务器地址
	AppId     string        // appId 需要先向日志接收服务器注册
	AppToken  string        // appToken 向日志接收服务器注册后获得
	BatchSize int           // 一次打包传输的对象个数
	Timeout   time.Duration // http 请求超时时间
	Compress  bool          // 是否允许使用gzip压缩http数据
	Interval  int           // 自动发送间隔时间 (秒)
}

type request struct {
	App      string `json:"app"`
	Token    string `json:"token"`
	SDK      string `json:"sdk"`
	Version  string `json:"version"`
	Compress bool   `json:"compress"`
	Size     int64  `json:"size"`
	Log      []byte `json:"log"`
}

const (
	DefaultTimeOut   = 30000
	DefaultBatchSize = 20
	MaxBatchSize     = 200
	DefaultInterval  = 30
)

func NewBatchConsumer(config SDBatchConfig) (SDConsumer, error) {
	if config.ServerUrl == "" {
		msg := fmt.Sprint("ServerUrl can not be empty")
		sdLogInfo(msg)
		return nil, errors.New(msg)
	}

	var batchSize int
	if config.BatchSize > MaxBatchSize {
		batchSize = MaxBatchSize
	} else if config.BatchSize <= 0 {
		batchSize = DefaultBatchSize
	} else {
		batchSize = config.BatchSize
	}
	if config.Timeout <= 0 {
		config.Timeout = time.Duration(DefaultTimeOut) * time.Millisecond
	}
	var interval int
	if config.Interval == 0 {
		interval = DefaultInterval
	} else {
		interval = config.Interval
	}
	c := &SDBatchConsumer{
		conf:            config,
		ticker:          time.NewTicker(time.Duration(interval) * time.Second),
		buffer:          NewSafeList(),
		listener:        make(chan *Data, batchSize*2),
		watchFlushForce: atomic.Int64{},
		watchFlush:      atomic.Int64{},
		watchStop:       make(chan struct{}),
		stopped:         make(chan struct{}),
		dirWatchStop:    make(chan struct{}),
		dirWatchStopped: make(chan struct{}),
	}
	if config.TempDir != "" {
		abs, err := checkAndMakeFolder(config.TempDir)
		if err != nil {
			return nil, err
		}
		config.TempDir = abs
		p := newPrinter(&printerConf{
			app:        config.AppId,
			folder:     config.TempDir,
			maxSize:    100,
			maxAge:     0,
			maxBackups: 0,
			compress:   true,
		})
		c.logPrinter = p
		c.conf.TempDir = config.TempDir
		c.watchDir()
	}
	c.listen()
	go func() {
		defer func() {
			c.stopped <- struct{}{}
		}()
		for {
			select {
			case <-c.watchStop: //退出前强制将所有日志发送到服务器
				sdLogInfo("batch consumer watcher stopping......")
				//强制将所有数据发送到服务器
				_ = c.innerFlush(true)
				c.watchFlushForce.Store(0)
				c.watchFlush.Store(0)
				sdLogInfo("batch consumer stopped send log count:%d", c.countSend)
				return
			case <-c.ticker.C: //定时传输日志
				sdLogInfo("ticker flush at:%s", time.Now().Format(time.RFC3339))
				//清空计数值
				c.watchFlushForce.Store(0)
				c.watchFlush.Store(0)
				//发送数据
				_ = c.innerFlush(true)
			default: //合批发送
				force := c.watchFlushForce.Swap(0)
				notForce := c.watchFlush.Swap(0)
				if force > 0 {
					sdLogInfo("force flush count:%d at:%s", force, time.Now().Format(time.RFC3339))
					//合批发送
					_ = c.innerFlush(true)
				} else if notForce > 0 {
					sdLogInfo("not force flush count:%d at:%s", notForce, time.Now().Format(time.RFC3339))
					//合批发送
					_ = c.innerFlush(false)
				} else {
					//减少空转
					time.Sleep(10 * time.Millisecond)
				}
			}
		}
	}()

	sdLogInfo("Mode: batch consumer, appId: %s, serverUrl: %s", c.conf.AppId, c.conf.ServerUrl)

	return c, nil
}

// listen 监听日志写入
func (c *SDBatchConsumer) listen() {
	go func() {
		for {
			select {
			case d, ok := <-c.listener:
				if !ok {
					sdLogInfo("batch consumer listener stopping......")
					//关闭信道，准备退出。先关闭定时器，再关闭发送监听信道。
					c.ticker.Stop()
					close(c.dirWatchStop)
					<-c.dirWatchStopped
					close(c.watchStop)
					return
				}
				atomic.AddInt64(&c.count, 1)
				c.buffer.PushBack(d)
				//合批发送
				if c.buffer.Len() >= c.conf.BatchSize {
					c.watchFlush.Add(1) //非强制分割，根据情况分割
				}
			}
		}
	}()

}

// watchDir 定时检查日志保存文件夹，上传日志文件
func (c *SDBatchConsumer) watchDir() {
	go func() {
		d := time.Duration(c.conf.Interval) * time.Second
		ticker := time.NewTicker(d)
		for {
			select {
			case <-ticker.C: //定时传输日志
				slog.Info("watchDir ticker, start processPath upload logFile")
				lineCount, err := c.logPrinter.LogLine()
				if err != nil {
					sdLogError("watchDir ticker read file line failed error:%s", err.Error())
					_ = c.logPrinter.ForceRotate() //强制切割
					time.Sleep(3 * time.Second)    //等待日志文件压缩完成
				} else if lineCount > 0 {
					_ = c.logPrinter.ForceRotate() //强制切割
					time.Sleep(3 * time.Second)    //等待日志文件压缩完成
				}
				c.processPath()
				ticker.Reset(d)
			case <-c.dirWatchStop:
				slog.Info("batch consumer watchDir stopping......")
				ticker.Stop()
				lineCount, err := c.logPrinter.LogLine()
				if err != nil {
					sdLogError("watchDir ticker read file line failed error:%s", err.Error())
					_ = c.logPrinter.Rotate()   //强制切割
					time.Sleep(3 * time.Second) //等待日志文件压缩完成
				} else if lineCount > 0 {
					_ = c.logPrinter.Rotate()   //强制切割
					time.Sleep(3 * time.Second) //等待日志文件压缩完成
				}
				c.processPath()
				c.dirWatchStopped <- struct{}{}
				return
			}
		}
	}()
}

func (c *SDBatchConsumer) Add(d Data) error {
	c.listener <- &d
	sdLogInfo("Enqueue event data: %v", d)

	return nil
}

func (c *SDBatchConsumer) Flush() error {
	c.watchFlushForce.Add(1)
	sdLogInfo("flush data")
	return nil
}

// pack 打包数据，准备发送
func (c *SDBatchConsumer) pack() (*bytes.Buffer, int, error) {
	b := bytes.NewBuffer([]byte{})
	size := 0
	for size < c.conf.BatchSize {
		data, ok := c.buffer.PopFront()
		if !ok {
			break
		}
		bs, err := json.Marshal(data)
		if err != nil {
			return nil, 0, err
		}
		size += 1
		b.Write(bs)
		b.Write([]byte("\n"))
	}

	return b, size, nil
}

func (c *SDBatchConsumer) innerFlush(force bool) error {
	//没有数据时直接返回
	if c.buffer.Len() == 0 {
		return nil
	}
	if c.buffer.Len() < c.conf.BatchSize && !force {
		return nil
	}
	b, size, err := c.pack()
	if err != nil {
		return err
	}
	atomic.AddInt64(&c.countSend, int64(size))
	params := parseTime(b.Bytes())
	for i := 0; i < 3; i++ {
		err = c.send(params, size)
		if err != nil {
			sdLogError("consumer batch send to http failed error:%s", err.Error())
			if i == 2 {
				c.writeFile(params)
				return err
			}
		} else {
			return nil
		}
	}

	return c.innerFlush(force)
}

func (c *SDBatchConsumer) writeFile(data []byte) {
	if c.logPrinter == nil {
		return
	}
	_, err := c.logPrinter.Write(data)
	if err != nil {
		sdLogError("SDBatchConsumer writeFile error:%s", err.Error())
	}
}

func (c *SDBatchConsumer) Close() error {
	sdLogInfo("batch consumer stopping....... log count=%d", c.count)
	close(c.listener)
	<-c.stopped
	if c.logPrinter != nil {
		_ = c.logPrinter.Close()
	}

	return nil
}

func (c *SDBatchConsumer) IsStringent() bool {
	return false
}

func (c *SDBatchConsumer) send(data []byte, size int) (err error) {
	var encodedData []byte
	if c.conf.Compress {
		encodedData, err = encodeData(data)
	} else {
		encodedData = data
	}
	if err != nil {
		return err
	}
	r := &request{
		App:      c.conf.AppId,
		Token:    c.conf.AppToken,
		SDK:      "go-sdk",
		Version:  shimmerdata_go.Version,
		Compress: c.conf.Compress,
		Size:     int64(size),
		Log:      encodedData,
	}
	reqData, err := json.Marshal(r)
	if err != nil {
		return err
	}
	postData := bytes.NewBuffer(reqData)

	var resp *http.Response
	req, _ := http.NewRequest("POST", c.conf.ServerUrl+"/LogServer/log/report", postData)
	client := &http.Client{Timeout: c.conf.Timeout}
	resp, err = client.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code int
		Msg  string
	}
	if len(body) > 0 {
		err = json.Unmarshal(body, &result)
		if err != nil {
			return err
		}
	}
	if http.StatusOK == resp.StatusCode {
		if result.Code != 0 {
			return fmt.Errorf("httpStatus:%d, Code:%d Msg:%s", resp.StatusCode, result.Code, result.Msg)
		} else {
			return nil
		}
	}

	return fmt.Errorf("httpStatus:%d, Code:%d Msg:%s", resp.StatusCode, result.Code, result.Msg)
}

// processPath 遍历文件夹，解析所有文件并上传
func (c *SDBatchConsumer) processPath() {
	fileDir := c.conf.TempDir
	info, err := os.Stat(fileDir)
	if err != nil {
		sdLogError("state tempDir error:%s", err.Error())
		return
	}
	if info.IsDir() {
		files, err := os.ReadDir(fileDir)
		if err != nil {
			sdLogError("read tempDir error:%s", err.Error())
			return
		}
		for _, file := range files {
			if file.Name() == filepath.Base(c.logPrinter.conf.filename) {
				continue
			}
			filePath := filepath.Join(fileDir, file.Name())
			if !file.IsDir() {
				//文件
				err = c.uploadFile(filePath)
				if err != nil {
					sdLogError("processPath uploadFile error:%s", err.Error())
					return
				}
				//删除文件
				err = os.Remove(filePath)
				if err != nil {
					sdLogError("processPath remove file:%s error:%s", filePath, err.Error())
					return
				}
			}
		}
	}
}

// Gzip
func encodeData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)

	_, err := gw.Write(data)
	if err != nil {
		_ = gw.Close()
		return nil, err
	}
	_ = gw.Close()

	return buf.Bytes(), nil
}
