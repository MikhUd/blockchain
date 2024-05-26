package config

import (
	"github.com/spf13/viper"
	"os"
)

type Config struct {
	Env              string  `mapstructure:"env" yaml:"env" env-required:"true"`
	Version          uint8   `mapstructure:"version" yaml:"version" env-required:"true"`
	MiningReward     float32 `mapstructure:"mining_reward" yaml:"mining_reward" env-required:"true"`
	MiningSender     string  `mapstructure:"mining_sender" yaml:"mining_sender" env-required:"true"`
	MiningDifficulty uint8   `mapstructure:"mining_difficulty" yaml:"mining_difficulty" env-required:"true"`
	MiningTimerSec   int     `mapstructure:"mining_timer_sec" yaml:"mining_timer_sec" env-required:"true"`

	NodesCount                 int   `mapstructure:"nodes_count" yaml:"nodes_count"`
	MaxMemberHeartbeatMisses   uint8 `mapstructure:"max_member_heartbeat_misses" yaml:"max_member_heartbeat_misses" env-required:"true"`
	MaxClusterHeartbeatMisses  uint8 `mapstructure:"max_cluster_heartbeat_misses" yaml:"max_cluster_heartbeat_misses" env-required:"true"`
	MemberJoinTimeoutMs        int   `mapstructure:"member_join_timeout_ms" yaml:"member_join_timeout_ms" env-required:"true"`
	HeartBeatTimeout           int   `mapstructure:"heartbeat_timeout" yaml:"heartbeat_timeout" env-required:"true"`
	MemberHeartbeatIntervalMs  int   `mapstructure:"member_heartbeat_interval_ms" yaml:"member_heartbeat_interval" env-required:"true"`
	ClusterHeartbeatIntervalMs int   `mapstructure:"cluster_heartbeat_interval_ms" yaml:"cluster_heartbeat_interval" env-required:"true"`
	MinLeaderElectionMs        int   `mapstructure:"min_leader_election_ms" yaml:"min_leader_election_ms" env-required:"true"`
	MaxLeaderElectionMs        int   `mapstructure:"max_leader_election_ms" yaml:"max_leader_election_ms" env-required:"true"`
	MemberRecoverTimeout       int   `mapstructure:"member_recover_timeout_ms" yaml:"member_recover_timeout_ms" env-required:"true"`
	MaxClusterJoinAttempts     uint8 `mapstructure:"max_cluster_join_attempts" yaml:"max_cluster_join_attempts" env-required:"true"`
	MaxMemberRecoverMisses     uint8 `mapstructure:"max_member_recover_misses" yaml:"max_member_recover_misses" env-required:"true"`
	MemberRecoverIntervalMs    int   `mapstructure:"member_recover_interval_ms" yaml:"member_recover_interval_ms" env-required:"true"`
	MemberTimeoutSec           int   `mapstructure:"member_timeout_sec" yaml:"member_timeout_sec" env-required:"true"`

	CassandraKeyspace string   `mapstructure:"cassandra_keyspace" yaml:"cassandra_keyspace" env-required:"true"`
	CassandraHosts    []string `mapstructure:"cassandra_hosts" yaml:"cassandra_hosts" env-required:"true"`

	HandleInterrupt int `mapstructure:"handle_interrupt" yaml:"handle_interrupt" env-required:"true"`
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
