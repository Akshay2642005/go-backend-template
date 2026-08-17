package main

import (
	"fmt"
	"log"

	"backend/internal/config"
)

func main() {
	config, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Could not load configuration:", err)
	}

	fmt.Println("Loaded configuration:", config)
}
