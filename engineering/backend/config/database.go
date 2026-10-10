package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// even you define the DB as a global and exported to the other packages,
// currently config.DB is used only in one place in the main.go
// return the database and the error from the ConnectDatabase() and pass it. by this way we can remove this package level global here
var DB *gorm.DB

func ConnectDatabase() {

	// first store the env variables in local variables
	// ex: db_host = os.Getenv("DB_HOST")
	// throws the errors for the missing envs.
	// then after access inside the fmt.Sprintf()
	// take sslmode from the envs.
	// dont we need the timezone here?

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

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

		// we can move this check after the loop to increase the readability and sleep,
		// if attempt < maxAttempts {
		// 	time.Sleep(retryDelay)
		//}
		if attempt == maxAttempts {
			log.Fatalf("Could not connect to database after %d attempts: %v", maxAttempts, err)
		}

		time.Sleep(retryDelay)
	}

	//if err != nil {
	//	log.Fatalf("Could not connect to database after %d attempts: %v", maxAttempts, err)
	//}

	sqlDB, err := database.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying sql.DB for pool configuration: %v", err)
	}

	// does gorm.Open() ping the database? if so we dont need this redundent ping loop.
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

	// access all these hardcoded values from env variables. then only we can easily change without touching the code
	sqlDB.SetMaxOpenConns(30)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	fmt.Println("Database connected successfully!")
	DB = database
	// return the database,nil here. in other breaking places return the nil,err
}
