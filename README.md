# Brawl Chronicle

A simple tool that tracks new Magic: The Gathering cards legal in Brawl format daily using the Scryfall API and displays them on a GitHub Pages site.

## How it works

1. **Data Collection** (`cmd/fetcher/main.go`): Downloads Oracle cards from Scryfall's bulk API with caching
2. **Filtering**: Filters for cards legal in Brawl format only
3. **Comparison**: Compares with known cards to find new additions
4. **Efficient Storage**: Saves only card IDs to `data/history.json` (not full card data)
5. **Rendering** (`cmd/renderer/main.go`): Generates static HTML using cached Oracle cards for full details
6. **Publishing**: GitHub Actions deploys the site to GitHub Pages

## Structure

```
├── cmd/
│   ├── fetcher/
│   │   └── main.go           # Brawl card fetcher and processor
│   └── renderer/
│       └── main.go           # HTML generator
├── docs/
│   ├── index.html            # Generated site (created by renderer)
│   └── style.css             # Static CSS
├── data/
│   ├── history.json          # Efficient storage - card IDs only
│   └── oracle-cards.json     # Cached Oracle cards (gitignored)
└── .github/workflows/
    └── daily-check.yml       # Daily automation
```

## Setup

1. Fork this repository
2. Enable GitHub Pages in repository settings (set source to GitHub Actions)
3. The workflow will run automatically daily at 12:00 UTC
4. Manual runs: Go to Actions → Daily MTG Card Check → Run workflow

## Local Development

```bash
# Run data collection
go run cmd/fetcher/main.go

# Generate HTML from collected data
go run cmd/renderer/main.go data/history.json

# View the site
open docs/index.html
```

## Data

- Uses Scryfall's `oracle_cards` bulk data endpoint
- Filters for Brawl-legal cards only (`legalities.brawl == "legal"`)
- **Efficient Storage**: Only stores card IDs in history, not full card objects
- **Caching**: Oracle cards cached locally to avoid re-downloading
- History grows over time but remains lightweight (IDs only)
- Renderer reconstructs full card details from cached Oracle data

## Key Features

- **Memory Efficient**: Stores only card IDs (not full card data) in history
- **Smart Caching**: Reuses downloaded Oracle cards between fetcher and renderer  
- **Brawl Focused**: Filters specifically for Brawl format legality
- **Incremental Updates**: Only tracks newly added cards each day
- **Proper API Usage**: Includes required User-Agent and Accept headers

## GitHub Actions

The workflow:
1. Downloads Oracle cards data (with caching)
2. Filters for Brawl-legal cards
3. Compares with known cards from history
4. Updates history with new card IDs
5. Generates HTML using cached Oracle data
6. Commits changes
7. Deploys to GitHub Pages

Runs daily but can be triggered manually for testing.