package shimmerdata

import (
	"fmt"
	"gopkg.in/natefinch/lumberjack.v2"
	"os/exec"
	"strconv"
	"strings"
)

type printerConf struct {
	app        string //APP
	folder     string //日志存放文件夹
	maxSize    int    //文件大小（默认100MB）
	maxAge     int    //文件保留时长（单位天，默认是一直保留）
	maxBackups int    //最大文件个数（0=默认不限制）
	compress   bool   //是否压缩日志（gzip）
	filename   string //日志文件名
}

type printer struct {
	lumberjack.Logger
	conf *printerConf
}

func newPrinter(conf *printerConf) *printer {
	filename := fmt.Sprintf("%s/%s-logback.log", conf.folder, conf.app)
	conf.filename = filename
	p := &printer{
		Logger: lumberjack.Logger{
			Filename:   filename,
			MaxSize:    conf.maxSize,
			MaxAge:     conf.maxAge,
			MaxBackups: conf.maxBackups,
			LocalTime:  false,
			Compress:   conf.compress,
		},
		conf: conf,
	}
	return p
}

func (p *printer) Print(data []byte) error {
	_, err := p.Write(data)
	return err
}

// ForceRotate 强制切分文件。注意：如果切分后立即关闭printer会导致日志文件不能被压缩，因为lumberjack的压缩是异步的。
func (p *printer) ForceRotate() error {
	return p.Rotate()
}

// LogLine 查看日志条数
func (p *printer) LogLine() (int64, error) {
	cmd := exec.Command("wc", "-l", p.Logger.Filename)
	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found in $PATH") {
			return 0, nil
		}
		return 0, err
	}
	// 输出是以空格分隔的（行数和文件名），我们只需要行数部分
	lineCountStr := strings.Fields(string(output))[0]
	lineCount, err := strconv.ParseInt(lineCountStr, 10, 64)
	if err != nil {
		return 0, err
	}

	return lineCount, nil
}
