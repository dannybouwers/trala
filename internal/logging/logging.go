// Package logging provides shared debug logging for the Trala dashboard.
package logging

import (
	"log"

	"server/internal/config"
)

// Debugf logs a message only if LOG_LEVEL is set to "debug".
func Debugf(format string, v ...interface{}) {
	if config.GetLogLevel() == "debug" {
		log.Printf("DEBUG: "+format, v...)
	}
}
