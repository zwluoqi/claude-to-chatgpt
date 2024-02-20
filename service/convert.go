package service

import (
	// "encoding/json"
	"fmt"
	"github.com/oldweipro/claude-to-chatgpt/model"
	"math/rand"
	"time"
)

// generateRandomCharacters creates a string of random digits and lowercase letters.
func generateRandomCharacters(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789" // Charset with all digits and lowercase letters
	rand.Seed(time.Now().UnixNano())                       // Ensure different randomness for each execution
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// OpenaiToClaudeParams 转换成claude的参数
func OpenaiToClaudeParams(chatCompletionRequest model.ChatCompletionRequest) *model.ChatMessageRequest {

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

	// Append random characters if history length is less than 10,000 characters
	const desiredLength = 8192
	currentLength := len(prompt)
	if currentLength < desiredLength {
		gibberishLength := desiredLength - currentLength
		gibberish := generateRandomCharacters(gibberishLength)
		prompt = gibberish + prompt
	}

	return model.NewChatMessageRequest("", prompt, chatCompletionRequest.Model)
}
