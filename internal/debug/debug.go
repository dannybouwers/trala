// Package debug provides shared debug utilities for the Trala dashboard.
// It avoids code duplication of debug logging functions across packages.
package debug

import (
	"log"

	"server/internal/config"
)

var conf *config.TralaConfiguration

// Init stores the configuration instance for use by debug functions.
func Init(c *config.TralaConfiguration) {
	conf = c
}

// Debugf logs a message only if LOG_LEVEL is set to "debug".
// Uses config.GetLogLevel() to respect both config file and env var.
func Debugf(format string, v ...interface{}) {
	if conf != nil && conf.GetLogLevel() == "debug" {
		log.Printf("DEBUG: "+format, v...)
	}
}

// IsDebugEnabled returns true if LOG_LEVEL=debug is set (via config file or env var).
func IsDebugEnabled() bool {
	return conf != nil && conf.GetLogLevel() == "debug"
}
