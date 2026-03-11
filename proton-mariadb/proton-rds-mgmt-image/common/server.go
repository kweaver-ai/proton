package common

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	jsoniter "github.com/json-iterator/go"
)

// TimeStampToString 毫秒时间戳转RFC3339格式的字符串
func TimeStampToString(t int64) string {
	const num int64 = 1e3
	return time.Unix(t/num, 0).Format(time.RFC3339)
}

// StringToTimeStamp RFC3339格式的字符串转纳秒时间戳
func StringToTimeStamp(t string) (int64, error) {
	tt, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return 0, err
	}
	return tt.UnixNano(), nil
}

// GetJSONValue 读取请求body
func GetJSONValue(c *gin.Context, v interface{}) error {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	err = jsoniter.Unmarshal(body, v)
	if err != nil {
		return NewHTTPError(err.Error(), BadRequest, nil)
	}
	return nil
}

// ReplyOK 响应成功
func ReplyOK(c *gin.Context, statusCode int, body interface{}) {
	b := make([]byte, 0)
	if body != nil {
		b, _ = jsoniter.Marshal(body)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.String(statusCode, string(b))
}

// ReplyError 响应错误
func ReplyError(c *gin.Context, err error) {
	var statusCode int
	var body string
	switch e := err.(type) {
	case *HTTPError:
		statusCode = e.Code / 1e6
		body = e.Error()
	case ExHTTPError:
		statusCode = e.Status
		body = e.Error()
	default:
		statusCode = http.StatusInternalServerError
		body = NewHTTPError(e.Error(), InternalError, nil).Error()
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.String(statusCode, body)
}

// JSONValueDesc json格式描述
// reflect.Bool , for JSON booleans
// reflect.Float64 , for JSON numbers
// reflect.String , for JSON strings
// reflect.Slice , for JSON arrays
// reflect.Map , for JSON objects
type JSONValueDesc struct {
	Kind      reflect.Kind
	Required  bool
	Exist     bool
	ValueDesc map[string]*JSONValueDesc
}

// CheckJSONValue 检查请求参数json格式
func CheckJSONValue(key string, jsonV interface{}, jsonValueDesc *JSONValueDesc) error {
	kind := reflect.ValueOf(jsonV).Kind()
	if kind != jsonValueDesc.Kind {
		return NewHTTPError(fmt.Sprintf("type of %s should be %v", key, jsonValueDesc.Kind), BadRequest, nil)
	} else if kind == reflect.Map {
		obj := jsonV.(map[string]interface{})
		for k, valueDesc := range jsonValueDesc.ValueDesc {
			if v, ok := obj[k]; ok {
				err := CheckJSONValue(fmt.Sprintf("%s.%s", key, k), v, valueDesc)
				if err != nil {
					return err
				}
				valueDesc.Exist = true
			} else if valueDesc.Required {
				return NewHTTPError(fmt.Sprintf("%v is required", fmt.Sprintf("%s.%s", key, k)), BadRequest, nil)
			}
		}
	} else if kind == reflect.Slice {
		arr := jsonV.([]interface{})
		for i, element := range arr {
			err := CheckJSONValue(fmt.Sprintf(`%s[%d]`, key, i), element, jsonValueDesc.ValueDesc["element"])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func DecryptPwd(inData string) (out string, err error) {
	data := []byte(inData)
	dataLen := len(data)
	if 0 == dataLen {
		return out, nil
	}
	if dataLen%2 != 0 {
		return out, errors.New("invalid input string to decode")
	}

	blockNum := dataLen / 2
	outData := make([]byte, blockNum)
	for idx := 0; idx < blockNum; idx++ {
		c1 := data[idx*2]
		c2 := data[idx*2+1]

		c1 = c1 - 65
		c2 = c2 - 65
		b2 := c2*16 + c1
		b1 := b2 ^ 32
		outData[idx] = b1
	}

	out = string(outData[:])

	return out, nil
}

func GetDiskFreeSize(path string) (int64, error) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &fs)
	if err != nil {
		return 0, err
	}
	free := int64(fs.Bfree) * fs.Bsize
	return free, nil
}

func GetDiskUsedSize(path string) (int64, error) {
	var size int64 = 0
	stack := []string{path}
	for {
		if len(stack) == 0 {
			break
		}
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		f, err := os.Stat(v)
		if err != nil {
			return 0, err
		}
		size = size + f.Size()

		if f.IsDir() {
			rd, err := ioutil.ReadDir(v)
			if err != nil {
				return 0, err
			}
			for _, fi := range rd {
				stack = append(stack, filepath.Join(v, fi.Name()))
			}

		}
	}
	return size, nil
}

// 异步cmd, 可随时停止;
// exitFunc 执行完成时回调;
// cancelFunc cancel时回调;
func AsyncRun(ctx context.Context, exitFunc func(int), cancelFunc func(error), name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	c := make(chan int, 1)
	go func() {
		_ = cmd.Run()
		c <- 1
	}()
	go func() {
		select {
		case <-ctx.Done():
			err := cmd.Process.Kill()
			cancelFunc(err)
		case <-c:
			exitFunc(cmd.ProcessState.ExitCode())
		}
	}()
}

func ParseMySQLError(err error) error {
	errCode := InternalError
	if e, ok := err.(*mysql.MySQLError); ok {
		switch e.Number {
		case 1045:
			errCode = AuthenticationFailed
		case 1044:
			errCode = AccessDenied
		case 1142:
			errCode = AccessDenied
		default:
		}
	}
	newErr := NewHTTPError(err.Error(), errCode, nil)
	return newErr
}
