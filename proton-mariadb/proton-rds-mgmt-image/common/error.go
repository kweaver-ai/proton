package common

import (
	"fmt"

	jsoniter "github.com/json-iterator/go"
)

var (
	// Languages 支持的语言
	Languages = [3]string{"zh_CN", "zh_TW", "en_US"}
)

var (
	i18n         = make(map[int]map[string]string)
	code2Message = make(map[int]string)
)

// SetLang 设置语言
func SetLang(lang string) {
	valid := false
	for _, l := range Languages {
		if l == lang {
			valid = true
		}
	}
	if !valid {
		panic("invalid lang")
	}
	for code := range i18n {
		code2Message[code] = i18n[code][lang]
	}
}

// Register 注册code对应message
func Register(langRes map[int]map[string]string) {
	for code, message := range langRes {
		if _, ok := i18n[code]; ok {
			panic(fmt.Sprintf("duplicate code: %v", code))
		}
		i18n[code] = make(map[string]string)
		for _, lang := range Languages {
			if m, ok := message[lang]; ok {
				i18n[code][lang] = m
			} else {
				panic(fmt.Sprintf("language %v not exists", lang))
			}
		}
	}
}

// HTTPError 服务错误结构体
type HTTPError struct {
	Cause   string                 `json:"cause"`
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Detail  map[string]interface{} `json:"detail,omitempty"`
}

// NewHTTPError 新建一个HTTPError
func NewHTTPError(cause string, code int, detail map[string]interface{}) *HTTPError {
	return &HTTPError{
		Cause:   cause,
		Code:    code,
		Message: code2Message[code],
		Detail:  detail,
	}
}

func (err HTTPError) Error() string {
	errstr, _ := jsoniter.Marshal(err)
	return string(errstr)
}

// ExHTTPError 其他服务响应的错误结构体
type ExHTTPError struct {
	Status int
	Body   []byte
}

func (err ExHTTPError) Error() string {
	return string(err.Body)
}
