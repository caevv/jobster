package store

import (
	"fmt"
	"strings"
)

// SupportedDrivers lists all available store drivers.
var SupportedDrivers = []string{"bbolt", "json"}

// NewStore creates a new Store instance based on the specified driver.
// Supported drivers:
//   - "bbolt": BoltDB-backed persistent storage (recommended for production)
//   - "json": JSON file-backed storage (suitable for testing and small deployments)
//
// The path parameter specifies where the store data will be persisted.
func NewStore(driver, path string) (Store, error) {
	driver = strings.ToLower(strings.TrimSpace(driver))

	if path == "" {
		return nil, fmt.Errorf("store path is required")
	}

	switch driver {
	case "bbolt":
		return NewBoltStore(path)
	case "json":
		return NewJSONStore(path)
	default:
		return nil, fmt.Errorf("unsupported store driver: %s (supported: %v)", driver, SupportedDrivers)
	}
}
