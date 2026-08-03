package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BulkDataInfo struct {
	Data []BulkDataItem `json:"data"`
}

type BulkDataItem struct {
	Type             string `json:"type"`
	DownloadURI      string `json:"download_uri"`
	JSONLDownloadURI string `json:"jsonl_download_uri"`
	UpdatedAt        string `json:"updated_at"`
}

type BulkDownload struct {
	URL       string
	JSONLines bool
}

type Card struct {
	ID         string            `json:"id"`
	OracleID   string            `json:"oracle_id"`
	Name       string            `json:"name"`
	CMC        float64           `json:"cmc"`
	Colors     []string          `json:"colors"`
	Legalities map[string]string `json:"legalities"`
	ImageURIs  map[string]string `json:"image_uris"`
	Games      []string          `json:"games"`
}

// Oracle-based data structure - track oracle_ids for unique cards
type DayResult struct {
	Date         string   `json:"date"`
	AddedOracles []string `json:"added_oracles"` // oracle_ids of new cards
	TotalCards   int      `json:"total_cards"`
	FirstRun     bool     `json:"first_run"`

	// Legacy support for old format
	AddedCards []string `json:"added_cards"`
}

type HistoryData struct {
	Days []DayResult `json:"days"`
}

func main() {
	dataDir := "data"
	resultsDir := filepath.Join(dataDir, "results")
	oracleFile := filepath.Join(dataDir, "default-cards.json")

	os.MkdirAll(resultsDir, 0755)

	historyFile := filepath.Join(dataDir, "history.json")

	// Check if we already have default cards cached and if it's fresh (less than 23 hours old)
	var currentCards []Card
	shouldDownload := true

	if stat, err := os.Stat(oracleFile); err == nil {
		// Check if cache is less than 23 hours old
		cacheAge := time.Since(stat.ModTime())
		if cacheAge < 23*time.Hour {
			fmt.Printf("Using cached default cards data (%.1f hours old)\n", cacheAge.Hours())
			shouldDownload = false
		} else {
			fmt.Printf("Cache is %.1f hours old, refreshing...\n", cacheAge.Hours())
		}
	}

	if shouldDownload {
		// Download and cache default cards
		fmt.Println("Fetching Scryfall bulk data info...")
		download, err := getDownload()
		if err != nil {
			fmt.Printf("Error getting download URL: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Downloading from: %s\n", download.URL)
		currentCards, err = downloadCards(download)
		if err != nil {
			fmt.Printf("Error downloading cards: %v\n", err)
			os.Exit(1)
		}

		// Keep the cache as a JSON array regardless of Scryfall's download format.
		fmt.Println("Saving default cards to cache...")
		if err := saveCards(currentCards, oracleFile); err != nil {
			fmt.Printf("Error saving default cards: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Downloaded %d cards\n", len(currentCards))
	} else {
		// Load cached default cards
		fmt.Println("Loading cached default cards...")
		var err error
		currentCards, err = loadRawCards(oracleFile)
		if err != nil {
			fmt.Printf("Error loading cached default cards: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Loaded %d cards from cache\n", len(currentCards))
	}

	// Filter for Brawl-legal cards and build oracle_id mapping
	brawlCards := filterBrawlLegalCards(currentCards)
	fmt.Printf("Found %d Brawl-legal cards\n", len(brawlCards))

	// Build oracle_id to best card mapping (prefer Arena)
	oracleToCard := buildOracleMapping(brawlCards)
	fmt.Printf("Unique oracle cards: %d\n", len(oracleToCard))

	// Load existing history
	history := loadHistory(historyFile)

	// Build set of all known oracle_ids from history
	knownOracles := buildKnownOraclesFromHistory(history)

	// Check if this is first run (no history or transitioning from old format)
	if len(history.Days) == 0 || len(knownOracles) == 0 {
		fmt.Println("First run - initializing with all current oracle cards")

		// On first run, add all current oracle_ids
		var addedOracles []string

		for oracleID := range oracleToCard {
			addedOracles = append(addedOracles, oracleID)
		}

		result := DayResult{
			Date:         time.Now().UTC().Format("2006-01-02"),
			AddedOracles: addedOracles,
			TotalCards:   len(oracleToCard),
			FirstRun:     true,
		}

		// Clear history for fresh start with oracle-based format
		history.Days = []DayResult{result}
	} else {
		// Find new oracle_ids (in current but not in our known set)
		fmt.Println("Comparing with known oracle cards...")
		newOracles := findNewOracles(knownOracles, oracleToCard)

		fmt.Printf("Found %d new oracle cards\n", len(newOracles))

		// Only add entry if there are new cards or if it's been more than a day since last entry
		shouldAddEntry := len(newOracles) > 0

		// Also add entry if last entry was yesterday or earlier (to track total count changes)
		if len(history.Days) > 0 {
			lastDate := history.Days[len(history.Days)-1].Date
			today := time.Now().UTC().Format("2006-01-02")
			if lastDate != today {
				shouldAddEntry = true
			}
		}

		if shouldAddEntry {
			var addedOracles []string

			for _, oracleID := range newOracles {
				addedOracles = append(addedOracles, oracleID)
			}

			result := DayResult{
				Date:         time.Now().UTC().Format("2006-01-02"),
				AddedOracles: addedOracles,
				TotalCards:   len(oracleToCard),
				FirstRun:     false,
			}

			// Remove existing entry for today if it exists
			history = removeEntryForToday(history)
			history.Days = append(history.Days, result)

			fmt.Printf("Added entry with %d new oracle cards\n", len(newOracles))
		} else {
			fmt.Println("No new oracle cards and already have entry for today")
		}
	}

	// Save history
	if err := saveHistory(history, historyFile); err != nil {
		fmt.Printf("Error saving history: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Data updated. History saved to %s\n", historyFile)
}

func getDownload() (BulkDownload, error) {
	req, err := http.NewRequest("GET", "https://api.scryfall.com/bulk-data", nil)
	if err != nil {
		return BulkDownload{}, err
	}

	req.Header.Set("User-Agent", "BrawlChronicle/1.0")
	req.Header.Set("Accept", "application/json;q=0.9,*/*;q=0.8")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return BulkDownload{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return BulkDownload{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var bulkInfo BulkDataInfo
	if err := json.NewDecoder(resp.Body).Decode(&bulkInfo); err != nil {
		return BulkDownload{}, err
	}
	return findDefaultCardsDownload(bulkInfo)
}

func findDefaultCardsDownload(bulkInfo BulkDataInfo) (BulkDownload, error) {
	for _, data := range bulkInfo.Data {
		if data.Type == "default_cards" {
			if data.DownloadURI != "" {
				return BulkDownload{URL: data.DownloadURI}, nil
			}
			if data.JSONLDownloadURI != "" {
				return BulkDownload{URL: data.JSONLDownloadURI, JSONLines: true}, nil
			}
			return BulkDownload{}, fmt.Errorf("default_cards has no download URI")
		}
	}

	return BulkDownload{}, fmt.Errorf("default_cards not found in bulk data")
}

func downloadCards(download BulkDownload) ([]Card, error) {
	if strings.TrimSpace(download.URL) == "" {
		return nil, fmt.Errorf("download URL is empty")
	}

	req, err := http.NewRequest("GET", download.URL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "BrawlChronicle/1.0")
	req.Header.Set("Accept", "application/json;q=0.9,*/*;q=0.8")

	client := &http.Client{Timeout: 15 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var reader io.Reader = resp.Body
	if !resp.Uncompressed && isGzipResponse(resp, download.URL) {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("opening gzip download: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	return decodeCards(reader, download.JSONLines)
}

func isGzipResponse(resp *http.Response, downloadURL string) bool {
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	contentEncoding := strings.ToLower(resp.Header.Get("Content-Encoding"))
	parsedURL, err := url.Parse(downloadURL)
	return contentEncoding == "gzip" || strings.Contains(contentType, "application/gzip") ||
		(err == nil && strings.HasSuffix(strings.ToLower(parsedURL.Path), ".gz"))
}

func decodeCards(reader io.Reader, jsonLines bool) ([]Card, error) {
	decoder := json.NewDecoder(reader)
	if !jsonLines {
		var cards []Card
		if err := decoder.Decode(&cards); err != nil {
			return nil, fmt.Errorf("decoding card array: %w", err)
		}
		return cards, nil
	}

	var cards []Card
	for {
		var card Card
		if err := decoder.Decode(&card); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decoding JSONL card %d: %w", len(cards)+1, err)
		}
		cards = append(cards, card)
	}
	return cards, nil
}

func loadCards(filename string) ([]Card, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cards []Card
	if err := json.NewDecoder(file).Decode(&cards); err != nil {
		return nil, err
	}

	return cards, nil
}

func saveCards(cards []Card, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	if err := json.NewEncoder(file).Encode(cards); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func loadRawCards(filename string) ([]Card, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cards []Card
	if err := json.Unmarshal(data, &cards); err != nil {
		return nil, err
	}

	return cards, nil
}

func loadHistory(filename string) HistoryData {
	file, err := os.Open(filename)
	if err != nil {
		return HistoryData{Days: []DayResult{}}
	}
	defer file.Close()

	var history HistoryData
	json.NewDecoder(file).Decode(&history)
	return history
}

func saveHistory(history HistoryData, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(history)
}

// Build set of all known cards from history (legacy - for old format support)
func buildKnownCardsFromHistory(history HistoryData) map[string]bool {
	knownCards := make(map[string]bool)
	// This function is kept for compatibility but not used in oracle-based tracking
	return knownCards
}

// Build oracle_id to best card mapping (prefer Arena)
func buildOracleMapping(cards []Card) map[string]Card {
	oracleToCard := make(map[string]Card)

	for _, card := range cards {
		existing, exists := oracleToCard[card.OracleID]
		if !exists || (hasArenaInFetcher(card.Games) && !hasArenaInFetcher(existing.Games)) {
			oracleToCard[card.OracleID] = card
		}
	}

	return oracleToCard
}

func buildKnownOraclesFromHistory(history HistoryData) map[string]bool {
	known := make(map[string]bool)
	for _, day := range history.Days {
		// Handle new format (AddedOracles)
		if day.AddedOracles != nil {
			for _, oracleID := range day.AddedOracles {
				known[oracleID] = true
			}
		}
		// For old data with AddedCards, we'll treat this as a fresh start
	}
	return known
}

func findNewOracles(knownOracles map[string]bool, oracleToCard map[string]Card) []string {
	var newOracles []string
	for oracleID := range oracleToCard {
		if !knownOracles[oracleID] {
			newOracles = append(newOracles, oracleID)
		}
	}
	return newOracles
}

func hasArenaInFetcher(games []string) bool {
	for _, game := range games {
		if game == "arena" {
			return true
		}
	}
	return false
}

func removeEntryForToday(history HistoryData) HistoryData {
	today := time.Now().UTC().Format("2006-01-02")
	var filteredDays []DayResult

	for _, day := range history.Days {
		if day.Date != today {
			filteredDays = append(filteredDays, day)
		}
	}

	history.Days = filteredDays
	return history
}

func findNewCards(knownCards map[string]bool, currentCards []Card) []Card {
	var newCards []Card

	for _, card := range currentCards {
		if !knownCards[card.ID] {
			newCards = append(newCards, card)
		}
	}

	return newCards
}

func filterBrawlLegalCards(cards []Card) []Card {
	var brawlCards []Card

	for _, card := range cards {
		// Check if card is legal in brawl
		if legality, exists := card.Legalities["brawl"]; exists && legality == "legal" {
			brawlCards = append(brawlCards, card)
		}
	}

	return brawlCards
}
