package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config represents common bot settings
type Config struct {
	BotToken     string
	ReportChatID int64
	Debug        bool
	SmsAPIKey    string
	VpnHost      string
	WorkingDir   string
}

// Get current config
func Get() *Config {
	token := os.Getenv("BOT_TOKEN")
	if len(token) == 0 {
		panic("BOT_TOKEN required")
	}

	reportChatID, err := strconv.ParseInt(os.Getenv("ADMIN_CHAT_ID"), 10, 64)
	fmt.Println(os.Getenv("ADMIN_CHAT_ID"))
	if err != nil {
		panic(err)
	}

	debug, err := strconv.ParseBool(os.Getenv("DEV"))
	if err != nil {
		debug = false
	}

	workingDir := os.Getenv("WORKING_DIR")
	if len(workingDir) == 0 {
		workingDir = "data"
	}

	c := &Config{
		BotToken:     token,
		ReportChatID: reportChatID,
		Debug:        debug,
		WorkingDir:   workingDir,
	}

	return c
}