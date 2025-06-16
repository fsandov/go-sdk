package main

import (
	"github.com/fsandov/go-sdk/pkg/database"
)

func main() {
	db, err := database.Open(database.DefaultMySqlConfig, nil)
	if err != nil {
		panic(err)
	}

	db.Exec("CREATE TABLE IF NOT EXISTS users (id INT PRIMARY KEY, name VARCHAR(255))")
}
