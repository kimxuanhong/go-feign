package feign

import (
	"github.com/spf13/viper"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Url        string            `mapstructure:"url" yaml:"url"`
	Timeout    time.Duration     `mapstructure:"timeout" yaml:"timeout"`
	RetryCount int               `mapstructure:"retry_count" yaml:"retry_count"`
	RetryWait  time.Duration     `mapstructure:"retry_wait" yaml:"retry_wait"`
	Headers    map[string]string `mapstructure:"headers" yaml:"headers"`
}

func NewConfig() *Config {
	timeout, _ := time.ParseDuration(getEnv("FEIGN_TIMEOUT", "30s"))
	retryWait, _ := time.ParseDuration(getEnv("FEIGN_RETRY_WAIT", "1s"))
	retryCount, _ := strconv.Atoi(getEnv("FEIGN_RETRY_COUNT", "0"))
	return &Config{
		Timeout:    timeout,
		RetryCount: retryCount,
		RetryWait:  retryWait,
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func GetConfig(configs ...*Config) *Config {
	if len(configs) > 0 && configs[0] != nil {
		return configs[0]
	}
	viper.SetDefault("feign.timeout", "30s")
	viper.SetDefault("feign.retry_count", "0")
	viper.SetDefault("feign.retry_wait", "1s")
	return &Config{
		Timeout:    viper.GetDuration("feign.timeout"),
		RetryCount: viper.GetInt("feign.retry_count"),
		RetryWait:  viper.GetDuration("feign.retry_wait"),
	}
}
