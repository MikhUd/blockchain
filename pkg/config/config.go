package config

import (
	"flag"
	"github.com/spf13/viper"
	"os"
)

type Config struct {
	Env              string  `mapstructure:"env" yaml:"env" env-default:"local"`
	Version          uint8   `mapstructure:"version" yaml:"version" env-required:"true"`
	MiningReward     float32 `mapstructure:"mining_reward" yaml:"mining_reward" env-required:"true"`
	MiningSender     string  `mapstructure:"mining_sender" yaml:"mining_sender" env-required:"true"`
	MiningDifficulty uint8   `mapstructure:"mining_difficulty" yaml:"mining_difficulty" env-required:"true"`
	MiningTimerSec   int     `mapstructure:"mining_timer_sec" yaml:"mining_timer_sec" env-required:"true"`
	NodesCount       int     `mapstructure:"nodes_count" yaml:"nodes_count" env-required:"true"`
}

const (
	EnvLocal = "local"
	EnvDev   = "dev"
	EnvProd  = "prod"
)

const (
	Initialized = 0
	Running     = 1
	Stopped     = 2
)

func MustLoad() *Config {
	path := fetchConfigPath()
	if path == "" {
		panic("config path is empty")
	}

	return MustLoadByPath(path)
}

func MustLoadByPath(configPath string) *Config {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config file does not exist: " + configPath)
	}

	viper.SetConfigFile(configPath)
	if err := viper.ReadInConfig(); err != nil {
		panic("Error reading config file" + err.Error())
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		panic("Error unmarshaling config:" + err.Error())
	}

	return &cfg
}

func fetchConfigPath() string {
	var res string

	flag.StringVar(&res, "config", "", "path to config file")
	flag.Parse()

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}

	return res
}
