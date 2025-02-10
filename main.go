package main

import (
	"fmt"
	"log"
	"thetv-apg/epg"
)

func main() {
	fmt.Println("Starting EPG update process...")

	err := epg.GenerateEPGForTVList()
	if err != nil {
		log.Fatalf("Failed to update EPG XML: %v", err)
	}

	fmt.Println("EPG update completed successfully.")
}
