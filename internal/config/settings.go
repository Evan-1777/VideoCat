package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings 应用级设置，持久化到应用数据目录。
type Settings struct {
	DefaultOutputDir string `json:"defaultOutputDir"`
	FFmpegPath       string `json:"ffmpegPath"`
}

func settingsPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// LoadSettings 读取设置；文件不存在时返回零值设置与 nil。
func LoadSettings() (Settings, error) {
	var s Settings
	path, err := settingsPath()
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

// Save 将设置写入应用数据目录。
func (s Settings) Save() error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
