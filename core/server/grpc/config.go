package core_server_v1_grpc

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port string `envconfig:"GRPC_PORT" default:"50051"`
}

func NewConfig() (Config, error) {
	var config Config
	if err := envconfig.Process("", &config); err != nil {
		return Config{}, fmt.Errorf("can't get grpc port from config")
	}

	return config, nil
}

func NewConfigMust() Config {
	config, err := NewConfig()
	if err != nil {
		err = fmt.Errorf("can't get port grpc from config: %w", err)
		panic(err)
	}
	return config
}
