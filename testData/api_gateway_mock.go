//go:build mock
// +build mock

package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	apiGatewayHostAndPort := os.Getenv("API_BASE_URL")
	if apiGatewayHostAndPort == "" {
		apiGatewayHostAndPort = "localhost:8090"
	}

	srv := startAPIGatewayMock(apiGatewayHostAndPort)

	//TODO Setup a channel to listen to interrupt signals and implement interrupt signal
	// when tests pass from the test-runner container
	// Block the main function for 8 minutes to run the Go Routine mock server
	time.Sleep(5 * time.Minute)

	// Shutdown the API GateWay server after the test completes
	defer func() {
		// Create a context with a timeout to ensure the server shuts down
		log.Println("API Gateway Mock Server Stopping...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Shutdown the server
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Server Shutdown Failed: %+v", err)
		}
	}()
}

func startAPIGatewayMock(hostname string) *http.Server {

	handler := func(w http.ResponseWriter, r *http.Request) {
		// Use this response to validate that the request was hitting the mock service
		// fmt.Fprintln(w, "Request Received By The API Gateway Mock")
		if r.URL.Path != "/t800/a" && r.URL.Path != "/t800/policy" && r.URL.Path != "/t800-healthcheck" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/t800/a" {
			//log.Println("Check API Key of the request...")
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/t800-healthcheck" {
			//log.Println("API Gateway Mock is Running - Respond to HealthCheck...")
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.URL.Path == "/t800/policy" {
			//log.Printf("API Gateway Mock call to get policies:  %v", r)

			// Unfortunately API-Gateway receives only API Key header
			// So we need to map X-Api-Key to X-Policy in the mock
			xKey := r.Header.Get("X-Api-Key")
			// If the API Key does not start with 123456 or "test-key" it is not valid :)
			if strings.Index(xKey, "123456") != 0 && strings.Index(xKey, "test-key") != 0 {
				w.WriteHeader(http.StatusForbidden) // 404 Forbidden
				return
			}
			w.WriteHeader(http.StatusOK)
			// Use for x-policy the rest of the API-Key after the test prefix
			response := map[string]string{"x-policy": strings.TrimPrefix(xKey, "123456")}
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(response)
			log.Printf("API Gateway Mock Returned X-Policy: %v", response)
			if err != nil {
				log.Printf("Failed to encode response: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}

	r := mux.NewRouter()
	r.HandleFunc("/t800/a", handler)
	r.HandleFunc("/t800/policy", handler)
	r.HandleFunc("/t800-healthcheck", handler)

	// Create the http.Server
	srv := &http.Server{
		Addr:    hostname,
		Handler: r,
	}

	// Start the API GateWay HTTP server in a Routine
	log.Println("API Gateway Mock Server Starting...")
	go func() {
		log.Printf("API Gateway Mock Server Started and it listens to: http://%v", hostname)
		err := srv.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to start API Gateway Mock server: %v", err)
			return
		}
	}()

	return srv
}
