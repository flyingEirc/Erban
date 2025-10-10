package model

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/openai/openai-go"
)

type weatherresponse struct {
	Location string `json:"location"`
}

var tool = []openai.ChatCompletionToolParam{
	{
		Type: "function",
		Function: openai.FunctionDefinitionParam{
			Name:        "get_weather",
			Description: openai.String("Get weather at the given location"),
			Parameters: openai.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "the location's style is : 'longitude,latitude',such as '116.4,39.9', and must ",
					},
				},
				"required": []string{"location"},
			},
		},
	},
}

func get_weather(location string) (string, error) {
	var response weatherresponse

	if err := json.Unmarshal([]byte(location), &response); err != nil {
		return "", err
	}

	var apikey string = "0e940da562694d6d97cc903a7b610b43"

	client := http.DefaultClient
	client.Transport = &http.Transport{
		Proxy: http.ProxyURL(&url.URL{
			Scheme: "http",
			Host:   "127.0.0.1:10808",
		}),
	}

	baseUrl := "https://np6mtemp2p.re.qweatherapi.com/v7/weather/now"
	u, _ := url.Parse(baseUrl)
	q := u.Query()
	q.Add("location", response.Location)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("X-QW-Api-Key", apikey)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

func executeToolCall(funcname, args string) (string, error) {
	switch funcname {
	case "get_weather":
		return get_weather(args)
	default:
		return "", nil
	}
}
