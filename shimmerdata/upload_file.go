package shimmerdata

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	shimmerdata_go "github.com/ShimmerGames-Co-Ltd/shimmerdata-go"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type LogFileUploadReq struct {
	App      string `json:"app"`      //app id
	Token    string `json:"token"`    //app token
	Sdk      string `json:"sdk"`      //sdk类型
	Version  string `json:"version"`  //sdk版本
	Compress bool   `json:"compress"` //是否使用gzip压缩
	Md5      string `json:"md5"`      //日志文件的MD5
	Filename string `json:"filename"` //文件名
	Start    int64  `json:"start"`    //日志写入起始位置
	End      int64  `json:"end"`      //最后一块文件索引
	Total    int64  `json:"total"`    //总块数
	Content  []byte `json:"content"`  // 文件块内容
}

const chunkSize int64 = 1024 * 1024 // 每块 1MB

// uploadFile 上传文件
func (c *SDBatchConsumer) uploadFile(fileDir string) error {
	// 打开文件
	file, err := os.Open(fileDir)
	if err != nil {
		return fmt.Errorf("uploadFile open file error: %s", err.Error())
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("无法获取文件信息: %s", err.Error())
	}
	fileSize := fileInfo.Size()
	md5Str, err := fileMD5(file)
	if err != nil {
		return err
	}
	filename := filepath.Base(fileDir)
	compress := false
	if filepath.Ext(fileDir) == ".gz" {
		compress = true
	}

	in := &LogFileUploadReq{
		App:      c.conf.AppId,
		Token:    c.conf.AppToken,
		Sdk:      "go-sdk",
		Version:  shimmerdata_go.Version,
		Compress: compress,
		Md5:      md5Str,
		Filename: filename,
		Total:    fileSize,
	}
	var uploadedBytes int64 = 0 // 已上传的字节数
	for uploadedBytes < fileSize {
		// 计算当前块大小
		remaining := fileSize - uploadedBytes
		currentChunkSize := chunkSize
		if remaining < chunkSize {
			currentChunkSize = remaining
		}

		// 读取当前块数据
		buffer := make([]byte, currentChunkSize)
		_, err = file.ReadAt(buffer, uploadedBytes)
		if err != nil && err != io.EOF {
			return fmt.Errorf("uploadFile read file error: %s", err.Error())
		}
		in.Start = uploadedBytes
		in.End = uploadedBytes + currentChunkSize
		in.Content = buffer

		inDate, err := json.Marshal(in)
		if err != nil {
			return err
		}

		// 准备 HTTP 请求
		req, err := http.NewRequest("POST", c.conf.ServerUrl+"/LogServer/log/upload", bytes.NewReader(inDate))
		if err != nil {
			return fmt.Errorf("uploadFile create POST request error: %s", err.Error())
		}

		// 执行请求
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("uploadFile POST error: %s", err.Error())
		}

		// 检查响应状态
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

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
			var errStr string
			if result.Code != 0 {
				errStr = fmt.Sprintf("httpStatus:%d, Code:%d Msg:%s", resp.StatusCode, result.Code, result.Msg)
			}

			return fmt.Errorf("uploadFile failed:%s", errStr)
		}

		resp.Body.Close()
		// 更新已上传的字节数
		uploadedBytes += currentChunkSize
		sdLogInfo("%s have upload: %d/%d Bytes\n", fileDir, uploadedBytes, fileSize)
	}

	sdLogInfo("upload log file:%s success", fileDir)
	return nil
}

func fileMD5(file *os.File) (string, error) {
	// 创建 MD5 哈希器
	hash := md5.New()
	// 将文件内容拷贝到哈希器
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	// 计算哈希值并转换为字符串
	return hex.EncodeToString(hash.Sum(nil)), nil
}
