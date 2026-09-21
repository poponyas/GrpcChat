package core_logger

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Level  string `envconfig:"LEVEL" default:"debug"`
	Folder string `envconfig:"FOLDER" default:"logs"`
}

func NewConfig() (Config, error) {
	var config Config
	if err := envconfig.Process("LOGGER", &config); err != nil {
		return Config{}, fmt.Errorf(
			"can't get LOGGER fields by config: %w", err,
		)
	}

	return config, nil
}

func NewConfigMust() Config {
	config, err := NewConfig()
	if err != nil {
		err := fmt.Errorf("can't get config: %w", err)
		panic(err)
	}

	return config
}
