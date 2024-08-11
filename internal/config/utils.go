package config

import (
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"os"
)

const envVarConfigContent = "SMCR_CONFIG"

func LoadConfigOrDie(configPath string) *Config {
	var configBuf []byte
	if configData, ok := os.LookupEnv(envVarConfigContent); ok {
		log.Infof("Loading config from envvar %s", envVarConfigContent)
		configBuf = []byte(configData)
	} else {
		buf, err := os.ReadFile(configPath)
		if err != nil {
			log.Fatalf("Failed to read config file %s: %v", configPath, err)
		}
		configBuf = buf
	}

	config := Config{}
	if err := yaml.Unmarshal(configBuf, &config); err != nil {
		log.Fatalf("Failed to parse yaml from config file %s: %v", configPath, err)
	}
	config.Init()
	return &config
}
