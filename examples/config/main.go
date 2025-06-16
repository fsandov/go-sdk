package main

import (
	"fmt"
	"os"

	"github.com/fsandov/go-sdk/pkg/config"
)

func main() {
	cfg := &config.AppConfig{
		AppName:     os.Getenv("SERVICE_NAME"),
		Environment: os.Getenv("ENVIRONMENT"),
		Port:        os.Getenv("PORT"),
		Extras: map[string]interface{}{
			"feature_flags": os.Getenv("FEATURE_FLAGS"),
			"custom_map": map[string]interface{}{
				"foo": "bar",
			},
		},
	}
	config.Init(cfg)

	c := config.Get()
	fmt.Println(c)
}
