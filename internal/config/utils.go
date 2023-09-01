package config

import (
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"os"
)

func LoadConfigOrDie(configPath string) *Config {
	buf, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Failed to read config file %s: %v", configPath, err)
	}

	config := Config{}
	if err := yaml.Unmarshal(buf, &config); err != nil {
		log.Fatalf("Failed to parse yaml from config file %s: %v", configPath, err)
	}
	config.Init()
	return &config
}
