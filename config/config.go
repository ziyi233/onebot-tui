package config

import (
	"log" // 引入 log 包
	"os"

	"gopkg.in/yaml.v3"
)

// Config 结构体保持不变
type Config struct {
	WebSocketURL string `yaml:"webSocketUrl"`
	AccessToken  string `yaml:"accessToken"`
	DatabasePath string `yaml:"databasePath"`
	TUI          struct {
		MessageHistoryLimit int `yaml:"messageHistoryLimit"`
	} `yaml:"tui"`
}

// LoadConfig - 更新后的版本
func LoadConfig(path string) (*Config, error) {
	// 创建默认配置
	cfg := &Config{
		WebSocketURL: "ws://127.0.0.1:3001",
		DatabasePath: "onebot.db",
	}
	cfg.TUI.MessageHistoryLimit = 50

	// 尝试读取文件
	data, err := os.ReadFile(path)
	if err != nil {
		// 关键修改：如果出错，打印警告日志，然后返回默认配置
		log.Printf("警告: 无法读取配置文件 '%s'。将使用默认设置。错误: %v", path, err)
		return cfg, nil
	}

	// 如果文件读取成功，解析它
	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		// 解析错误是严重问题，需要返回错误
		return nil, err
	}

	log.Printf("成功从 '%s' 加载配置。", path)
	return cfg, nil
}
