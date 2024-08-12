package config

import (
	"flag"
	"os"
	"strconv"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Clients       ClientsConfig `yaml:"clients"`
	BindAddr      string        `yaml:"bind_addr"`
	LogLevel      string        `yaml:"log_level"`
	LocalHostMode bool          `yaml:"localhost_mode"`
}

type AppConfig struct {
	AppSecret string `env-required:"true" env:"APP_SECRET"`
	AppID     int64  `env-required:"true" env:"APP_ID"`
}

type Client struct {
	Adress       string        `yaml:"adress"`
	Timeout      time.Duration `yaml:"timeout"`
	RetriesCount int           `yaml:"retries_count"`
}

type ClientsConfig struct {
	SSO Client `yaml:"sso"`
}

func MustLoad() (*Config, *AppConfig) {
	configPath, appCfg := fetchConfigPath()
	if configPath == "" {
		panic("config path is empty")
	}

	return MustLoadPath(configPath), appCfg
}

func MustLoadPath(configPath string) *Config {
	// check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config file does not exist: " + configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic("cannot read config: " + err.Error())
	}

	return &cfg
}

// fetchConfigPath fetches config path from command line flag or environment variable.
// Priority: flag > env > default.
// Default value is empty string.
func fetchConfigPath() (string, *AppConfig) {
	var res, appID string
	var appCfg AppConfig
	flag.StringVar(&appCfg.AppSecret, "app_secret", "chat-secret", "secret key for app")
	flag.StringVar(&appID, "app_id", "1", "id of app")
	flag.StringVar(&res, "config", "/home/egor/DEV/apartment_building_app/appartament_building_chat/config/config.yaml", "path to config file")
	flag.Parse()
	var err error
	appCfg.AppID, err = strconv.ParseInt(appID, 10, 64)

	if err != nil {
		panic("cannot read flag: " + err.Error())
	}

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}

	return res, &appCfg
}
