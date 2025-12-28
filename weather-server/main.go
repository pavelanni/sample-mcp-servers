// Weather MCP Server
// A simple MCP server that provides weather information using the Open-Meteo free API.
// Supports StreamableHTTP transport for gateway testing.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Tool input/output types

type GetCurrentWeatherInput struct {
	Latitude  float64 `json:"latitude" jsonschema:"latitude coordinate (-90 to 90)"`
	Longitude float64 `json:"longitude" jsonschema:"longitude coordinate (-180 to 180)"`
}

type CurrentWeatherOutput struct {
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Temperature   float64 `json:"temperature_celsius"`
	WindSpeed     float64 `json:"wind_speed_kmh"`
	WindDirection int     `json:"wind_direction_degrees"`
	WeatherCode   int     `json:"weather_code"`
	Description   string  `json:"description"`
	IsDay         bool    `json:"is_day"`
	Time          string  `json:"time"`
}

type GetForecastInput struct {
	Latitude  float64 `json:"latitude" jsonschema:"latitude coordinate (-90 to 90)"`
	Longitude float64 `json:"longitude" jsonschema:"longitude coordinate (-180 to 180)"`
	Days      int     `json:"days,omitempty" jsonschema:"number of forecast days (1-7, default 3)"`
}

type DailyForecast struct {
	Date             string  `json:"date"`
	TempMax          float64 `json:"temp_max_celsius"`
	TempMin          float64 `json:"temp_min_celsius"`
	WeatherCode      int     `json:"weather_code"`
	Description      string  `json:"description"`
	PrecipitationSum float64 `json:"precipitation_mm"`
}

type ForecastOutput struct {
	Latitude  float64         `json:"latitude"`
	Longitude float64         `json:"longitude"`
	Daily     []DailyForecast `json:"daily"`
}

// Weather code to description mapping
var weatherCodeDescriptions = map[int]string{
	0:  "Clear sky",
	1:  "Mainly clear",
	2:  "Partly cloudy",
	3:  "Overcast",
	45: "Fog",
	48: "Depositing rime fog",
	51: "Light drizzle",
	53: "Moderate drizzle",
	55: "Dense drizzle",
	61: "Slight rain",
	63: "Moderate rain",
	65: "Heavy rain",
	71: "Slight snow",
	73: "Moderate snow",
	75: "Heavy snow",
	77: "Snow grains",
	80: "Slight rain showers",
	81: "Moderate rain showers",
	82: "Violent rain showers",
	85: "Slight snow showers",
	86: "Heavy snow showers",
	95: "Thunderstorm",
	96: "Thunderstorm with slight hail",
	99: "Thunderstorm with heavy hail",
}

func getWeatherDescription(code int) string {
	if desc, ok := weatherCodeDescriptions[code]; ok {
		return desc
	}
	return "Unknown"
}

// Open-Meteo API response structures
type OpenMeteoCurrentResponse struct {
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	CurrentWeather struct {
		Temperature   float64 `json:"temperature"`
		WindSpeed     float64 `json:"windspeed"`
		WindDirection int     `json:"winddirection"`
		WeatherCode   int     `json:"weathercode"`
		IsDay         int     `json:"is_day"`
		Time          string  `json:"time"`
	} `json:"current_weather"`
}

type OpenMeteoForecastResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Daily     struct {
		Time             []string  `json:"time"`
		Temperature2mMax []float64 `json:"temperature_2m_max"`
		Temperature2mMin []float64 `json:"temperature_2m_min"`
		WeatherCode      []int     `json:"weathercode"`
		PrecipitationSum []float64 `json:"precipitation_sum"`
	} `json:"daily"`
}

// Tool handlers

func getCurrentWeather(_ context.Context, _ *mcp.CallToolRequest, input GetCurrentWeatherInput) (*mcp.CallToolResult, CurrentWeatherOutput, error) {
	log.Printf("[DEBUG] get_current_weather tool called with input: latitude=%.4f, longitude=%.4f", input.Latitude, input.Longitude)

	// Validate coordinates
	if input.Latitude < -90 || input.Latitude > 90 {
		log.Printf("[ERROR] Invalid latitude: %.4f (must be between -90 and 90)", input.Latitude)
		return nil, CurrentWeatherOutput{}, fmt.Errorf("latitude must be between -90 and 90")
	}
	if input.Longitude < -180 || input.Longitude > 180 {
		log.Printf("[ERROR] Invalid longitude: %.4f (must be between -180 and 180)", input.Longitude)
		return nil, CurrentWeatherOutput{}, fmt.Errorf("longitude must be between -180 and 180")
	}

	// Build API URL
	apiURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current_weather=true",
		input.Latitude, input.Longitude,
	)
	log.Printf("[DEBUG] Fetching weather from API: %s", apiURL)

	// Fetch from API
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Printf("[ERROR] Failed to fetch weather data: %v", err)
		return nil, CurrentWeatherOutput{}, fmt.Errorf("failed to fetch weather data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[ERROR] API returned non-OK status: %d", resp.StatusCode)
		return nil, CurrentWeatherOutput{}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp OpenMeteoCurrentResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		log.Printf("[ERROR] Failed to parse API response: %v", err)
		return nil, CurrentWeatherOutput{}, fmt.Errorf("failed to parse API response: %w", err)
	}

	result := CurrentWeatherOutput{
		Latitude:      apiResp.Latitude,
		Longitude:     apiResp.Longitude,
		Temperature:   apiResp.CurrentWeather.Temperature,
		WindSpeed:     apiResp.CurrentWeather.WindSpeed,
		WindDirection: apiResp.CurrentWeather.WindDirection,
		WeatherCode:   apiResp.CurrentWeather.WeatherCode,
		Description:   getWeatherDescription(apiResp.CurrentWeather.WeatherCode),
		IsDay:         apiResp.CurrentWeather.IsDay == 1,
		Time:          apiResp.CurrentWeather.Time,
	}
	log.Printf("[DEBUG] Weather data retrieved: temp=%.1f°C, description=%s, wind=%.1f km/h",
		result.Temperature, result.Description, result.WindSpeed)

	return nil, result, nil
}

func getForecast(_ context.Context, _ *mcp.CallToolRequest, input GetForecastInput) (*mcp.CallToolResult, ForecastOutput, error) {
	log.Printf("[DEBUG] get_forecast tool called with input: latitude=%.4f, longitude=%.4f, days=%d",
		input.Latitude, input.Longitude, input.Days)

	// Validate coordinates
	if input.Latitude < -90 || input.Latitude > 90 {
		log.Printf("[ERROR] Invalid latitude: %.4f (must be between -90 and 90)", input.Latitude)
		return nil, ForecastOutput{}, fmt.Errorf("latitude must be between -90 and 90")
	}
	if input.Longitude < -180 || input.Longitude > 180 {
		log.Printf("[ERROR] Invalid longitude: %.4f (must be between -180 and 180)", input.Longitude)
		return nil, ForecastOutput{}, fmt.Errorf("longitude must be between -180 and 180")
	}

	// Validate and set default days
	days := input.Days
	if days <= 0 {
		days = 3
		log.Printf("[DEBUG] Days was <= 0, using default: %d", days)
	}
	if days > 7 {
		days = 7
		log.Printf("[DEBUG] Days exceeded max, capping at: %d", days)
	}

	// Build API URL
	params := url.Values{}
	params.Set("latitude", fmt.Sprintf("%f", input.Latitude))
	params.Set("longitude", fmt.Sprintf("%f", input.Longitude))
	params.Set("daily", "temperature_2m_max,temperature_2m_min,weathercode,precipitation_sum")
	params.Set("forecast_days", fmt.Sprintf("%d", days))
	params.Set("timezone", "auto")

	apiURL := "https://api.open-meteo.com/v1/forecast?" + params.Encode()
	log.Printf("[DEBUG] Fetching forecast from API: %s", apiURL)

	// Fetch from API
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		log.Printf("[ERROR] Failed to fetch forecast data: %v", err)
		return nil, ForecastOutput{}, fmt.Errorf("failed to fetch forecast data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[ERROR] API returned non-OK status: %d", resp.StatusCode)
		return nil, ForecastOutput{}, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp OpenMeteoForecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		log.Printf("[ERROR] Failed to parse API response: %v", err)
		return nil, ForecastOutput{}, fmt.Errorf("failed to parse API response: %w", err)
	}

	// Build daily forecasts
	var daily []DailyForecast
	for i := range apiResp.Daily.Time {
		if i >= len(apiResp.Daily.Temperature2mMax) {
			break
		}
		daily = append(daily, DailyForecast{
			Date:             apiResp.Daily.Time[i],
			TempMax:          apiResp.Daily.Temperature2mMax[i],
			TempMin:          apiResp.Daily.Temperature2mMin[i],
			WeatherCode:      apiResp.Daily.WeatherCode[i],
			Description:      getWeatherDescription(apiResp.Daily.WeatherCode[i]),
			PrecipitationSum: apiResp.Daily.PrecipitationSum[i],
		})
	}

	log.Printf("[DEBUG] Forecast retrieved: %d days of data", len(daily))
	result := ForecastOutput{
		Latitude:  apiResp.Latitude,
		Longitude: apiResp.Longitude,
		Daily:     daily,
	}
	return nil, result, nil
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
	portFlag := flag.String("port", "", "HTTP port to listen on (overrides WEATHER_SERVER_PORT env var)")
	corsFlag := flag.Bool("cors", true, "Enable CORS middleware (needed for browser-based clients like mcp-inspector)")
	flag.Parse()

	// Get port from command-line flag, environment, or use default
	port := *portFlag
	if port == "" {
		port = os.Getenv("WEATHER_SERVER_PORT")
		if port == "" {
			port = "8083"
		}
	}

	// Create MCP server
	log.Printf("[DEBUG] Creating MCP server...")
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "weather-server",
			Version: "1.0.0",
		},
		nil,
	)
	log.Printf("[DEBUG] MCP server created: name=%s, version=%s", "weather-server", "1.0.0")

	// Add tools
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_current_weather",
			Description: "Get current weather conditions for a location specified by latitude and longitude coordinates.",
		},
		getCurrentWeather,
	)

	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_forecast",
			Description: "Get weather forecast for a location. Returns daily forecasts including temperature range, weather conditions, and precipitation.",
		},
		getForecast,
	)
	log.Printf("[DEBUG] Tools added: get_current_weather, get_forecast")

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
			"server":       "weather-server",
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
	log.Printf("Weather MCP Server starting...")
	log.Printf("========================================")
	log.Printf("Address: %s", addr)
	log.Printf("Health endpoint: http://localhost%s/health", addr)
	log.Printf("MCP endpoint: http://localhost%s/mcp", addr)
	log.Printf("Available tools: get_current_weather, get_forecast")
	log.Printf("========================================")
	log.Printf("[DEBUG] Starting HTTP server on %s...", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("[ERROR] Server failed to start: %v", err)
	}
}
