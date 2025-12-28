// Moon Phase MCP Server
// A simple MCP server that provides moon phase information using public APIs.
// Supports StreamableHTTP transport for gateway testing.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool input/output types

type GetMoonPhaseInput struct {
	Date string `json:"date,omitempty" jsonschema:"date in YYYY-MM-DD format, defaults to today"`
}

type MoonPhaseOutput struct {
	Date          string  `json:"date"`
	Phase         string  `json:"phase"`
	Illumination  float64 `json:"illumination"`
	DaysUntilFull int     `json:"days_until_full"`
	Emoji         string  `json:"emoji"`
}

type GetMoonCalendarInput struct {
	Month int `json:"month" jsonschema:"month number (1-12)"`
	Year  int `json:"year" jsonschema:"year (e.g., 2025)"`
}

type MoonCalendarOutput struct {
	Month    int    `json:"month"`
	Year     int    `json:"year"`
	NewMoon  string `json:"new_moon"`
	FirstQtr string `json:"first_quarter"`
	FullMoon string `json:"full_moon"`
	LastQtr  string `json:"last_quarter"`
}

// Moon phase calculation (simplified algorithm)
func calculateMoonPhase(t time.Time) (string, float64, string) {
	// Simplified moon phase calculation
	// Based on the synodic month (29.53 days)
	const synodicMonth = 29.53058867

	// Known new moon: January 6, 2000
	knownNewMoon := time.Date(2000, 1, 6, 18, 14, 0, 0, time.UTC)
	daysSince := t.Sub(knownNewMoon).Hours() / 24

	// Current position in the lunar cycle
	cyclePosition := daysSince / synodicMonth
	cyclePosition = cyclePosition - float64(int(cyclePosition)) // Get fractional part
	if cyclePosition < 0 {
		cyclePosition += 1
	}

	// Illumination (approximate)
	illumination := (1 - (1 + float64(int(cyclePosition*100)%100-50)/50)) / 2
	if cyclePosition < 0.5 {
		illumination = cyclePosition * 2
	} else {
		illumination = (1 - cyclePosition) * 2
	}

	// Determine phase name and emoji
	var phase, emoji string
	switch {
	case cyclePosition < 0.0625:
		phase, emoji = "New Moon", "🌑"
	case cyclePosition < 0.1875:
		phase, emoji = "Waxing Crescent", "🌒"
	case cyclePosition < 0.3125:
		phase, emoji = "First Quarter", "🌓"
	case cyclePosition < 0.4375:
		phase, emoji = "Waxing Gibbous", "🌔"
	case cyclePosition < 0.5625:
		phase, emoji = "Full Moon", "🌕"
	case cyclePosition < 0.6875:
		phase, emoji = "Waning Gibbous", "🌖"
	case cyclePosition < 0.8125:
		phase, emoji = "Last Quarter", "🌗"
	case cyclePosition < 0.9375:
		phase, emoji = "Waning Crescent", "🌘"
	default:
		phase, emoji = "New Moon", "🌑"
	}

	return phase, illumination * 100, emoji
}

func daysUntilFullMoon(t time.Time) int {
	const synodicMonth = 29.53058867
	knownNewMoon := time.Date(2000, 1, 6, 18, 14, 0, 0, time.UTC)
	daysSince := t.Sub(knownNewMoon).Hours() / 24
	cyclePosition := daysSince / synodicMonth
	cyclePosition = cyclePosition - float64(int(cyclePosition))
	if cyclePosition < 0 {
		cyclePosition += 1
	}

	// Full moon is at position 0.5
	daysToFull := (0.5 - cyclePosition) * synodicMonth
	if daysToFull < 0 {
		daysToFull += synodicMonth
	}
	return int(daysToFull)
}

// Tool handlers

func getMoonPhase(_ context.Context, _ *mcp.CallToolRequest, input GetMoonPhaseInput) (*mcp.CallToolResult, MoonPhaseOutput, error) {
	log.Printf("[DEBUG] get_moon_phase tool called with input: date=%s", input.Date)

	var t time.Time
	var err error

	if input.Date == "" {
		t = time.Now()
		log.Printf("[DEBUG] No date provided, using current time: %s", t.Format("2006-01-02"))
	} else {
		t, err = time.Parse("2006-01-02", input.Date)
		if err != nil {
			log.Printf("[ERROR] Invalid date format: %v", err)
			return nil, MoonPhaseOutput{}, fmt.Errorf("invalid date format, use YYYY-MM-DD: %w", err)
		}
		log.Printf("[DEBUG] Parsed date: %s", t.Format("2006-01-02"))
	}

	phase, illumination, emoji := calculateMoonPhase(t)
	daysToFull := daysUntilFullMoon(t)

	log.Printf("[DEBUG] Moon phase calculated: phase=%s, illumination=%.2f%%, days_until_full=%d",
		phase, illumination, daysToFull)

	return nil, MoonPhaseOutput{
		Date:          t.Format("2006-01-02"),
		Phase:         phase,
		Illumination:  illumination,
		DaysUntilFull: daysToFull,
		Emoji:         emoji,
	}, nil
}

func getMoonCalendar(_ context.Context, _ *mcp.CallToolRequest, input GetMoonCalendarInput) (*mcp.CallToolResult, MoonCalendarOutput, error) {
	log.Printf("[DEBUG] get_moon_calendar tool called with input: month=%d, year=%d", input.Month, input.Year)

	if input.Month < 1 || input.Month > 12 {
		log.Printf("[ERROR] Invalid month: %d (must be 1-12)", input.Month)
		return nil, MoonCalendarOutput{}, fmt.Errorf("month must be between 1 and 12")
	}
	if input.Year < 1900 || input.Year > 2100 {
		log.Printf("[ERROR] Invalid year: %d (must be 1900-2100)", input.Year)
		return nil, MoonCalendarOutput{}, fmt.Errorf("year must be between 1900 and 2100")
	}

	// Find key moon phases in the given month
	startDate := time.Date(input.Year, time.Month(input.Month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0)

	log.Printf("[DEBUG] Calculating moon calendar from %s to %s",
		startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	var newMoon, firstQtr, fullMoon, lastQtr string

	for d := startDate; d.Before(endDate); d = d.AddDate(0, 0, 1) {
		phase, _, _ := calculateMoonPhase(d)
		prevPhase, _, _ := calculateMoonPhase(d.AddDate(0, 0, -1))

		if phase != prevPhase {
			dateStr := d.Format("2006-01-02")
			switch phase {
			case "New Moon":
				if newMoon == "" {
					newMoon = dateStr
					log.Printf("[DEBUG] Found New Moon on %s", dateStr)
				}
			case "First Quarter":
				if firstQtr == "" {
					firstQtr = dateStr
					log.Printf("[DEBUG] Found First Quarter on %s", dateStr)
				}
			case "Full Moon":
				if fullMoon == "" {
					fullMoon = dateStr
					log.Printf("[DEBUG] Found Full Moon on %s", dateStr)
				}
			case "Last Quarter":
				if lastQtr == "" {
					lastQtr = dateStr
					log.Printf("[DEBUG] Found Last Quarter on %s", dateStr)
				}
			}
		}
	}

	result := MoonCalendarOutput{
		Month:    input.Month,
		Year:     input.Year,
		NewMoon:  newMoon,
		FirstQtr: firstQtr,
		FullMoon: fullMoon,
		LastQtr:  lastQtr,
	}

	log.Printf("[DEBUG] Moon calendar result: NewMoon=%s, FirstQtr=%s, FullMoon=%s, LastQtr=%s",
		newMoon, firstQtr, fullMoon, lastQtr)

	return nil, result, nil
}

// responseWriter wraps http.ResponseWriter to capture status code, headers, and body
type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	headers     http.Header
	body        []byte
	isStreaming bool
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	// Capture headers before writing
	rw.headers = make(http.Header)
	for k, v := range rw.ResponseWriter.Header() {
		rw.headers[k] = v
	}
	// Check if this is a streaming response
	contentType := rw.ResponseWriter.Header().Get("Content-Type")
	rw.isStreaming = (contentType == "text/event-stream")
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	// Don't capture body for streaming responses (SSE)
	if !rw.isStreaming {
		// Capture body for non-streaming responses
		if rw.body == nil {
			rw.body = make([]byte, 0)
		}
		// Only capture first 1KB to avoid memory issues
		if len(rw.body) < 1024 {
			remaining := 1024 - len(rw.body)
			if len(b) > remaining {
				rw.body = append(rw.body, b[:remaining]...)
			} else {
				rw.body = append(rw.body, b...)
			}
		}
	}
	return rw.ResponseWriter.Write(b)
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
	portFlag := flag.String("port", "", "HTTP port to listen on (overrides MOON_SERVER_PORT env var)")
	corsFlag := flag.Bool("cors", true, "Enable CORS middleware (needed for browser-based clients like mcp-inspector)")
	flag.Parse()

	// Get port from command-line flag, environment, or use default
	port := *portFlag
	if port == "" {
		port = os.Getenv("MOON_SERVER_PORT")
		if port == "" {
			port = "8081"
		}
	}

	// Create MCP server
	log.Printf("[DEBUG] Creating MCP server...")
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "moon-phase-server",
			Version: "1.0.0",
		},
		nil,
	)
	log.Printf("[DEBUG] MCP server created: name=%s, version=%s",
		"moon-phase-server", "1.0.0")

	// Add tools
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_moon_phase",
			Description: "Get the current moon phase for a specific date. Returns phase name, illumination percentage, days until full moon, and emoji.",
		},
		getMoonPhase,
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_moon_calendar",
			Description: "Get the moon phase calendar for a specific month, showing dates of new moon, first quarter, full moon, and last quarter.",
		},
		getMoonCalendar,
	)
	log.Printf("[DEBUG] Tools added: get_moon_phase, get_moon_calendar")

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
			"server":       "moon-phase-server",
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
	log.Printf("Moon Phase MCP Server starting...")
	log.Printf("========================================")
	log.Printf("Address: %s", addr)
	log.Printf("Health endpoint: http://localhost%s/health", addr)
	log.Printf("MCP endpoint: http://localhost%s/mcp", addr)
	log.Printf("Available tools: get_moon_phase, get_moon_calendar")
	log.Printf("========================================")
	log.Printf("[DEBUG] Starting HTTP server on %s...", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("[ERROR] Server failed to start: %v", err)
	}
}
