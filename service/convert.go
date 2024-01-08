package service

import (
	"encoding/json"
	"fmt"
	"github.com/oldweipro/claude-to-chatgpt/model"
	// "math/rand"
	// "time"
)

// OpenaiToClaudeParams 转换成claude的参数
func OpenaiToClaudeParams(chatCompletionRequest model.ChatCompletionRequest) *model.ChatMessageRequest {
	// completionMessages := chatCompletionRequest.Messages
	// text := completionMessages[len(completionMessages)-1]
	// history := completionMessages[:len(completionMessages)-1]
	// textMarshal, err := json.Marshal(text)
	// if err != nil {
	// 	fmt.Println("Text marshal err:", err)
	// }
	// textMessage := string(textMarshal)
	prompt := ""
	for _, message := range chatCompletionRequest.Messages {
		if message.Role == "user" {
			prompt += fmt.Sprintf("\n\nHuman: %s", message.Content)
		} else if message.Role == "assistant" {
			prompt += fmt.Sprintf("\n\nAssistant: %s", message.Content)
		} else if message.Role == "system" {
			// if prompt == "" {
			// 	prompt = message.StringContent()
			// }
			prompt += fmt.Sprintf("\n\nAssistant: %s", message.Content)
		}
	}
	prompt += "\n\nAssistant:"

	//textMessage = "附件中存放着对话的上下文，作答时请忽略上下文的json格式，请以最新的prompt作答：" + textMessage
	// textMessage = "The attachment contains the context of the conversation. When answering, please ignore the json format of the context. Please answer with the latest prompt: " + textMessage
	textMessage := ""
	modelName := "claude-2.0"
	if chatCompletionRequest.Model == "claude-2.0" || chatCompletionRequest.Model == "claude-2.1" {
		modelName = chatCompletionRequest.Model
	}
	return model.NewChatMessageRequest(textMessage, prompt, modelName)
}

// // generateRandomCharacters creates a string of random digits and lowercase letters.
// func generateRandomCharacters(length int) string {
// 	const charset = "abcdefghijklmnopqrstuvwxyz0123456789" // Charset with all digits and lowercase letters
// 	rand.Seed(time.Now().UnixNano())                       // Ensure different randomness for each execution
// 	b := make([]byte, length)
// 	for i := range b {
// 		b[i] = charset[rand.Intn(len(charset))]
// 	}
// 	return string(b)
// }

// // OpenaiToClaudeParams 转换成claude的参数
// func OpenaiToClaudeParams(chatCompletionRequest model.ChatCompletionRequest) *model.ChatMessageRequest {
// 	completionMessages := chatCompletionRequest.Messages
// 	// text := completionMessages[len(completionMessages)-1]
// 	// history := completionMessages[:len(completionMessages)-1]
// 	// textMarshal, err := json.Marshal(text)
// 	// if err != nil {
// 	//   fmt.Println("Text marshal err:", err)
// 	// }
// 	// textMessage := string(textMarshal)
// 	historyMessage := ""
// 	if len(completionMessages) > 0 {
// 		historyMarshal, err := json.Marshal(completionMessages)
// 		if err != nil {
// 			fmt.Println("History marshal err:", err)
// 		}
// 		historyMessage = string(historyMarshal)
// 	}

// 	// Append random characters if history length is less than 10,000 characters
// 	const desiredLength = 4096
// 	currentLength := len(historyMessage)
// 	if currentLength < desiredLength {
// 		gibberishLength := desiredLength - currentLength
// 		gibberish := generateRandomCharacters(gibberishLength)
// 		historyMessage = gibberish + historyMessage
// 	}

// 	//textMessage = "附件中存放着对话的上下文，作答时请忽略上下文的json格式，请以最新的prompt作答：" + textMessage
// 	// textMessage = "The attachment contains the context of the conversation. When answering, please ignore the json format of the context. Please answer with the latest prompt: " + textMessage
// 	return model.NewChatMessageRequest("", historyMessage)
// }
