package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDatabase() {

	// first store all the environment variables in local variables and log appropreate messages for the missing variables.
	// Then access in the other places.

	dsn := fmt.Sprintf(
		// access sllmode through the envs. make it disable for the local development.
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	// can we reduce this number to 3?
	const maxAttempts = 10
	const retryDelay = 3 * time.Second

	var database *gorm.DB
	var err error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		database, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}

		log.Printf("Failed to connect to database (attempt %d/%d): %v", attempt, maxAttempts, err)

		if attempt == maxAttempts {
			log.Fatalf("Could not connect to database after %d attempts: %v", maxAttempts, err)
		}

		time.Sleep(retryDelay)
	}

	sqlDB, err := database.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying sql.DB for pool configuration: %v", err)
	}

	// isn't this redundant. g
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err = sqlDB.Ping(); err == nil {
			break
		}
		log.Printf("Database ping failed (attempt %d/%d): %v", attempt, maxAttempts, err)
		if attempt == maxAttempts {
			log.Fatalf("Database did not become pingable after %d attempts: %v", maxAttempts, err)
		}
		time.Sleep(retryDelay)
	}

	// all these pool values should be accessible through configurations. then only we can easily change those.
	sqlDB.SetMaxOpenConns(30)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	fmt.Println("Database connected successfully!")
	// return the db and the error. then in the main file check the error
	DB = database
}
