package main

import (
	"crawler/internal/config"
	"fmt"
	"os"
)

func main() {

	config, err := config.LoadConfig(os.Args[1:])
	if err != nil {
		fmt.Printf("оштбка закгрузки конфига")
		return
	}

	fmt.Println(config)
}
