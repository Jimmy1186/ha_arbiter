package config

import (
	"log"
	"os"

	"github.com/goccy/go-yaml"
)

type Config struct {
	ARBITOR_NAME string `yaml:"ARBITOR_NAME"`
	DEFAULT_ROLE int32  `yaml:"DEFAULT_ROLE"`
	PRIORITY     int32  `yaml:"PRIORITY"`
	SERVER_IP    string `yaml:"SERVER_IP"`
	SERVER_PORT  string `yaml:"SERVER_PORT"`
	CLIENT_IP    string `yaml:"CLIENT_IP"`
	CLIENT_PORT  string `yaml:"CLIENT_PORT"`
}

var Cfg Config

func init() {
	data, err := os.ReadFile("config/config.yaml")
	if err != nil {
		log.Fatal("Cannot read config.yaml:", err)
	}

	if err := yaml.Unmarshal(data, &Cfg); err != nil {
		log.Fatal("YAML parse error:", err)
	}
}
