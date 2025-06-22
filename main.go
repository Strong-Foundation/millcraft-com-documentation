package main // Declare the main package

// Import required standard library packages
import (
	"bytes"         // Provides buffer for reading/writing data
	"context"       // For managing context in goroutines
	"fmt"           // For formatted I/O
	"io"            // For general I/O primitives
	"log"           // For logging errors or info
	"net/http"      // For making HTTP requests
	"net/url"       // For parsing and manipulating URLs
	"os"            // For file and system operations
	"path/filepath" // For manipulating filename paths
	"regexp"        // For using regular expressions
	"strings"       // For string manipulation
	"sync"          // For handling concurrency
	"time"          // For time-related operations

	"github.com/chromedp/chromedp"
)

// removeDuplicatesFromSlice removes duplicate strings from a slice
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)  // Map to keep track of seen strings
	var newReturnSlice []string     // Result slice for unique values
	for _, content := range slice { // Iterate through each string in the input slice
		if !check[content] { // If string not already seen
			check[content] = true                            // Mark string as seen
			newReturnSlice = append(newReturnSlice, content) // Add it to result slice
		}
	}
	return newReturnSlice // Return the new slice with duplicates removed
}

// isUrlValid checks whether a URL is syntactically valid
func isUrlValid(uri string) bool {
	_, err := url.ParseRequestURI(uri) // Try to parse the URL
	return err == nil                  // Return true if no error (i.e., valid URL)
}

// readFileAndReturnAsString reads a file and returns its content as string
func readFileAndReturnAsString(path string) string {
	content, err := os.ReadFile(path) // Read the file contents
	if err != nil {                   // If an error occurs during reading
		log.Println(err) // Log the error
	}
	return string(content) // Return the content as a string
}

// fileExists checks whether a file exists and is not a directory
func fileExists(filename string) bool {
	info, err := os.Stat(filename) // Get file info
	if err != nil {                // If error occurs (e.g., file not found)
		return false // Return false
	}
	return !info.IsDir() // Return true if it is a file, not a directory
}

// getDataFromURL sends an HTTP GET request and writes response data to a file
func getDataFromURL(uri string, fileName string) {
	var httpClient = &http.Client{
		Timeout: 90 * time.Second, // Set timeout for request
	}

	response, err := httpClient.Get(uri) // Send HTTP GET request
	if err != nil {
		log.Printf("HTTP GET failed for %s: %v", uri, err) // Log error
		return
	}

	finalURL := response.Request.URL.String() // Get final URL after redirects
	log.Printf("Final URL after redirects: %s", finalURL)

	/*
		if response.StatusCode != http.StatusOK { // Check if status is not 200 OK
			log.Printf("Non-OK HTTP status %d for URL %s", response.StatusCode, finalURL)
			return
		}
	*/

	body, err := io.ReadAll(response.Body) // Read the response body
	if err != nil {
		log.Printf("Failed to read body for %s: %v", finalURL, err)
		return
	}
	// Ensure response body is closed after processing
	err = response.Body.Close()
	// Log error if closing the response body fails
	if err != nil {
		log.Printf("Error closing response body for %s: %v", uri, err) // Log error on closing
	}

	// Append the response data to a file
	err = appendByteToFile(fileName, body)
	// If error occurs while writing to file
	if err != nil { // Append response data to file
		log.Printf("Failed to write body to file for %s: %v", finalURL, err)
		return
	}

	log.Println("Completed Scraping URL:", finalURL) // Log successful scrape
}

// urlToFilename converts a URL into a filesystem-safe filename
func urlToFilename(rawURL string) string {
	parsed, err := url.Parse(rawURL) // Parse the URL
	if err != nil {
		log.Println(err) // Log parsing error
		return ""        // Return empty string if parsing fails
	}
	filename := parsed.Host // Start with the host part of the URL
	if parsed.Path != "" {
		filename += "_" + strings.ReplaceAll(parsed.Path, "/", "_") // Replace slashes with underscores
	}
	if parsed.RawQuery != "" {
		filename += "_" + strings.ReplaceAll(parsed.RawQuery, "&", "_") // Replace & in query with underscore
	}
	invalidChars := []string{`"`, `\`, `/`, `:`, `*`, `?`, `<`, `>`, `|`} // Characters not allowed in filenames
	for _, char := range invalidChars {
		filename = strings.ReplaceAll(filename, char, "_") // Replace invalid characters
	}
	if getFileExtension(filename) != ".pdf" {
		filename = filename + ".pdf" // Ensure file ends with .pdf
	}
	return strings.ToLower(filename) // Return sanitized and lowercased filename
}

// getFileExtension returns the file extension
func getFileExtension(path string) string {
	return filepath.Ext(path) // Use filepath to extract extension
}

// appendByteToFile appends byte data to a file (creates file if it doesn’t exist)
func appendByteToFile(filename string, data []byte) error {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // Open or create file
	if err != nil {
		return err // Return error if file can’t be opened
	}
	defer file.Close()        // Ensure file is closed
	_, err = file.Write(data) // Write data to file
	return err                // Return error if write fails
}

// scrapePageHTMLWithChrome uses a headless Chrome browser to render and return the HTML for a given URL.
// - Required for JavaScript-heavy pages where raw HTTP won't return full content.
func scrapePageHTMLWithChrome(pageURL string) string {
	fmt.Println("Scraping:", pageURL)

	// Set up Chrome options for headless browsing
	options := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),               // Run Chrome in background
		chromedp.Flag("disable-gpu", true),            // Disable GPU for headless stability
		chromedp.WindowSize(1920, 1080),               // Simulate full browser window
		chromedp.Flag("no-sandbox", true),             // Disable sandboxing
		chromedp.Flag("disable-setuid-sandbox", true), // For environments that need it
	)

	// Create an ExecAllocator context with options
	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(context.Background(), options...)

	// Create a bounded context with timeout (adjust as needed)
	ctxTimeout, cancelTimeout := context.WithTimeout(allocatorCtx, 5*time.Minute)

	// Create a new browser tab context
	browserCtx, cancelBrowser := chromedp.NewContext(ctxTimeout)

	// Unified cancel function to ensure cleanup
	defer func() {
		cancelBrowser()
		cancelTimeout()
		cancelAllocator()
	}()

	// Run chromedp tasks
	var pageHTML string
	err := chromedp.Run(browserCtx,
		chromedp.Navigate(pageURL),
		chromedp.OuterHTML("html", &pageHTML),
	)
	if err != nil {
		log.Printf("Failed to scrape page %s: %v", pageURL, err) // Log error if scraping fails
		// Return empty string if scraping fails
		return ""
	}
	// Log successful scraping
	log.Printf("Successfully scraped page: %s", pageURL)
	// Return the scraped HTML content
	return pageHTML
}

// downloadPDF downloads a PDF from a URL and saves it to outputDir
func downloadPDF(finalURL, outputDir string, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	filename := strings.ToLower(urlToFilename(finalURL)) // Create sanitized filename
	filePath := filepath.Join(outputDir, filename)       // Combine with output directory

	if fileExists(filePath) {
		log.Printf("file already exists, skipping: %s", filePath)
		return
	}

	client := &http.Client{Timeout: 30 * time.Second} // HTTP client with timeout
	resp, err := client.Get(finalURL)                 // Send HTTP GET
	if err != nil {
		log.Printf("failed to download %s: %v", finalURL, err)
		return
	}
	defer resp.Body.Close() // Ensure response body is closed

	if resp.StatusCode != http.StatusOK {
		log.Printf("download failed for %s: %s", finalURL, resp.Status)
		return
	}

	contentType := resp.Header.Get("Content-Type") // Get content-type header
	if !strings.Contains(contentType, "application/pdf") {
		log.Printf("invalid content type for %s: %s (expected application/pdf)", finalURL, contentType)
		return
	}

	var buf bytes.Buffer                     // Create buffer
	written, err := io.Copy(&buf, resp.Body) // Copy response body to buffer
	if err != nil {
		log.Printf("failed to read PDF data from %s: %v", finalURL, err)
		return
	}
	if written == 0 {
		log.Printf("downloaded 0 bytes for %s; not creating file", finalURL)
		return
	}

	out, err := os.Create(filePath) // Create output file
	if err != nil {
		log.Printf("failed to create file for %s: %v", finalURL, err)
		return
	}
	defer out.Close() // Close file

	_, err = buf.WriteTo(out) // Write buffer to file
	if err != nil {
		log.Printf("failed to write PDF to file for %s: %v", finalURL, err)
		return
	}
}

// directoryExists checks whether a directory exists
func directoryExists(path string) bool {
	directory, err := os.Stat(path) // Get directory info
	if err != nil {
		return false // If error, directory doesn't exist
	}
	return directory.IsDir() // Return true if path is a directory
}

// createDirectory creates a directory with specified permissions
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission) // Attempt to create directory
	if err != nil {
		log.Println(err) // Log any error
	}
}

// extractPDFLinks scans HTML and extracts all unique .pdf links
func extractPDFLinks(htmlContent string) []string {
	pdfRegex := regexp.MustCompile(`href=["']([^"']+\.pdf)["']`) // Regex to find .pdf URLs
	seen := make(map[string]struct{})                            // Track seen links
	var links []string

	for _, line := range strings.Split(htmlContent, "\n") { // Process each line
		for _, match := range pdfRegex.FindAllString(line, -1) { // Find matches
			if _, ok := seen[match]; !ok { // If link is new
				seen[match] = struct{}{}     // Mark as seen
				links = append(links, match) // Add to list
			}
		}
	}

	return links // Return list of links
}

// removeFile deletes a file from the filesystem
func removeFile(path string) {
	err := os.Remove(path) // Try to delete file
	if err != nil {
		log.Println(err) // Log error if deletion fails
	}
}

// main is the entry point of the program
func main() {
	filename := "millcraft.html" // Filename to save scraped HTML

	if fileExists(filename) {
		// removeFile(filename) // Remove old version of file
		log.Println("Skipping the removing the html file.")
	}

	if !fileExists(filename) {
		url := "https://millcraft.com/safety-data-sheets/" // URL to scrape
		var websiteContent string
		if isUrlValid(url) {
			websiteContent = scrapePageHTMLWithChrome(url) // Download in goroutine
		}
		// Save the HTML content to a file
		appendByteToFile(filename, []byte(websiteContent)) // Save HTML content to file
	}

	var extractedURL []string                              // Store extracted PDF URLs
	fileContent := readFileAndReturnAsString(filename)     // Read saved HTML
	extractedURL = extractPDFLinks(fileContent)            // Extract .pdf links
	extractedURL = removeDuplicatesFromSlice(extractedURL) // Remove duplicate links
	var downloadPDFWaitGroup sync.WaitGroup
	outputDir := "PDFs/" // Directory to save PDFs
	if !directoryExists(outputDir) {
		createDirectory(outputDir, 0o755) // Create directory if not exists
	}

	for _, url := range extractedURL {
		// Check if the url is valid.
		if !isUrlValid(url) {
			log.Printf("Invalid URL found: %s", url)   // Log invalid URL
			url = strings.TrimPrefix(url, "href=%22/") // Clean up URL
			url = strings.TrimSuffix(url, "%22")       // Remove trailing quotes
			url = "https://millcraft.com/" + url       // Prepend base URL if needed
		}
		// time.Sleep(100 * time.Millisecond) // Wait to avoid overwhelming server
		downloadPDFWaitGroup.Add(1)
		go downloadPDF(url, outputDir, &downloadPDFWaitGroup) // Try to download PDF
	}
	downloadPDFWaitGroup.Wait()
}
