package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

var PATH string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	PATH = filepath.Join(home, ".config", "kindle_cli", "config.json")
}

type Config struct {
	KindleEmail         string `json:"kindle_email"`
	MangaDexApiKey      string `json:"manga_dex_api_key"`
	SMTPHost            string `json:"smtp_host"`
	SMTPPort            int    `json:"smtp_port"`
	SMTPUsername        string `json:"smtp_user"`
	SMTPPassword        string `json:"smtp_pass"`
	DefaultLanguage     string `json:"default_language"`
	ConcurrentDownloads int    `json:"concurrent_downloads"`
	ApiBaseUrl          string `json:"api_base_url"`
	SendToKindle        bool   `json:"send_to_kindle"`
	DownloadDir         string `json:"download_dir"`
}

func DefaultConfig() *Config {
	return &Config{
		KindleEmail:         "",
		MangaDexApiKey:      "",
		SMTPHost:            "",
		SMTPPort:            465,
		SMTPUsername:        "",
		SMTPPassword:        "",
		DefaultLanguage:     "en",
		ConcurrentDownloads: 5,
		ApiBaseUrl:          "https://api.mangadex.org",
		SendToKindle:        false,
		DownloadDir:         "",
	}
}

func (c *Config) Save() error { // Implement saving the configuration to a file or other storage
	//check if config path exists, if not create it
	if err := os.MkdirAll(filepath.Dir(PATH), 0770); err != nil {
		return err
	}
	file, err := os.Create(PATH)
	if err != nil {
		return err
	}
	defer file.Close()

	//save the config to the path as json
	encoder := json.NewEncoder(file)
	return encoder.Encode(c)
}

func Load() (*Config, error) {
	// Implement loading the configuration from a file or other storage
	//check if config path exists, if not return default config
	if _, err := os.Stat(PATH); errors.Is(err, os.ErrNotExist) {
		return DefaultConfig(), nil
	}

	//load the config from the path
	data, err := os.ReadFile(PATH)
	if err != nil {
		return nil, err
	}

	//unmarshal the config from the path
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
