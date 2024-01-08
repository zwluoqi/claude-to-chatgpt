package service

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/oldweipro/claude-to-chatgpt/global"
	"strings"
)

// 声明一个map，键类型为string，值类型为int
// Correct package-level map declaration
var stringsBearerMap = make(map[string]int)

func GetSessionKey(c *gin.Context) (sk string) {
	auth := c.Request.Header.Get("Authorization")
	hasPrefix := strings.HasPrefix(auth, "Bearer ")

	apikey := ""
	if hasPrefix && len(auth) > 7 {
		apikey = auth[7:]
	}
	if apikey != "" {
		// Check if the map already has this key
		if index, exists := stringsBearerMap[apikey]; exists { // Error occurs here
			// If it exists, increment the index value
			stringsBearerMap[apikey] = index + 1 // And here
		} else {
			// If not, add the key and set the index to 1
			stringsBearerMap[apikey] = 0 // And here
		}

		// 使用逗号作为分隔符来分割字符串
		parts := strings.Split(apikey, ",")
		index := stringsBearerMap[apikey] % len(parts)
		sk = strings.TrimSpace(parts[index])
		if strings.HasPrefix(sk, "sk-ant-sid") {
			sk = "sessionKey=" + sk
		}
	}
	if sk == "" {
		sk = global.ServerConfig.Claude.GetSessionKey()
	}
	fmt.Println(sk)

	return sk
}
