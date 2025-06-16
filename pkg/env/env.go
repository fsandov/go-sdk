package env

import "os"

var (
	// environment holds the current environment value retrieved from the ENVIRONMENT variable.
	environment = os.Getenv("ENVIRONMENT")
)

// IsDevelopment returns true if the current environment is set to "development".
func IsDevelopment() bool {
	return environment == "development"
}

// IsProduction returns true if the current environment is set to "production".
func IsProduction() bool {
	return environment == "production"
}

// IsRemote returns true if the application is running in either "production" or "development" mode.
// This function helps to differentiate between remote environments and local development.
func IsRemote() bool {
	return IsProduction() || IsDevelopment()
}

// IsLocal returns true if the application is running in a local environment.
// This includes explicitly setting the environment to "local".
func IsLocal() bool {
	return environment == "local"
}

func GetEnvironment() string {
	return environment
}
