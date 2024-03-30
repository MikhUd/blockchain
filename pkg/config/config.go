package config

import (
	"github.com/spf13/viper"
	"os"
)

type Config struct {
	Env                        string  `mapstructure:"env" yaml:"env" env-required:"true"`
	Version                    uint8   `mapstructure:"version" yaml:"version" env-required:"true"`
	MiningReward               float32 `mapstructure:"mining_reward" yaml:"mining_reward" env-required:"true"`
	MiningSender               string  `mapstructure:"mining_sender" yaml:"mining_sender" env-required:"true"`
	MiningDifficulty           uint8   `mapstructure:"mining_difficulty" yaml:"mining_difficulty" env-required:"true"`
	MiningTimerSec             int     `mapstructure:"mining_timer_sec" yaml:"mining_timer_sec" env-required:"true"`
	NodesCount                 int     `mapstructure:"nodes_count" yaml:"nodes_count"`
	MaxNodeHeartbeatMisses     int     `mapstructure:"max_node_heartbeat_misses" yaml:"max_node_heartbeat_misses" env-required:"true"`
	MaxClusterHeartbeatMisses  uint8   `mapstructure:"max_cluster_heartbeat_misses" yaml:"max_cluster_heartbeat_misses" env-required:"true"`
	MemberJoinTimeout          int     `mapstructure:"member_join_timeout" yaml:"member_join_timeout" env-required:"true"`
	HeartBeatTimeout           int     `mapstructure:"heartbeat_timeout" yaml:"heartbeat_timeout" env-required:"true"`
	NodeHeartbeatIntervalMs    int     `mapstructure:"node_heartbeat_interval_ms" yaml:"node_heartbeat_interval" env-required:"true"`
	ClusterHeartbeatIntervalMs int     `mapstructure:"cluster_heartbeat_interval_ms" yaml:"node_heartbeat_interval" env-required:"true"`
	MinLeaderElectionMs        int     `mapstructure:"min_leader_election_ms" yaml:"node_heartbeat_interval" env-required:"true"`
	MaxLeaderElectionMs        int     `mapstructure:"max_leader_election_ms" yaml:"node_heartbeat_interval" env-required:"true"`
	NodeRecoverTimeout         int     `mapstructure:"node_recover_timeout_ms" yaml:"node_recover_timeout_ms" env-required:"true"`
}

func MustLoad(path string) *Config {
	if path == "" {
		panic("config path is empty")
	}

	return mustLoadByPath(path)
}

func mustLoadByPath(configPath string) *Config {
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
