package model

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/flyingeirc/erban/internal/chat/output"

	"google.golang.org/genai"
)

var totalgeminitoken int32 = 0

func Gemini(model string) *genai.Chat {
	ctx := context.TODO()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: os.Getenv("GEMINI_API_KEY"),
		// Backend:     nil,
		// Project:     nil,
		// Location:    nil,
		// HTTPClient:  nil,
		// Credentials: nil,
		// HTTPOptions:
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Set proxy
	client.ClientConfig().HTTPClient.Transport = &http.Transport{
		Proxy: http.ProxyURL(&url.URL{
			Scheme: "http",
			Host:   os.Getenv("PROXY_URL"),
		}),
	}

	// Set thinking budget
	thinkingBudget, err := strconv.Atoi(os.Getenv("THINKING_BUDGET"))
	if err != nil {
		log.Fatalf("Failed to convert thinking budget to int: %v", err)
	}
	thinkingBudget32 := int32(thinkingBudget)

	// Create a new chat
	chat, err := client.Chats.Create(ctx, model,
		&genai.GenerateContentConfig{
			ThinkingConfig: &genai.ThinkingConfig{
				ThinkingBudget: &thinkingBudget32,
			},
		},
		nil,
	)

	if err != nil {
		log.Fatalf("Failed to Create chat: %v", err)
	}
	return chat
}

func GeminiChat(chat *genai.Chat, text string, stream bool, p *output.Pacer) {
	ctx := context.TODO()

	var prompttoken, Candidatetoken int32
	// Unstreamed chat
	if !stream {
		resposne, err := chat.Send(ctx, &genai.Part{
			Text: text,
		})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(resposne.Text())
	}

	// stream chat
	response := chat.SendStream(ctx, &genai.Part{
		Text: text,
	})

	go p.Start()

	for chunk, err := range response {
		if err != nil {
			log.Fatal(err)
		}
		part := chunk.Candidates[0].Content.Parts[0]
		if string(chunk.Candidates[0].FinishReason) == "STOP" {
			totalgeminitoken += chunk.UsageMetadata.TotalTokenCount
			prompttoken = chunk.UsageMetadata.PromptTokenCount
			Candidatetoken = chunk.UsageMetadata.CandidatesTokenCount
			p.Feed(part.Text + string(chunk.Candidates[0].FinishReason))
		} else {
			p.Feed(part.Text)
		}
	}
	p.Wait()
	fmt.Printf("\nTotalToken:%d, UpToken:%d, DoToken:%d\n", totalgeminitoken, prompttoken, Candidatetoken)
	prompttoken, Candidatetoken = 0, 0
}
