package shimmerdata

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"time"
)

const (
	DATE_FORMAT = "2006-01-02 15:04:05.000"
	KEY_PATTERN = "^[a-zA-Z#][A-Za-z0-9_]{0,49}$"
)

// A string of 50 letters and digits that starts with '#' or a letter
var keyPattern, _ = regexp.Compile(KEY_PATTERN)

func mergeProperties(target, source map[string]interface{}) {
	for k, v := range source {
		target[k] = v
	}
}

func extractTime(p map[string]interface{}) (string, error) {
	if t, ok := p["#time"]; ok {
		delete(p, "#time")
		switch v := t.(type) {
		case string:
			// 解析为 time.Time 对象，假设该字符串是本地时区时间
			parsedTime, err := time.Parse(DATE_FORMAT, v)
			if err != nil {
				sdLogError("#time parse error:%s", err.Error())
				return "", fmt.Errorf("#time format should be %s", DATE_FORMAT)
			}
			// 判断该时间是否是 UTC
			if !isUTC(parsedTime) {
				// 如果不是 UTC 时区，转换为 UTC
				parsedTime = parsedTime.UTC()
			}
			return parsedTime.Format(DATE_FORMAT), nil
		case time.Time:
			if !isUTC(v) {
				v = v.UTC()
			}
			return v.Format(DATE_FORMAT), nil
		default:
			return time.Now().UTC().Format(DATE_FORMAT), nil
		}
	}

	return time.Now().UTC().Format(DATE_FORMAT), nil
}

// 判断 time.Time 是否为 UTC 时区
func isUTC(t time.Time) bool {
	return t.Location() == time.UTC
}

func extractStringProperty(p map[string]interface{}, key string) string {
	if t, found := p[key]; found {
		delete(p, key)
		v, ok := t.(string)
		if !ok {
			sdLogError("Invalid data type of key:%s, value:%v", key, v)
		}
		return v
	}
	return ""
}

func isNotNumber(v interface{}) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
	case float32, float64:
	default:
		return true
	}
	return false
}

func formatProperties(d *Data, ta *SDAnalytics) error {

	if d.EventName != "" {
		matched := checkPattern([]byte(d.EventName))
		if !matched {
			msg := "invalid event name: " + d.EventName
			sdLogInfo(msg)
			return errors.New(msg)
		}
	}

	if d.Properties != nil {
		for k, v := range d.Properties {
			if ta.consumer.IsStringent() {
				isMatch := checkPattern([]byte(k))
				if !isMatch {
					msg := "invalid property key: " + k
					sdLogInfo(msg)
					return errors.New(msg)
				}
			}

			if d.Type == UserAdd && isNotNumber(v) {
				msg := "invalid property value: only numbers is supported by UserAdd"
				sdLogInfo(msg)
				return errors.New(msg)
			}

			// check value
			switch v.(type) {
			case int:
			case bool:
			case float64:
			case string:
			case time.Time:
				d.Properties[k] = v.(time.Time).Format(DATE_FORMAT)
			case []string:
				d.IsComplex = true
			default:
				d.IsComplex = true
			}
		}
	}

	return nil
}

func isNotArrayOrSlice(v interface{}) bool {
	typeOf := reflect.TypeOf(v)
	switch typeOf.Kind() {
	case reflect.Array:
	case reflect.Slice:
	default:
		return true
	}
	return false
}

func checkPattern(name []byte) bool {
	return keyPattern.Match(name)
}

func parseTime(input []byte) []byte {
	var re = regexp.MustCompile(`"((\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2})(?:\.(\d{3}))\d*)(Z|[\+-]\d{2}:\d{2})"`)
	var substitution = "\"$2 $3.$4\""

	for re.Match(input) {
		input = re.ReplaceAll(input, []byte(substitution))
	}
	return input
}

func generateUUID() string {
	newUUID, err := uuid.NewUUID()
	if err != nil {
		return ""
	}
	return newUUID.String()
}

func checkAndMakeFolder(folder string) (string, error) {
	// 转换为绝对路径
	absPath, err := filepath.Abs(folder)
	if err != nil {
		return "", err
	}
	if !folderExists(absPath) {
		err = createFolder(absPath)
		if err != nil {
			return "", err
		}
	}

	return absPath, nil
}

func createFolder(path string) error {
	return os.MkdirAll(path, os.ModePerm)
}

func folderExists(folderPath string) bool {
	// 使用 os.Stat 检查路径信息
	info, err := os.Stat(folderPath)
	if os.IsNotExist(err) {
		return false // 文件夹不存在
	}
	return info.IsDir() // 判断路径是否为文件夹
}
