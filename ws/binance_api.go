package ws

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/levigross/grequests"
)

func GetListenKey(host, apiKey, apiSecret string) (listenKey string, err error) {
	hash := hmac.New(sha512.New, []byte(apiSecret))
	hash.Write([]byte(fmt.Sprintf("timestamp=%d", time.Now().UnixMilli())))

	requestBody := map[string]interface{}{
		"timestamp": time.Now().UnixMilli(),
		"signature": hex.EncodeToString(hash.Sum(nil)),
	}

	requestOptions := &grequests.RequestOptions{
		JSON:    requestBody,
		Headers: map[string]string{"X-MBX-APIKEY": apiKey},
	}

	// 发送HTTP POST请求以获取listenKey
	url := host + "/fapi/v1/listenKey"
	resp, err := grequests.Post(url, requestOptions)
	if err != nil {
		return
	}

	// 检查响应状态码
	if resp.StatusCode != 200 {
		err = fmt.Errorf("GetListenKey HTTP request failed with status code: %d, msg: %s", resp.StatusCode, string(resp.Bytes()))
		return
	}

	// 解析JSON响应以获取listenKey
	var response map[string]interface{}
	if err = resp.JSON(&response); err != nil {
		err = fmt.Errorf("GetListenKey error decoding JSON response:%v", err)
		return
	}

	listenKey, ok := response["listenKey"].(string)
	if !ok {
		err = fmt.Errorf("GetListenKey failed to extract listenKey from the response")
		return
	}

	return listenKey, nil
}

func RefreshListenKey(host, apiKey string) (err error) {
	requestOptions := &grequests.RequestOptions{
		Headers: map[string]string{"X-MBX-APIKEY": apiKey},
	}

	// 发送HTTP PUT 请求以延长 listenKey 时间
	url := host + "/fapi/v1/listenKey"
	resp, err := grequests.Put(url, requestOptions)
	if err != nil {
		return
	}

	// 检查响应状态码
	if resp.StatusCode != 200 {
		err = fmt.Errorf("RefreshListenKey HTTP request failed with status code: %d, msg: %s", resp.StatusCode, string(resp.Bytes()))
		return
	}

	return nil
}
