package main

import (
	"strings"
	"testing"
)

func TestValidateCardImagesRejectsMissingMetadata(t *testing.T) {
	displayData := DisplayData{Days: []DisplayDay{{
		Date:  "2026-08-03",
		Cards: []DisplayCard{{ID: "one"}, {ID: "two"}},
	}}}

	err := validateCardImages(displayData)
	if err == nil || !strings.Contains(err.Error(), "without any image URLs") {
		t.Fatalf("expected missing-image error, got %v", err)
	}
}

func TestValidateCardImagesAllowsRenderedCards(t *testing.T) {
	displayData := DisplayData{Days: []DisplayDay{{
		Date:  "2026-08-03",
		Cards: []DisplayCard{{ID: "one", ImageURL: "https://example.com/card.jpg"}},
	}}}

	if err := validateCardImages(displayData); err != nil {
		t.Fatal(err)
	}
}
