package model

import "gorm.io/gorm"

var getDB func() *gorm.DB

// SetDBGetter injects the shared database accessor from the repository layer.
func SetDBGetter(getter func() *gorm.DB) {
	getDB = getter
}

// GetDB returns the shared database handle for callers that need direct access.
func GetDB() *gorm.DB {
	if getDB == nil {
		return nil
	}
	return getDB()
}
