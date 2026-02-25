package config

import (
	"log"
	"os"

	"github.com/goccy/go-yaml"
)

type Config struct {
	FLEET_HB_INTERVAL int32 `yaml:"FLEET_HB_INTERVAL"`
	FLEET_HB_TIMEOUT  int32 `yaml:"FLEET_HB_TIMEOUT"`

	OTHER_HA_HB_INTERVAL int32 `yaml:"OTHER_HA_HB_INTERVAL"`
	OTHER_HA_HB_TIMEOUT  int32 `yaml:"OTHER_HA_HB_TIMEOUT"`

	SERVER_IP   string `yaml:"SERVER_IP"`
	SERVER_PORT string `yaml:"SERVER_PORT"`
	CLIENT_IP   string `yaml:"CLIENT_IP"`
	CLIENT_PORT string `yaml:"CLIENT_PORT"`

	WEB_API_PORT string `yaml:"WEB_API_PORT"`
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
