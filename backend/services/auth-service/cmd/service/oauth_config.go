package main

import "os"

type GoogleConfig struct {
    ClientID     string
    ClientSecret string
    RedirectURL  string
}

type FTConfig struct {
    ClientID     string
    ClientSecret string
    RedirectURL  string
}

func loadGoogleConfigFromEnv() GoogleConfig {
    return GoogleConfig{
        ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
        ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
        RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
    }
}

func loadFTConfigFromEnv() FTConfig {
    return FTConfig{
        ClientID:     os.Getenv("FT_CLIENT_ID"),
        ClientSecret: os.Getenv("FT_CLIENT_SECRET"),
        RedirectURL:  os.Getenv("FT_REDIRECT_URL"),
    }
}
