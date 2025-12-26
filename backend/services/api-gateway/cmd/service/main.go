package main

import (
	"log"
	"net/http"
)

func main() {
	cfg, err := loadConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	r := setupRouter(cfg)

	log.Printf("api-gateway listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("api-gateway: %v", err)
	}
}
