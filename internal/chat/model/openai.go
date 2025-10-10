package model

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/flyingeirc/erban/internal/chat/output"
	"github.com/flyingeirc/erban/internal/chat/tools"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

type Chat struct {
	client           openai.Client
	totalopenaitoken int64
	prompttoken      int64
	completiontoken  int64
	openaicontext    []openai.ChatCompletionMessageParamUnion
	istollcall       bool
	param            openai.ChatCompletionNewParams
	p                *output.Pacer
}

type Mc struct {
	Proxy   url.URL
	Model   string
	Reason  string
	Key     string
	Baseurl string
}

func Openai(modelconf *Mc) *Chat {
	var client openai.Client

	newclient := &http.Client{}
	var ProxyFunc func(*http.Request) (*url.URL, error)
	if modelconf.Proxy.Scheme != "" && modelconf.Proxy.Host != "" {
		ProxyFunc = http.ProxyURL(&modelconf.Proxy)
	} else {
		ProxyFunc = http.ProxyFromEnvironment
	}

	newclient.Transport = &http.Transport{Proxy: ProxyFunc}
	if modelconf.Baseurl == "" {
		client = openai.NewClient(
			option.WithAPIKey(modelconf.Key),
			option.WithHTTPClient(newclient),
		)
	} else {
		client = openai.NewClient(
			option.WithAPIKey(modelconf.Key),
			option.WithHTTPClient(newclient),
			option.WithBaseURL(modelconf.Baseurl),
		)
	}

	param := openai.ChatCompletionNewParams{
		Tools:           tool,
		Model:           modelconf.Model,
		ReasoningEffort: shared.ReasoningEffort(modelconf.Reason),
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: param.NewOpt(true),
		},
	}
	pacer := output.NewPacer(1000, 8*time.Millisecond, 12)

	return &Chat{
		client:        client,
		openaicontext: []openai.ChatCompletionMessageParamUnion{},
		param:         param,
		p:             pacer,
	}
}

func (c *Chat) Start(ctx context.Context, text string, p *output.Pacer) {
	builders := map[int]*tools.ToolCallbuilder{}
	var toolCalls []openai.ChatCompletionMessageToolCallParam

	if text != "" {
		c.openaicontext = append(c.openaicontext, openai.UserMessage(text))
	}

	c.param.Messages = c.openaicontext

	// 流式处理
	chat := c.client.Chat.Completions.NewStreaming(ctx, c.param)
	defer chat.Close()

	if chat.Err() != nil {
		log.Println(chat.Err())
		return
	}

	go p.Start()

	var completeContent strings.Builder // 累积完整的响应内容

	for chat.Next() {
		chunk := chat.Current()

		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]

			if choice.FinishReason == "stop" {
				// 只添加一次完整的 assistant 消息
				if completeContent.Len() > 0 {
					c.openaicontext = append(c.openaicontext, openai.AssistantMessage(completeContent.String()))
				}
				p.Cancel()
			} else if choice.FinishReason == "tool_calls" {
				// 处理工具调用
				for _, b := range builders {
					tc, err := b.Done()
					if err != nil {
						log.Printf("Tool call builder error: %v", err)
						continue
					}
					toolCalls = append(toolCalls, tc)
				}
				assistantTC := &openai.ChatCompletionAssistantMessageParam{
					ToolCalls: toolCalls,
				}
				c.openaicontext = append(c.openaicontext, openai.ChatCompletionMessageParamUnion{OfAssistant: assistantTC})

				// 执行工具调用并添加结果到上下文
				for _, tc := range toolCalls {
					result, err := executeToolCall(tc.Function.Name, tc.Function.Arguments)
					if err != nil {
						fmt.Println(err)
					}
					toolResult := openai.ToolMessage(result, tc.ID)
					c.openaicontext = append(c.openaicontext, toolResult)
				}
				c.istollcall = true
			} else if choice.Delta.Content != "" {
				// 修复：正确检查空指针
				content := choice.Delta.Content
				p.Feed(content)
				completeContent.WriteString(content) // 累积内容
			}

			// 处理工具调用
			for _, tc := range choice.Delta.ToolCalls {
				b := builders[int(tc.Index)]
				if b == nil {
					b = &tools.ToolCallbuilder{}
					builders[int(tc.Index)] = b
				}
				if !reflect.DeepEqual(tc.Function, openai.ChatCompletionChunkChoiceDeltaToolCallFunction{}) {
					name := tc.Function.Name
					b.Feed(tc.ID, name, tc.Function.Arguments)
				}
			}
		}

		// Token 统计
		if !reflect.DeepEqual(chunk.Usage, openai.CompletionUsage{}) {
			c.totalopenaitoken += chunk.Usage.TotalTokens
			c.prompttoken, c.completiontoken = chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens
			if c.istollcall {
				c.istollcall = false
				c.Start(ctx, "", p)
				return
			} else {
				p.Wait()
			}
		}
	}

	fmt.Printf("\nTotaltoken:%d, Uptoken:%d, Dotoken:%d\n", c.totalopenaitoken, c.prompttoken, c.completiontoken)
}
