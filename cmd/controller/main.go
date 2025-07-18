package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/ziyi233/onebot-tui/adapter"
)

const apiBaseURL = "http://localhost:9090"

func main() {
	var rootCmd = &cobra.Command{Use: "onebot"}

	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "列出所有好友和群聊",
		Run: func(cmd *cobra.Command, args []string) {
			resp, err := http.Get(apiBaseURL + "/get_chats")
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
			defer resp.Body.Close()
			var chats []adapter.ChatInfo
			json.NewDecoder(resp.Body).Decode(&chats)
			fmt.Println("--- 群聊 / 好友 ---")
			for _, chat := range chats {
				fmt.Printf("类型: %-7s | ID: %-12s | 名称: %s\n", chat.Type, chat.ID, chat.Name)
			}
		},
	}

	var useCmd = &cobra.Command{
		Use:   "use [ID]",
		Short: "切换当前聊天窗口",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			resp, err := http.Post(apiBaseURL+"/set_active_chat?id="+id, "", nil)
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
			defer resp.Body.Close()
			io.Copy(os.Stdout, resp.Body)
		},
	}

	var sendCmd = &cobra.Command{
		Use:   "send [消息...]",
		Short: "向当前窗口发送消息",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			message := strings.Join(args, " ")
			resp, err := http.Post(apiBaseURL+"/send_message", "text/plain", bytes.NewBufferString(message))
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
			defer resp.Body.Close()
			// 可以在这里打印“发送成功”或服务器的响应
		},
	}

	rootCmd.AddCommand(listCmd, useCmd, sendCmd)
	rootCmd.Execute()
}
