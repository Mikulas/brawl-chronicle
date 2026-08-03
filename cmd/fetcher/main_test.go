package main

import (
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
