package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	port := getEnv("PORT", "8081")
	name := getEnv("SERVICE_NAME", "upstream")

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if ms, _ := strconv.Atoi(getEnv("LATENCY_MS", "0")); ms > 0 {
			time.Sleep(time.Duration(ms) * time.Millisecond)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"service":       name,
			"path":          r.URL.Path,
			"method":        r.Method,
			"tenant_id":     r.Header.Get("X-Tenant-ID"),
			"forwarded_for": r.Header.Get("X-Forwarded-For"),
			"timestamp":     time.Now().UTC().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": name})
	})

	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(map[string]string{"status": "slow", "service": name})
	})

	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "simulated error"})
	})

	addr := ":" + port
	log.Printf("[%s] listening on %s", name, addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
