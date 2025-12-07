package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed static/*
var staticFiles embed.FS

type IPInfo struct {
	IP          string    `json:"ip"`
	City        string    `json:"city"`
	Country     string    `json:"country"`
	LastUpdated time.Time `json:"last_updated"`
	mu          sync.RWMutex
}

var serverIPInfo = &IPInfo{}

type IPAPIResponse struct {
	Query   string `json:"query"`
	City    string `json:"city"`
	Country string `json:"country"`
	Status  string `json:"status"`
}

func fetchIPInfo() (*IPInfo, error) {
	resp, err := http.Get("http://ip-api.com/json/")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch IP: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp IPAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if apiResp.Status != "success" {
		return nil, fmt.Errorf("API returned error status")
	}

	info := &IPInfo{
		IP:          apiResp.Query,
		City:        apiResp.City,
		Country:     apiResp.Country,
		LastUpdated: time.Now(),
	}

	log.Printf("IP fetched: %s (%s, %s)", info.IP, info.City, info.Country)
	return info, nil
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "File not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func handleServerIP(w http.ResponseWriter, r *http.Request) {
	info, err := fetchIPInfo()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	serverIPInfo.mu.Lock()
	serverIPInfo.IP = info.IP
	serverIPInfo.City = info.City
	serverIPInfo.Country = info.Country
	serverIPInfo.LastUpdated = info.LastUpdated
	serverIPInfo.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func handleClientIP(w http.ResponseWriter, r *http.Request) {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	if idx := strings.Index(clientIP, ":"); idx != -1 {
		clientIP = clientIP[:idx]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"ip": clientIP})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/server-ip", handleServerIP)
	http.HandleFunc("/api/client-ip", handleClientIP)

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
