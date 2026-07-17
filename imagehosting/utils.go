// 辅助函数
package imagehosting

import "encoding/json"

// jsonUnmarshal 封装 json.Unmarshal 用于跨文件调用
func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
