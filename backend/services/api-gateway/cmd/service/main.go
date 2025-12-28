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

	// tlsEnabled := getenv("TLS_ENABLED", "false") == "true"
	// if tlsEnabled {
	// 	certFile := getenv("TLS_CERT_FILE", "/certs/cert.pem")
	// 	keyFile := getenv("TLS_KEY_FILE", "/certs/key.pem")
	// 	log.Printf("api-gateway listening on HTTPS :%s", port)
	// 	if err := http.ListenAndServeTLS(":"+port, certFile, keyFile, r); err != nil {
	// 		log.Fatalf("api-gateway (TLS): %v", err)
	// 	}
	// } else {
	// 	log.Printf("api-gateway listening on HTTP :%s", port)
	// 	if err := http.ListenAndServe(":"+port, r); err != nil {
	// 		log.Fatalf("api-gateway: %v", err)
	// 	}
	// }
}
