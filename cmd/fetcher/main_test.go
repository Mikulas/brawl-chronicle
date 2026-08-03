package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindDefaultCardsDownloadSupportsLegacyJSON(t *testing.T) {
	info := BulkDataInfo{Data: []BulkDataItem{{
		Type:        "default_cards",
		DownloadURI: "https://example.com/default-cards.json",
	}}}

	download, err := findDefaultCardsDownload(info)
	if err != nil {
		t.Fatal(err)
	}
	if download.URL != "https://example.com/default-cards.json" || download.JSONLines {
		t.Fatalf("unexpected download: %#v", download)
	}
}

func TestFindDefaultCardsDownloadSupportsJSONL(t *testing.T) {
	info := BulkDataInfo{Data: []BulkDataItem{{
		Type:             "default_cards",
		JSONLDownloadURI: "https://example.com/default-cards.jsonl.gz",
	}}}

	download, err := findDefaultCardsDownload(info)
	if err != nil {
		t.Fatal(err)
	}
	if download.URL != "https://example.com/default-cards.jsonl.gz" || !download.JSONLines {
		t.Fatalf("unexpected download: %#v", download)
	}
}

func TestDecodeCardsSupportsArrayAndJSONL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		jsonLines bool
	}{
		{name: "array", input: `[{"id":"one"},{"id":"two"}]`},
		{name: "jsonl", input: "{\"id\":\"one\"}\n{\"id\":\"two\"}\n", jsonLines: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cards, err := decodeCards(strings.NewReader(test.input), test.jsonLines)
			if err != nil {
				t.Fatal(err)
			}
			if len(cards) != 2 || cards[0].ID != "one" || cards[1].ID != "two" {
				t.Fatalf("unexpected cards: %#v", cards)
			}
		})
	}
}

func TestDecodeCardsReportsInvalidJSONLRecord(t *testing.T) {
	_, err := decodeCards(strings.NewReader("{\"id\":\"one\"}\nnot-json\n"), true)
	if err == nil || !strings.Contains(err.Error(), "JSONL card 2") {
		t.Fatalf("expected second-record error, got %v", err)
	}
}

func TestJSONLCachePreservesRendererFields(t *testing.T) {
	input := `{"id":"one","oracle_id":"oracle-one","name":"Test Card","cmc":2,"colors":["U"],"image_uris":{"normal":"https://example.com/card.jpg"}}`
	cards, err := decodeCards(strings.NewReader(input), true)
	if err != nil {
		t.Fatal(err)
	}

	cacheFile := filepath.Join(t.TempDir(), "default-cards.json")
	if err := saveCards(cards, cacheFile); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		t.Fatal(err)
	}
	var cachedCards []Card
	if err := json.Unmarshal(data, &cachedCards); err != nil {
		t.Fatal(err)
	}

	if len(cachedCards) != 1 || cachedCards[0].CMC != 2 ||
		len(cachedCards[0].Colors) != 1 || cachedCards[0].Colors[0] != "U" ||
		cachedCards[0].ImageURIs["normal"] != "https://example.com/card.jpg" {
		t.Fatalf("renderer fields were not preserved: %#v", cachedCards)
	}
}

func TestFilterCompetitiveBrawlLegalCards(t *testing.T) {
	cards := []Card{
		{ID: "competitive", Legalities: map[string]string{"competitivebrawl": "legal", "brawl": "not_legal"}},
		{ID: "brawl-only", Legalities: map[string]string{"competitivebrawl": "not_legal", "brawl": "legal"}},
		{ID: "missing", Legalities: map[string]string{"brawl": "legal"}},
	}

	filtered := filterCompetitiveBrawlLegalCards(cards)
	if len(filtered) != 1 || filtered[0].ID != "competitive" {
		t.Fatalf("unexpected Competitive Brawl cards: %#v", filtered)
	}
}
