package model

import (
	"bytes"
	"encoding/json"
	"strings"
)

// CustomJSONEncoder 自定义JSON编码器，确保URL不被转义
func CustomJSONEncoder(v interface{}) ([]byte, error) {
	// 先使用标准JSON编码
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	// 将Unicode转义的&字符替换回原始&字符
	// 注意：这里我们只替换URL中的\u0026，不替换其他地方的转义字符
	str := string(data)
	str = strings.ReplaceAll(str, `\u0026`, "&")

	return []byte(str), nil
}

// EncodeDownloadURLResponse 编码下载URL响应，确保URL不被转义
func EncodeDownloadURLResponse(response *DownloadURLResponse) ([]byte, error) {
	// 创建一个临时结构体来避免循环引用
	type tempResponse struct {
		URL     string `json:"url"`
		Expires int64  `json:"expires_in_seconds"`
	}

	temp := &tempResponse{
		URL:     response.URL,
		Expires: response.Expires,
	}

	return CustomJSONEncoder(temp)
}

// EncodeAPIResponse 编码API响应，确保URL不被转义
func EncodeAPIResponse(response *APIResponse) ([]byte, error) {
	// 如果响应数据是DownloadURLResponse类型，特殊处理
	if downloadResp, ok := response.Data.(*DownloadURLResponse); ok {
		downloadData, err := EncodeDownloadURLResponse(downloadResp)
		if err != nil {
			return nil, err
		}

		// 手动构建响应
		var buf bytes.Buffer
		buf.WriteString(`{"success":true,"data":`)
		buf.Write(downloadData)
		buf.WriteString(`}`)

		return buf.Bytes(), nil
	}

	// 其他响应使用标准编码
	return json.Marshal(response)
}
