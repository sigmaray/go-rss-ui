package main

import (
	"log"
	"os"

	"github.com/morkid/paginate"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB
var Paginator *paginate.Pagination

func ConnectDatabase() {
	dsn := GetDSN()

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database!\n%v", err)
		os.Exit(1)
	}

	// db.AutoMigrate(&User{})

	DB = db
	Paginator = paginate.New(&paginate.Config{
		DefaultSize: 50,
		PageStart:   1, // Pages start from 1, not 0
	})
}

