package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	text_template "text/template" // Use text/template for RSS generation to avoid HTML escaping
	"time"
)

// Full card data structure for rendering
type Card struct {
	ID         string            `json:"id"`
	OracleID   string            `json:"oracle_id"`
	Name       string            `json:"name"`
	ManaCost   string            `json:"mana_cost"`
	CMC        float64           `json:"cmc"`
	TypeLine   string            `json:"type_line"`
	Colors     []string          `json:"colors"`
	Rarity     string            `json:"rarity"`
	SetName    string            `json:"set_name"`
	Legalities map[string]string `json:"legalities"`
	ImageURIs  map[string]string `json:"image_uris"`
	Games      []string          `json:"games"`
}

// Updated data structure to match fetcher oracle format
type DayResult struct {
	Date         string            `json:"date"`
	AddedOracles []string          `json:"added_oracles"`
	CardMapping  map[string]string `json:"card_mapping"`
	TotalCards   int               `json:"total_cards"`
	FirstRun     bool              `json:"first_run"`
	
	// Legacy support for old format
	AddedCards []string `json:"added_cards"`
}

type HistoryData struct {
	Days []DayResult `json:"days"`
}

// Helper struct for template rendering
type DisplayCard struct {
	ID          string
	Name        string
	ImageURL    string
	ScryfallURL string
	Colors      []string
	CMC         float64
}

type DisplayDay struct {
	Date       string
	Cards      []DisplayCard
	TotalCards int
	FirstRun   bool
}

type DisplayData struct {
	Days []DisplayDay
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run render.go <history.json>")
		os.Exit(1)
	}

	historyFile := os.Args[1]
	outputDir := "docs"

	// Load history
	history, err := loadHistory(historyFile)
	if err != nil {
		fmt.Printf("Error loading history: %v\n", err)
		os.Exit(1)
	}

	// Load default cards from cached file
	fmt.Println("Loading default cards from cache...")
	artworkCards, err := loadOracleCards("data/default-cards.json")
	if err != nil {
		fmt.Printf("Error loading default cards: %v\n", err)
		os.Exit(1)
	}

	// Create card lookup map with Arena preference
	cardLookup := make(map[string]Card)
	
	// Group cards by ID and prefer Arena versions
	for _, card := range artworkCards {
		existing, exists := cardLookup[card.ID]
		if !exists || (hasArena(card.Games) && !hasArena(existing.Games)) {
			cardLookup[card.ID] = card
		}
	}

	// Create output directory
	os.MkdirAll(outputDir, 0755)

	// Generate HTML
	if err := generateHTML(history, cardLookup, outputDir); err != nil {
		fmt.Printf("Error generating HTML: %v\n", err)
		os.Exit(1)
	}

	// Generate RSS feed
	if err := generateRSS(history, cardLookup, outputDir); err != nil {
		fmt.Printf("Error generating RSS: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("HTML and RSS generated in %s/\n", outputDir)
}

func loadHistory(filename string) (HistoryData, error) {
	file, err := os.Open(filename)
	if err != nil {
		return HistoryData{}, err
	}
	defer file.Close()

	var history HistoryData
	if err := json.NewDecoder(file).Decode(&history); err != nil {
		return HistoryData{}, err
	}

	return history, nil
}

func loadOracleCards(filename string) ([]Card, error) {
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

func generateHTML(history HistoryData, cardLookup map[string]Card, outputDir string) error {
	// Convert to display format
	displayData := convertToDisplayData(history, cardLookup)

	// Sort days in reverse chronological order (newest first)
	sort.Slice(displayData.Days, func(i, j int) bool {
		return displayData.Days[i].Date > displayData.Days[j].Date
	})

	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Brawl Chronicle</title>
    <link rel="stylesheet" href="style.css">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
    <link rel="alternate" type="application/rss+xml" title="Brawl Chronicle RSS Feed" href="feed.xml">
</head>
<body>
    <div class="header">
        <h1>Brawl Chronicle</h1>
        <p>Daily tracking of new Magic: The Gathering cards legal in Brawl format</p>
        <div class="links">
            <a href="feed.xml" title="RSS Feed" class="header-link">
                <i class="fas fa-rss"></i> RSS Feed
            </a>
            <a href="https://github.com/Mikulas/brawl-chronicle" target="_blank" title="GitHub Project" class="header-link">
                <i class="fab fa-github"></i> GitHub
            </a>
        </div>
        {{if .Days}}
        <div class="last-updated">Last updated: {{(index .Days 0).Date}}</div>
        {{end}}
    </div>

    {{range .Days}}
    {{if or .FirstRun (gt (len .Cards) 0)}}
    <div class="day">
        <div class="day-header">
            <div class="date">{{.Date}}</div>
            <div class="count">
                {{if .FirstRun}}
                First Run - {{thousands .TotalCards}} cards
                {{else}}
                {{thousands (len .Cards)}} new cards
                {{end}}
            </div>
        </div>
        
        {{if .FirstRun}}
        <div class="first-run">
            Initial data collection - {{thousands .TotalCards}} Brawl-legal cards in database
        </div>
        {{else}}
        <div class="cards">
            {{range .Cards}}
            {{if .ImageURL}}
            <div class="card">
                <a href="{{.ScryfallURL}}" target="_blank" title="{{.Name}}">
                    <img src="{{.ImageURL}}" alt="{{.Name}}" loading="lazy">
                </a>
            </div>
            {{end}}
            {{end}}
        </div>
        {{end}}
    </div>
    {{end}}
    {{end}}

    {{if not .Days}}
    <div class="no-cards">
        No data available yet.
    </div>
    {{end}}
</body>
</html>`

	// Create template with custom functions
	funcMap := template.FuncMap{
		"thousands": addThousandsSeparator,
	}
	
	t, err := template.New("index").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(outputDir, "index.html"))
	if err != nil {
		return err
	}
	defer file.Close()

	return t.Execute(file, displayData)
}

func convertToDisplayData(history HistoryData, cardLookup map[string]Card) DisplayData {
	var displayDays []DisplayDay

	for _, day := range history.Days {
		var cards []DisplayCard
		
		// Only process individual cards if it's NOT a first run
		if !day.FirstRun {
			// Handle both new oracle format and legacy format
			var cardIDs []string
			
			if day.AddedOracles != nil {
				// New oracle-based format: select best card for each oracle_id
				for _, oracleID := range day.AddedOracles {
					if bestCard, found := selectBestCard(oracleID, cardLookup); found {
						cardIDs = append(cardIDs, bestCard.ID)
					}
				}
			} else if day.AddedCards != nil {
				// Legacy format: use AddedCards directly
				cardIDs = day.AddedCards
			}
			
			// Convert IDs to full card data
			for _, id := range cardIDs {
				if card, exists := cardLookup[id]; exists {
					// Get image URL (prefer normal, fallback to large, then small)
					imageURL := ""
					if card.ImageURIs != nil {
						if url, ok := card.ImageURIs["normal"]; ok {
							imageURL = url
						} else if url, ok := card.ImageURIs["large"]; ok {
							imageURL = url
						} else if url, ok := card.ImageURIs["small"]; ok {
							imageURL = url
						}
					}
					
					// Build Scryfall URL
					scryfallURL := fmt.Sprintf("https://scryfall.com/card/%s", card.ID)
					
					cards = append(cards, DisplayCard{
						ID:          card.ID,
						Name:        card.Name,
						ImageURL:    imageURL,
						ScryfallURL: scryfallURL,
						Colors:      card.Colors,
						CMC:         card.CMC,
					})
				} else {
					// If card not found, show just the ID
					cards = append(cards, DisplayCard{
						ID:   id,
						Name: "Unknown Card",
					})
				}
			}

			// Sort cards by Wizards style: color order then CMC then name
			sort.Slice(cards, func(i, j int) bool {
				return compareCardsWizardsStyle(cards[i], cards[j])
			})
		}
		// For first run, cards slice stays empty

		displayDays = append(displayDays, DisplayDay{
			Date:       day.Date,
			Cards:      cards,
			TotalCards: day.TotalCards,
			FirstRun:   day.FirstRun,
		})
	}

	return DisplayData{Days: displayDays}
}

// getColorOrder returns the priority for Wizards color ordering (WUBRG + multicolor + colorless)
func getColorOrder(colors []string) int {
	if len(colors) == 0 {
		return 6 // Colorless
	}
	if len(colors) > 1 {
		return 5 // Multicolor
	}
	
	// Single color in WUBRG order
	switch colors[0] {
	case "W":
		return 0 // White
	case "U":
		return 1 // Blue
	case "B":
		return 2 // Black
	case "R":
		return 3 // Red
	case "G":
		return 4 // Green
	default:
		return 6 // Unknown/Colorless
	}
}

// compareCardsWizardsStyle compares cards in Wizards style: color order, then CMC, then name
func compareCardsWizardsStyle(a, b DisplayCard) bool {
	// First, compare by color order
	colorOrderA := getColorOrder(a.Colors)
	colorOrderB := getColorOrder(b.Colors)
	
	if colorOrderA != colorOrderB {
		return colorOrderA < colorOrderB
	}
	
	// If same color category, compare by CMC
	if a.CMC != b.CMC {
		return a.CMC < b.CMC
	}
	
	// If same CMC, compare by name
	return a.Name < b.Name
}

// addThousandsSeparator adds commas to numbers for better readability
func addThousandsSeparator(n int) string {
	str := strconv.Itoa(n)
	if len(str) <= 3 {
		return str
	}
	
	var result strings.Builder
	for i, digit := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(digit)
	}
	return result.String()
}

// hasArena checks if a card is available on Arena
func hasArena(games []string) bool {
	for _, game := range games {
		if game == "arena" {
			return true
		}
	}
	return false
}

// selectBestCard chooses the best card for an oracle_id (prefer Arena, then regular frames)
func selectBestCard(oracleID string, cardLookup map[string]Card) (Card, bool) {
	var candidates []Card
	
	// Find all cards with this oracle_id
	for _, card := range cardLookup {
		if card.OracleID == oracleID {
			candidates = append(candidates, card)
		}
	}
	
	if len(candidates) == 0 {
		return Card{}, false
	}
	
	// Step 1: Filter for Arena versions if available
	var arenaCards []Card
	for _, card := range candidates {
		if hasArena(card.Games) {
			arenaCards = append(arenaCards, card)
		}
	}
	
	// Use Arena cards if we found any, otherwise use all candidates
	finalCandidates := candidates
	if len(arenaCards) > 0 {
		finalCandidates = arenaCards
	}
	
	// Step 2: Prefer regular frames over special printings
	// Look for cards without "showcase", "borderless", "etched", etc in the ID or special frames
	var regularFrames []Card
	for _, card := range finalCandidates {
		// Simple heuristic: prefer cards that don't have special frame indicators
		cardID := strings.ToLower(card.ID)
		if !strings.Contains(cardID, "showcase") && 
		   !strings.Contains(cardID, "borderless") && 
		   !strings.Contains(cardID, "etched") &&
		   !strings.Contains(cardID, "extended") {
			regularFrames = append(regularFrames, card)
		}
	}
	
	// Use regular frames if we found any, otherwise use final candidates
	if len(regularFrames) > 0 {
		return regularFrames[0], true
	}
	
	return finalCandidates[0], true
}

func generateRSS(history HistoryData, cardLookup map[string]Card, outputDir string) error {
	// Convert to display format
	displayData := convertToDisplayData(history, cardLookup)
	
	// Sort days in reverse chronological order (newest first)
	sort.Slice(displayData.Days, func(i, j int) bool {
		return displayData.Days[i].Date > displayData.Days[j].Date
	})

	rssTemplate := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
	<channel>
		<title>Brawl Chronicle</title>
		<link>https://mikulas.github.io/brawl-chronicle/</link>
		<description>Daily tracking of new Magic: The Gathering cards legal in Brawl format</description>
		<language>en-us</language>
		<lastBuildDate>{{.LastUpdate}}</lastBuildDate>
		{{range .Days}}{{if or .FirstRun (gt (len .Cards) 0)}}
		<item>
			<title>{{if .FirstRun}}Initial Collection - {{thousands .TotalCards}} cards{{else}}{{thousands (len .Cards)}} new cards on {{.Date}}{{end}}</title>
			<link>https://mikulas.github.io/brawl-chronicle/#{{.Date}}</link>
			<guid>https://mikulas.github.io/brawl-chronicle/#{{.Date}}</guid>
			<pubDate>{{.PubDate}}</pubDate>
			<description><![CDATA[
				{{if .FirstRun}}
				Initial data collection - {{thousands .TotalCards}} Brawl-legal cards in database
				{{else}}
				{{range .Cards}}{{if .ImageURL}}<p><strong>{{.Name}}</strong><br/><img src="{{.ImageURL}}" alt="{{.Name}}" style="max-width:200px;"/></p>{{end}}{{end}}
				{{end}}
			]]></description>
		</item>
		{{end}}{{end}}
	</channel>
</rss>`

	// Create template with custom functions using text/template for proper XML output
	textFuncMap := text_template.FuncMap{
		"thousands": addThousandsSeparator,
	}
	
	t, err := text_template.New("rss").Funcs(textFuncMap).Parse(rssTemplate)
	if err != nil {
		return err
	}

	// Create RSS data with proper dates
	type RSSDay struct {
		DisplayDay
		PubDate string
	}
	
	type RSSData struct {
		Days       []RSSDay
		LastUpdate string
	}
	
	var rssDays []RSSDay
	for _, day := range displayData.Days {
		// Convert date to RFC2822 format for RSS
		date, err := time.Parse("2006-01-02", day.Date)
		if err != nil {
			date = time.Now() // fallback
		}
		
		rssDays = append(rssDays, RSSDay{
			DisplayDay: day,
			PubDate:    date.Format(time.RFC1123Z),
		})
	}
	
	rssData := RSSData{
		Days:       rssDays,
		LastUpdate: time.Now().Format(time.RFC1123Z),
	}

	// Write RSS file
	rssFile := filepath.Join(outputDir, "feed.xml")
	file, err := os.Create(rssFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// Execute template and write raw XML (text/template doesn't escape HTML)
	return t.Execute(file, rssData)
}