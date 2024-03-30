package config

const (
	EnvLocal = "local"
	EnvDev   = "dev"
	EnvProd  = "prod"
)

const (
	Initialized = iota
	Running
	Stopped
)

const (
	Active = iota
	Inactive
)

const (
	Leader = iota
	Follower
	Candidate
)
