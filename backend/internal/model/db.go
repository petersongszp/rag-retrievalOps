package model

import "gorm.io/gorm"

func GetDB() *gorm.DB {
	if getDB == nil {
		return nil
	}
	return getDB()
}
