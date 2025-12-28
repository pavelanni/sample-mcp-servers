// Quotes MCP Server
// A simple MCP server that provides random quotes and quote search functionality.
// Uses public APIs and local fallback data.
// Supports StreamableHTTP transport for gateway testing.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Quote database (fallback when API is unavailable)
var quotes = []Quote{
	{Text: "The only way to do great work is to love what you do.", Author: "Steve Jobs", Category: "motivation"},
	{Text: "Innovation distinguishes between a leader and a follower.", Author: "Steve Jobs", Category: "innovation"},
	{Text: "Stay hungry, stay foolish.", Author: "Steve Jobs", Category: "motivation"},
	{Text: "Life is what happens when you're busy making other plans.", Author: "John Lennon", Category: "life"},
	{Text: "The future belongs to those who believe in the beauty of their dreams.", Author: "Eleanor Roosevelt", Category: "motivation"},
	{Text: "It is during our darkest moments that we must focus to see the light.", Author: "Aristotle", Category: "wisdom"},
	{Text: "The only thing we have to fear is fear itself.", Author: "Franklin D. Roosevelt", Category: "courage"},
	{Text: "In the middle of difficulty lies opportunity.", Author: "Albert Einstein", Category: "wisdom"},
	{Text: "Imagination is more important than knowledge.", Author: "Albert Einstein", Category: "wisdom"},
	{Text: "Be the change you wish to see in the world.", Author: "Mahatma Gandhi", Category: "motivation"},
	{Text: "An eye for an eye only ends up making the whole world blind.", Author: "Mahatma Gandhi", Category: "wisdom"},
	{Text: "The best time to plant a tree was 20 years ago. The second best time is now.", Author: "Chinese Proverb", Category: "wisdom"},
	{Text: "Talk is cheap. Show me the code.", Author: "Linus Torvalds", Category: "programming"},
	{Text: "First, solve the problem. Then, write the code.", Author: "John Johnson", Category: "programming"},
	{Text: "Code is like humor. When you have to explain it, it's bad.", Author: "Cory House", Category: "programming"},
	{Text: "Simplicity is the soul of efficiency.", Author: "Austin Freeman", Category: "programming"},
	{Text: "Any fool can write code that a computer can understand. Good programmers write code that humans can understand.", Author: "Martin Fowler", Category: "programming"},
	{Text: "The most damaging phrase in the language is: We've always done it this way.", Author: "Grace Hopper", Category: "innovation"},
}

// Tool input/output types

type Quote struct {
	Text     string `json:"text"`
	Author   string `json:"author"`
	Category string `json:"category,omitempty"`
}

type GetRandomQuoteInput struct {
	Category string `json:"category,omitempty" jsonschema:"filter by category: motivation, wisdom, programming, innovation, life, courage"`
}

type SearchQuotesInput struct {
	Query string `json:"query" jsonschema:"search term to find in quotes or author names"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum number of results (default 5, max 10)"`
}

type SearchQuotesOutput struct {
	Quotes []Quote `json:"quotes"`
	Total  int     `json:"total"`
}

type ListCategoriesOutput struct {
	Categories []string `json:"categories"`
}

// Tool handlers

func getRandomQuote(_ context.Context, _ *mcp.CallToolRequest, input GetRandomQuoteInput) (*mcp.CallToolResult, Quote, error) {
	log.Printf("[DEBUG] get_random_quote tool called with input: category=%s", input.Category)

	// Try to fetch from ZenQuotes API first
	quote, err := fetchQuoteFromAPI()
	if err == nil && input.Category == "" {
		log.Printf("[DEBUG] Successfully fetched quote from API: author=%s", quote.Author)
		return nil, quote, nil
	}
	if err != nil {
		log.Printf("[DEBUG] API fetch failed, falling back to local quotes: %v", err)
	}

	// Fall back to local quotes
	var filteredQuotes []Quote
	if input.Category != "" {
		category := strings.ToLower(input.Category)
		log.Printf("[DEBUG] Filtering quotes by category: %s", category)
		for _, q := range quotes {
			if strings.ToLower(q.Category) == category {
				filteredQuotes = append(filteredQuotes, q)
			}
		}
		if len(filteredQuotes) == 0 {
			log.Printf("[ERROR] No quotes found for category: %s", input.Category)
			return nil, Quote{}, fmt.Errorf("no quotes found for category: %s", input.Category)
		}
		log.Printf("[DEBUG] Found %d quotes in category %s", len(filteredQuotes), category)
	} else {
		filteredQuotes = quotes
		log.Printf("[DEBUG] Using all %d local quotes", len(filteredQuotes))
	}

	// Return random quote
	idx := rand.Intn(len(filteredQuotes))
	selectedQuote := filteredQuotes[idx]
	log.Printf("[DEBUG] Selected random quote: author=%s, category=%s", selectedQuote.Author, selectedQuote.Category)
	return nil, selectedQuote, nil
}

func searchQuotes(_ context.Context, _ *mcp.CallToolRequest, input SearchQuotesInput) (*mcp.CallToolResult, SearchQuotesOutput, error) {
	log.Printf("[DEBUG] search_quotes tool called with input: query=%s, limit=%d", input.Query, input.Limit)

	if input.Query == "" {
		log.Printf("[ERROR] Query is required but was empty")
		return nil, SearchQuotesOutput{}, fmt.Errorf("query is required")
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 5
		log.Printf("[DEBUG] Limit was <= 0, using default: %d", limit)
	}
	if limit > 10 {
		limit = 10
		log.Printf("[DEBUG] Limit exceeded max, capping at: %d", limit)
	}

	query := strings.ToLower(input.Query)
	log.Printf("[DEBUG] Searching for query (case-insensitive): %s", query)
	var results []Quote

	for _, q := range quotes {
		if strings.Contains(strings.ToLower(q.Text), query) ||
			strings.Contains(strings.ToLower(q.Author), query) ||
			strings.Contains(strings.ToLower(q.Category), query) {
			results = append(results, q)
			log.Printf("[DEBUG] Match found: author=%s, category=%s", q.Author, q.Category)
			if len(results) >= limit {
				break
			}
		}
	}

	log.Printf("[DEBUG] Search completed: found %d results (limit was %d)", len(results), limit)
	return nil, SearchQuotesOutput{
		Quotes: results,
		Total:  len(results),
	}, nil
}

func listCategories(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, ListCategoriesOutput, error) {
	log.Printf("[DEBUG] list_categories tool called")

	categorySet := make(map[string]bool)
	for _, q := range quotes {
		if q.Category != "" {
			categorySet[q.Category] = true
		}
	}

	var categories []string
	for c := range categorySet {
		categories = append(categories, c)
	}

	log.Printf("[DEBUG] Found %d categories: %v", len(categories), categories)
	return nil, ListCategoriesOutput{Categories: categories}, nil
}

// Helper function to fetch from external API
func fetchQuoteFromAPI() (Quote, error) {
	log.Printf("[DEBUG] Fetching quote from ZenQuotes API...")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://zenquotes.io/api/random")
	if err != nil {
		log.Printf("[DEBUG] API request failed: %v", err)
		return Quote{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[ERROR] API returned non-OK status: %d", resp.StatusCode)
		return Quote{}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp []struct {
		Q string `json:"q"` // quote text
		A string `json:"a"` // author
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		log.Printf("[ERROR] Failed to decode API response: %v", err)
		return Quote{}, err
	}

	if len(apiResp) == 0 {
		log.Printf("[ERROR] Empty response from API")
		return Quote{}, fmt.Errorf("empty response from API")
	}

	log.Printf("[DEBUG] Successfully fetched quote from API: author=%s", apiResp[0].A)
	return Quote{
		Text:   apiResp[0].Q,
		Author: apiResp[0].A,
	}, nil
}

// corsMiddleware adds CORS headers and handles preflight requests
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[DEBUG] CORS Middleware: Request received: %s %s", r.Method, r.URL.Path)

		// Set CORS headers for all requests
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id, Content-Type, Cache-Control")

		// Handle preflight OPTIONS request
		if r.Method == "OPTIONS" {
			log.Printf("[DEBUG] CORS Middleware: Handling preflight OPTIONS request from %s", r.RemoteAddr)
			w.WriteHeader(http.StatusOK)
			log.Printf("[DEBUG] CORS Middleware: Sent 200 OK for preflight")
			return
		}

		log.Printf("[DEBUG] CORS Middleware: Passing request to next handler")
		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Define command-line flags
	portFlag := flag.String("port", "", "HTTP port to listen on (overrides QUOTES_SERVER_PORT env var)")
	corsFlag := flag.Bool("cors", true, "Enable CORS middleware (needed for browser-based clients like mcp-inspector)")
	flag.Parse()

	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	// Get port from command-line flag, environment, or use default
	port := *portFlag
	if port == "" {
		port = os.Getenv("QUOTES_SERVER_PORT")
		if port == "" {
			port = "8082"
		}
	}

	// Create MCP server
	log.Printf("[DEBUG] Creating MCP server...")
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "quotes-server",
			Version: "1.0.0",
		},
		nil,
	)
	log.Printf("[DEBUG] MCP server created: name=%s, version=%s", "quotes-server", "1.0.0")

	// Add tools
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_random_quote",
			Description: "Get a random inspirational quote, optionally filtered by category.",
		},
		getRandomQuote,
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "search_quotes",
			Description: "Search for quotes by keyword in the quote text, author name, or category.",
		},
		searchQuotes,
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "list_categories",
			Description: "List all available quote categories.",
		},
		listCategories,
	)
	log.Printf("[DEBUG] Tools added: get_random_quote, search_quotes, list_categories")

	// Create StreamableHTTP handler
	log.Printf("[DEBUG] Creating StreamableHTTP handler...")
	handler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server {
			log.Printf("[DEBUG] Server factory called for request from %s", r.RemoteAddr)
			return server
		},
		nil,
	)
	log.Printf("[DEBUG] StreamableHTTP handler created successfully")

	// Set up HTTP server
	mux := http.NewServeMux()
	if *corsFlag {
		mux.Handle("/mcp", corsMiddleware(handler))
		log.Printf("[DEBUG] Registered /mcp endpoint with CORS middleware")
	} else {
		mux.Handle("/mcp", handler)
		log.Printf("[DEBUG] Registered /mcp endpoint without CORS middleware")
	}

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[DEBUG] Health check endpoint called from %s", r.RemoteAddr)
		if *corsFlag {
			// Add CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":       "ok",
			"server":       "quotes-server",
			"version":      "1.0.0",
			"mcp_endpoint": "/mcp",
		})
	})
	log.Printf("[DEBUG] Registered /health endpoint")

	// Catch-all route to log unexpected requests
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[DEBUG] Unknown route accessed:")
		log.Printf("  Method: %s", r.Method)
		log.Printf("  Path: %s", r.URL.Path)
		log.Printf("  RemoteAddr: %s", r.RemoteAddr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "Not Found",
			"message": fmt.Sprintf("Path %s not found. Try /health or /mcp", r.URL.Path),
		})
	})

	addr := ":" + port
	log.Printf("========================================")
	log.Printf("Quotes MCP Server starting...")
	log.Printf("========================================")
	log.Printf("Address: %s", addr)
	log.Printf("Health endpoint: http://localhost%s/health", addr)
	log.Printf("MCP endpoint: http://localhost%s/mcp", addr)
	log.Printf("Available tools: get_random_quote, search_quotes, list_categories")
	log.Printf("========================================")
	log.Printf("[DEBUG] Starting HTTP server on %s...", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("[ERROR] Server failed to start: %v", err)
	}
}
