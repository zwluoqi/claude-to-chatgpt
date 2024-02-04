package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	http2 "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/tls-client"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oldweipro/claude-to-chatgpt/global"
	"github.com/oldweipro/claude-to-chatgpt/model"
	// "github.com/replit/database-go"
	"io"
	"io/ioutil"
	"math/rand"
	"time"
)

var (
	jar     = tls_client.NewCookieJar()
	options = []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(3600),
		tls_client.WithClientProfile(tls_client.Safari_Ipad_15_6),
		tls_client.WithNotFollowRedirects(),
		// create cookieJar instance and pass it as argument
		tls_client.WithCookieJar(jar),
		// Disable SSL verification
		tls_client.WithInsecureSkipVerify(),
	}
	client, _ = tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
)

// 定义ErrorDetail结构体来映射错误的详细信息
type ErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// 定义ErrorResponse结构体来映射整个错误响应
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

func RequestClaudeToResponse(c *gin.Context, params *model.ChatMessageRequest, stream bool) {
	sessionKey := GetSessionKey(c)
	organization, err := GetUUIDAndModel(sessionKey)
	if err != nil {
		HandleErrorResponse(c, err.Error())
		return
	}
	if organization.ConsumerBanned {
		HandleErrorResponse(c, "账号已被ban")
		return
	}
	// converations, err := GetChatConversations(sessionKey)
	// if err != nil {
	// 	HandleErrorResponse(c, err.Error())
	// 	return
	// }
	// for _, cconveration := range converations {
	// 	DeleteChatConversations(cconveration.UUID, sessionKey)
	// }

	params.Model = organization.ClaudeModel

	randomUrl := getRandomUrl()
	// randomUrl := "https://claude.ai"

	// 设置两个参数
	newStringUuid := uuid.NewString()
	// randomUrl = getRandomUrl()
	_, err = CreateChatConversations(randomUrl, newStringUuid, sessionKey)
	if err != nil {
		HandleErrorResponse(c, err.Error())
		return
	}

	appendMessageApi := fmt.Sprintf("%s/api/organizations/%s/chat_conversations/%s/completion", randomUrl, organization.Uuid, newStringUuid)
	// appendMessageApi := randomUrl + "/api/append_message"

	defer func(newStringUuid, sessionKey string) {
		DeleteChatConversations(newStringUuid, sessionKey)
	}(newStringUuid, sessionKey)

	// params.ConversationUuid = newStringUuid
	// params.OrganizationUuid = organization.Uuid
	// 发起请求
	marshal, err := json.Marshal(params)
	if err != nil {
		HandleErrorResponse(c, err.Error())
		return
	}
	fmt.Println("RequestClaudeToResponse:", appendMessageApi)
	// fmt.Println("RequestClaudeToResponse Content:", marshal)
	request, err := http2.NewRequest(http2.MethodPost, appendMessageApi, bytes.NewBuffer(marshal))
	if err != nil {
		HandleErrorResponse(c, err.Error())
		return
	}
	SetSimpleHeaders(request, sessionKey)
	response, err := client.Do(request)
	if response.StatusCode != 200 && response.StatusCode != 201 {
		// fmt.Println(response)
		// 读取响应体
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Println("读取响应体失败:", err)
			return
		}

		jsonData := string(body)
		// 打印响应体
		fmt.Println("响应体内容:", jsonData)
		// 将要解析到的结构体变量
		var errResp ErrorResponse

		// 解析JSON到结构体
		err = json.Unmarshal([]byte(jsonData), &errResp)
		if err != nil {
			fmt.Println("解析JSON出错:", err)
			return
		}

		HandleErrorResponse(c, "claude错误"+errResp.Error.Message)
		return
	}
	reader := bufio.NewReader(response.Body)
	var originalResponse model.ChatMessageResponse
	var isRole = true
	fmt.Println("stream:", stream)
	if stream {
		// Response content type is text/event-stream
		c.Header("Content-Type", "text/event-stream")
		// // 告知客户端在完成此次响应后关闭连接
		// c.Header("Connection", "close")
	} else {
		// Response content type is application/json
		c.Header("Content-Type", "application/json")
	}
	var fullResponseText string
	completionResponse := model.ChatCompletionStreamResponse{
		ID:      "chatcmpl-7f1DmyzTWtiysnyfSS4i187kus2Ao",
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   "gpt-3.5-turbo-0613",
		Choices: []model.ChatCompletionStreamChoice{
			{
				Index: 0,
				Delta: model.ChatCompletionStreamChoiceDelta{
					Content: originalResponse.Completion,
				},
				FinishReason: nil,
			},
		},
	}
	lineCount := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("err == io.EOF:", err, line)
				break
			}
			return
		}
		// fmt.Println("line:", line)
		if len(line) < 6 {
			continue
		}
		lineCount += 1
		line = line[6:]
		if isRole {
			completionResponse.Choices[0].Delta.Role = "assistant"
		} else {
			completionResponse.Choices[0].Delta.Content = originalResponse.Completion
			fullResponseText += originalResponse.Completion
		}
		completionResponse.Choices[0].Delta.Role = ""
		isRole = false
		if stream {
			resp, _ := json.Marshal(completionResponse)
			responseString := "data: " + string(resp) + "\n\n"
			c.Writer.WriteString(responseString)
			c.Writer.Flush()
		}
		err = json.Unmarshal([]byte(line), &originalResponse)
		if err != nil {
			continue
		}
		if originalResponse.Stop != "" && stream {
			completionResponse.Choices[0].FinishReason = "stop"
			completionResponse.Choices[0].Delta = model.ChatCompletionStreamChoiceDelta{}
			resp, _ := json.Marshal(completionResponse)
			responseString := "data: " + string(resp) + "\n\n"
			c.Writer.WriteString(responseString)
			c.Writer.Flush()
			defer response.Body.Close()
			break
		}
	}
	fmt.Println("stream end:")
	if stream {
		c.Writer.WriteString("data: [DONE]\n\n")
		c.Writer.Flush()
	} else {
		c.JSON(200, NewChatCompletion(fullResponseText))
	}
}

func HandleErrorResponse(c *gin.Context, err string) {
	fmt.Println(err)
	c.JSON(403, gin.H{"error": gin.H{
		"message": err,
		"type":    403,
		"param":   nil,
		"code":    403,
	}})
}

func CreateChatConversations(randomUrl, newStringUuid, sessionKey string) (model.ChatConversationResponse, error) {
	var chatConversationResponse model.ChatConversationResponse
	organization, err := GetUUIDAndModel(sessionKey)
	if err != nil {
		return chatConversationResponse, err
	}
	chatConversationsApi := randomUrl + "/api/organizations/" + organization.Uuid + "/chat_conversations"
	err = client.SetProxy(global.ServerConfig.HttpProxy)
	if err != nil {
		return chatConversationResponse, err
	}
	conversation := model.NewChatConversationRequest(newStringUuid, "")
	marshal, err := json.Marshal(conversation)
	if err != nil {
		return chatConversationResponse, err
	}
	request, err := http2.NewRequest(http2.MethodPost, chatConversationsApi, bytes.NewBuffer(marshal))

	if err != nil {
		return chatConversationResponse, err
	}
	SetSimpleHeaders(request, sessionKey)

	res, err := client.Do(request)
	if err != nil {
		return chatConversationResponse, err
	}
	defer res.Body.Close()

	if res.StatusCode != 201 && res.StatusCode != 200 {
		return chatConversationResponse, errors.New("Claude创建会话出错: " + res.Status)
	}
	fmt.Println("CreateChatConversations:", res.StatusCode)
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return chatConversationResponse, err
	}
	err = json.Unmarshal(body, &chatConversationResponse)
	if err != nil {
		return chatConversationResponse, err
	}
	return chatConversationResponse, err
}

// 假设你已有相应的结构体来匹配你的JSON数据
type Conversation struct {
	UUID string `json:"uuid"`
}

func GetChatConversations(sessionKey string) ([]Conversation, error) {
	organization, err := GetUUIDAndModel(sessionKey)
	if err != nil {
		return nil, err
	}
	// err = client.SetProxy(global.ServerConfig.HttpProxy)
	// if err != nil {
	// 	return err
	// }
	randomUrl := getRandomUrl()

	chatConversationsApi := randomUrl + "/api/organizations/" + organization.Uuid + "/chat_conversations"
	request, err := http2.NewRequest(http2.MethodGet, chatConversationsApi, nil)
	if err != nil {
		return nil, err
	}
	SetSimpleHeaders(request, sessionKey)

	res, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	// 读取响应体
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println("读取响应体失败:", err)
		return nil, err
	}
	if res.StatusCode != 200 {
		// all, _ := io.ReadAll(res.Body)
		return nil, errors.New("GetChatConversations err" + string(body))
	}

	// 解析JSON数据
	var conversations []Conversation
	if err := json.Unmarshal(body, &conversations); err != nil {
		return nil, errors.New("json.Unmarshal" + string(body))
	}

	return conversations, nil
}

func DeleteChatConversations(newStringUuid, sessionKey string) error {
	organization, err := GetUUIDAndModel(sessionKey)
	if err != nil {
		return err
	}
	// err = client.SetProxy(global.ServerConfig.HttpProxy)
	// if err != nil {
	// 	return err
	// }
	randomUrl := getRandomUrl()

	chatConversationsApi := randomUrl + "/api/organizations/" + organization.Uuid + "/chat_conversations/"
	fmt.Println("before delete:", chatConversationsApi, newStringUuid)
	request, err := http2.NewRequest(http2.MethodDelete, chatConversationsApi+newStringUuid, nil)
	if err != nil {
		return err
	}
	SetSimpleHeaders(request, sessionKey)

	res, err := client.Do(request)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// 读取响应体
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println("读取响应体失败:", err)
		return err
	}
	if res.StatusCode != 204 {
		return errors.New("delete chat conversations err" + string(body))
	} else {
		fmt.Println("success after delete:", newStringUuid, string(body))
	}
	return nil
}

// OrganizationsResponse - 结构体来存储API响应
type OrganizationsResponse struct {
	Uuid string `json:"uuid"`
	// 其他可能的字段...
}

// 定义一个新的结构体，包含Uuid和Model两个字段
type OrginValue struct {
	Uuid                   string
	ClaudeModel            string
	ConsumerBanned         bool
	ConsumerRestrictedMode bool
}

var (
	cache = make(map[string]OrginValue)
	// mutex     = &sync.Mutex{}             // 使用mutex来保证并发安全
	cacheFile = "sessionKeys.json" // 替换为实际的文件路径
	loaded    = false              // 标记缓存是否已加载
)

// 从文件加载缓存
func LoadCache() error {
	// mutex.Lock()
	// defer mutex.Unlock()

	data, err := ioutil.ReadFile(cacheFile)
	if err != nil {
		return err // 文件可能不存在或无法读取
	}

	// data, err := database.Get("sessionKeys")
	//  if err != nil{
	//    data, err := ioutil.ReadFile(cacheFile)
	//    if err != nil {
	//    	return err // 文件可能不存在或无法读取
	//    }
	//    database.Set("sessionKeys",data)
	//  }

	return json.Unmarshal(data, &cache)
}

// 将缓存写入文件
func SaveCache() error {
	// mutex.Lock()
	// defer mutex.Unlock()

	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	// database.Set("sessionKeys",data)
	return ioutil.WriteFile(cacheFile, data, 0644)
}

func startHourlyTimer() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop() // 确保在函数退出时停止定时器
	doEveryHour()

	for {
		select {
		case <-ticker.C:
			doEveryHour()
		}
	}
}

func DissAccountFlag(sessionKey, uuid, typeFlag string) error {
	randomUrl := getRandomUrl()
	fmt.Println("清除flag:" + typeFlag)
	deleteUrl := fmt.Sprintf("%s/api/organizations/%s/flags/%s/dismiss", randomUrl, uuid, typeFlag)
	request, err := http2.NewRequest(http2.MethodPost, deleteUrl, nil)

	if err != nil {
		return err
	}
	SetSimpleHeaders(request, sessionKey)
	res, err := client.Do(request)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return errors.New(fmt.Sprintf("Claude清除flag失败: %s, %s", res.Status, string(body)))
	}
	return nil
}
func DissAccountFlags(sessionKey, uuid string) (bool, error) {
	organization, err := RequestOrganizations(sessionKey)
	if err != nil {
		return false, err
	}
	// fmt.Println("清除账号flag start:" + uuid)
	for _, flag := range organization.ActiveFlags {
		if flag.Type == "consumer_banned" {
			red := "\033[31m"
			reset := "\033[0m"
			fmt.Println(red + "consumer_banned:" + uuid + reset)
			return true, nil
		}
		DissAccountFlag(sessionKey, uuid, flag.Type)
	}
	// fmt.Println("清除账号flag end:" + uuid)
	return false, nil
}

func doEveryHour() {
	fmt.Println("Hourly task executed at", time.Now())
	// 执行每小时需要进行的任务
	for sessionKey, value := range cache {
		ban, err := DissAccountFlags(sessionKey, value.Uuid)
		if err == nil {
			if ban {
				value.ConsumerBanned = true
			}
		}
		// 等待五秒
		time.Sleep(5 * time.Second)
	}
}

func GetUUIDAndModel(sessionKey string) (*OrginValue, error) {
	// 如果缓存还未加载，则尝试加载它
	if !loaded {
		if err := LoadCache(); err != nil {
			// 处理加载错误，例如可以记录日志或返回错误
			panic(err)
		}
		go startHourlyTimer() // 在一个新的goroutine中启动定时器
		loaded = true
	}

	// 首先检查缓存
	if account, found := cache[sessionKey]; found {
		// 如果找到了，直接从缓存返回结果
		return &account, nil
	}
	organization, err := RequestOrganizations(sessionKey)
	if err != nil {
		return nil, err
	}

	consumer_banned := false
	for _, flag := range organization.ActiveFlags {
		if flag.Type == "consumer_banned" {
			consumer_banned = true
		}
	}

	claudeModel, err := RequestAccountModel(sessionKey)
	if err != nil {
		return nil, err
	}

	SetUUIDAndModel(sessionKey, organization.Uuid, claudeModel, consumer_banned)
	// 首先检查缓存
	if account, found := cache[sessionKey]; found {
		// 如果找到了，直接从缓存返回结果
		return &account, nil
	}

	return nil, errors.New("获取UUID失败")
}

func SetUUIDAndModel(sessionKey string, uuid string, claudeModel string, ban bool) {
	cache[sessionKey] = OrginValue{Uuid: uuid, ClaudeModel: claudeModel, ConsumerBanned: ban}
	SaveCache()
}

func RequestOrganizations(sessionKey string) (*model.OrganizationsResponse, error) {

	randomUrl := getRandomUrl()

	organizationsApi := randomUrl + "/api/organizations"
	fmt.Println(organizationsApi, sessionKey)
	request, err := http2.NewRequest(http2.MethodGet, organizationsApi, nil)

	if err != nil {
		return nil, err
	}
	SetSimpleHeaders(request, sessionKey)
	res, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("Claude获取组织出错: %s, %s", res.Status, string(body)))
	}

	var response []model.OrganizationsResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		fmt.Println(string(body))
		return nil, errors.New(string(body))
	}
	fmt.Println("get uuid", response[0].Uuid)

	return &response[0], err
}

func RequestAccountModel(sessionKey string) (string, error) {

	randomUrl := getRandomUrl()

	organizationsApi := randomUrl + "/api/auth/current_account"
	fmt.Println(organizationsApi, sessionKey)
	request, err := http2.NewRequest(http2.MethodGet, organizationsApi, nil)

	if err != nil {
		return "", err
	}
	SetSimpleHeaders(request, sessionKey)
	res, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	// body, err := io.ReadAll(res.Body)
	// if err != nil {
	// 	return "", err
	// }

	// if res.StatusCode != 200 {
	// 	return "", errors.New(fmt.Sprintf("Claude获取组织出错: %s, %s", res.Status, string(body)))
	// }

	// Use a map to hold the data
	var result map[string]interface{}

	// Decode the JSON response into the map
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		fmt.Println("Error:", err)
		return "", err
	}

	// Navigate through the map to get the nested data
	account := result["account"].(map[string]interface{})
	statsig := account["statsig"].(map[string]interface{})
	values := statsig["values"].(map[string]interface{})
	dynamicConfigs := values["dynamic_configs"].(map[string]interface{})
	config := dynamicConfigs["6zA9wvTedwkzjLxWy9PVe7yydI00XDQ6L5Fejjq/2o8="].(map[string]interface{})
	value := config["value"].(map[string]interface{})
	model := value["model"].(string)
	fmt.Println("get model", model)

	return model, err
}

func getRandomUrl() string {
	// 定义一个包含多个 URL 的数组
	urls := []string{
		"https://claude-web-proxy-deply-1.replit.app",
		"https://claude-web-proxy-deply-2.replit.app",
		"https://claude-web-proxy-deply-3.replit.app",
		"https://claude-web-proxy-deply-4.replit.app",
		"https://claude-web-proxy-deply-5.replit.app",
		// "https://defa2cdd-2bb1-45e1-9ed2-6e0df20c796d-00-2vxvss900mpxj.janeway.replit.dev",
	}

	// 初始化随机数生成器
	rand.Seed(time.Now().UnixNano())

	// 从数组中随机选择一个 URL
	randomUrl := urls[rand.Intn(len(urls))]
	return randomUrl
}
func SetSimpleHeaders(r *http2.Request, sessionKey string) {
	r.Header.Add("Cookie", sessionKey)
	r.Header.Add("Content-Type", "application/json")
}

func SetHeaders(r *http2.Request, sessionKey string) {
	r.Header.Add("Cookie", sessionKey)
	r.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36")
	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Accept", "*/*")
	r.Header.Add("Host", "claude.ai")
	r.Header.Add("Connection", "keep-alive")
}

func NewChatCompletion(fullResponseText string) model.ChatCompletionResponse {
	return model.ChatCompletionResponse{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "gpt-3.5-turbo-0613",
		Usage: model.Usage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		},
		Choices: []model.ChatCompletionChoice{
			{
				Message: model.ChatCompletionMessage{
					Content: fullResponseText,
					Role:    "assistant",
				},
				Index:        0,
				FinishReason: "stop",
			},
		},
	}
}
