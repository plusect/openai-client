package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/common-nighthawk/go-figure"
	"github.com/sashabaranov/go-openai"
	"gopkg.in/yaml.v3"
)

type UserConfig struct {
	Token        string `yaml:"token"`
	HttpProxyUrl string `yaml:"httpProxyUrl"`
}

func main() {
	//先读取command-line的参数
	tokenFlag := flag.String("token", "", "openai api token")
	httpProxyFlag := flag.String("httpProxyUrl", "", "httpProxyUrl")
	flag.Parse()
	if *tokenFlag != "" && *httpProxyFlag != "" {
		fmt.Println("读取了command line 的参数")
		chat(UserConfig{Token: *tokenFlag, HttpProxyUrl: *httpProxyFlag})
	} else {
		//没有指定参数,读取token文件
		config, errStr := loadConfig()
		if errStr != "" {
			fmt.Println(errStr)
			return
		}
		chat(config)
	}
}

func loadConfig() (UserConfig, string) {
	config := UserConfig{}
	configArr, err := os.ReadFile("config.yaml")
	if err != nil {
		return config, "token file read failed!"
	}

	yamlErr := yaml.Unmarshal(configArr, &config)
	if yamlErr != nil {
		return config, "token file format is error!"
	}
	if config.Token == "" || config.HttpProxyUrl == "" {
		return config, "config is error!"
	}
	return config, ""
}

func configProxy(httpProxyStr string) (*http.Transport, error) {
	proxyUrl, err := url.Parse(httpProxyStr)
	if err != nil {
		panic(err)
	}
	proxyTransport := &http.Transport{
		Proxy: http.ProxyURL(proxyUrl),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	return proxyTransport, err
}

func chat(userCOnfig UserConfig) {
	// 输出欢迎语
	myFigure := figure.NewFigure("ChatGPT", "", true)
	myFigure.Print()
	fmt.Println()

	// 配置代理
	proxyTransport, err := configProxy(userCOnfig.HttpProxyUrl)

	// 创建 ChatGPT 客户端
	config := openai.DefaultConfig(userCOnfig.Token)
	if err != nil {
		panic(err)
	}
	config.HTTPClient = &http.Client{
		Transport: proxyTransport,
		Timeout:   time.Second * 120,
	}
	client := openai.NewClientWithConfig(config)

	//初始化语句
	messages := []openai.ChatCompletionMessage{
		{
			Role:    "system",
			Content: "你是ChatGPT, OpenAI训练的大型语言模型, 请尽可能简洁地回答我的问题",
		},
	}

	fmt.Println("Please choose mode: 0 OR 1.\r\n0 is single-line text,1 is multi-line text.\r\nNote: Default is 0. And in mode 1,Question is committed by string '//end'")
	// 读取用户输入并交互
	mode := 0
	inputReader := bufio.NewReader(os.Stdin)
	modeInput, err := inputReader.ReadString('\n')
	if err != nil {
		fmt.Println(err)
		return
	}
	if modeInput == "1\r\n" {
		mode = 1
	}

	for {
		fmt.Print("マスター: ")

		// 读取用户输入并交互
		inputReader := bufio.NewReader(os.Stdin)
		userInput := ""
		var err error

		for {
			input, err := inputReader.ReadString('\n')
			if err != nil {
				fmt.Println(err)
				continue
			}
			if mode == 1 && input != "//end\r\n" {
				userInput = "" + userInput + input
			} else if mode == 1 && input == "//end\r\n" {
				break
			} else {
				userInput = input
				break
			}
		}

		if userInput == "" || userInput == "\n" {
			continue
		}
		if userInput == "quit\r\n" {
			fmt.Print("マスター: Have a nice day.")
			return
		}

		// if strings.HasSuffix(userInput, "\\c\n") {
		// 	// 数组还原
		// 	messages = []openai.ChatCompletionMessage{
		// 		{
		// 			Role:    "system",
		// 			Content: "你是ChatGPT, OpenAI训练的大型语言模型, 请尽可能简洁地回答我的问题",
		// 		},
		// 	}
		// 	fmt.Println("会话已重置")
		// 	continue
		// }

		messages = append(
			messages, openai.ChatCompletionMessage{
				Role:    "user",
				Content: userInput,
			},
		)

		// if len(messages) > 4096 {
		// 	// 数组还原
		// 	messages = []openai.ChatCompletionMessage{
		// 		{
		// 			Role:    "system",
		// 			Content: "你是ChatGPT, OpenAI训练的大型语言模型, 请尽可能简洁地回答我的问题",
		// 		},
		// 	}
		// 	fmt.Println("会话已重置")

		// 	// 重新添加消息
		// 	messages = append(
		// 		messages, openai.ChatCompletionMessage{
		// 			Role:    "user",
		// 			Content: userInput,
		// 		},
		// 	)
		// }

		// 调用 ChatGPT API 接口生成回答
		ctx := context.Background()
		req := openai.ChatCompletionRequest{
			Model:       openai.GPT3Dot5Turbo,
			Messages:    messages,
			MaxTokens:   1024,
			Temperature: 0,
			N:           1,
			Stream:      true,
		}

		stream, err := client.CreateChatCompletionStream(ctx, req)
		if err != nil {
			fmt.Printf("CompletionStream error: %v\n", err)
			return
		}
		defer stream.Close()

		// 格式化输出结果
		fmt.Print("サーバント: ")
		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				fmt.Print("\n")
				break
			}
			if err != nil {
				fmt.Printf("Stream error: %v\n", err)
				return
			}

			output := response.Choices[0].Delta.Content
			fmt.Print(output)
			messages = append(
				messages, openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: output,
				},
			)
		}
	}
}
