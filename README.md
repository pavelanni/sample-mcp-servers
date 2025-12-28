# MCP gateway test servers

Three simple MCP servers for testing MCP gateways with StreamableHTTP transport.

## Servers overview

| Server | Port | Tools | Description |
|--------|------|-------|-------------|
| moon-server | 8081 | 2 | Moon phase calculations |
| quotes-server | 8082 | 3 | Random quotes and search |
| weather-server | 8083 | 2 | Weather data via Open-Meteo API |

## Requirements

- Go 1.21 or later
- Internet access (for weather-server API calls)

## Quick start

### Build all servers

```bash
cd moon-server && go mod tidy && go build -o ../bin/moon-server . && cd ..
cd quotes-server && go mod tidy && go build -o ../bin/quotes-server . && cd ..
cd weather-server && go mod tidy && go build -o ../bin/weather-server . && cd ..
```

### Run servers

Run each in a separate terminal:

```bash
./bin/moon-server    # Runs on :8081
./bin/quotes-server  # Runs on :8082
./bin/weather-server # Runs on :8083
```

Or use custom ports:

```bash
PORT=9001 ./bin/moon-server
```

## MCP endpoints

All servers expose StreamableHTTP endpoints at `/mcp`:

- Moon: `http://localhost:8081/mcp`
- Quotes: `http://localhost:8082/mcp`
- Weather: `http://localhost:8083/mcp`

Health check endpoints at `/health`.

## Tools reference

### moon-server

#### get_moon_phase

Get the moon phase for a specific date.

**Input:**

```json
{
  "date": "2025-01-15"  // optional, defaults to today
}
```

**Output:**

```json
{
  "date": "2025-01-15",
  "phase": "Waning Gibbous",
  "illumination": 85.3,
  "days_until_full": 12,
  "emoji": "🌖"
}
```

#### get_moon_calendar

Get moon phase dates for a month.

**Input:**

```json
{
  "month": 1,
  "year": 2025
}
```

**Output:**

```json
{
  "month": 1,
  "year": 2025,
  "new_moon": "2025-01-29",
  "first_quarter": "2025-01-06",
  "full_moon": "2025-01-13",
  "last_quarter": "2025-01-21"
}
```

### quotes-server

#### get_random_quote

Get a random quote, optionally filtered by category.

**Input:**

```json
{
  "category": "programming"  // optional
}
```

**Output:**

```json
{
  "text": "Talk is cheap. Show me the code.",
  "author": "Linus Torvalds",
  "category": "programming"
}
```

#### search_quotes

Search quotes by keyword.

**Input:**

```json
{
  "query": "code",
  "limit": 5  // optional, default 5, max 10
}
```

**Output:**

```json
{
  "quotes": [...],
  "total": 3
}
```

#### list_categories

List available quote categories.

**Output:**

```json
{
  "categories": ["motivation", "wisdom", "programming", "innovation", "life", "courage"]
}
```

### weather-server

#### get_current_weather

Get current weather for a location.

**Input:**

```json
{
  "latitude": 40.7128,
  "longitude": -74.0060
}
```

**Output:**

```json
{
  "latitude": 40.71,
  "longitude": -74.01,
  "temperature_celsius": 12.5,
  "wind_speed_kmh": 15.2,
  "wind_direction_degrees": 180,
  "weather_code": 2,
  "description": "Partly cloudy",
  "is_day": true,
  "time": "2025-01-15T14:00"
}
```

#### get_forecast

Get weather forecast for a location.

**Input:**

```json
{
  "latitude": 40.7128,
  "longitude": -74.0060,
  "days": 5  // optional, default 3, max 7
}
```

**Output:**

```json
{
  "latitude": 40.71,
  "longitude": -74.01,
  "daily": [
    {
      "date": "2025-01-15",
      "temp_max_celsius": 15.2,
      "temp_min_celsius": 8.1,
      "weather_code": 61,
      "description": "Slight rain",
      "precipitation_mm": 2.5
    },
    ...
  ]
}
```

## Testing with MCP inspector

You can test these servers using the MCP Inspector tool:

```bash
npx @anthropic-ai/mcp-inspector
```

Then connect to any server using StreamableHTTP transport.

## Gateway configuration example

Example configuration for an MCP gateway:

```json
{
  "servers": [
    {
      "name": "moon",
      "url": "http://localhost:8081/mcp",
      "transport": "streamable-http"
    },
    {
      "name": "quotes",
      "url": "http://localhost:8082/mcp",
      "transport": "streamable-http"
    },
    {
      "name": "weather",
      "url": "http://localhost:8083/mcp",
      "transport": "streamable-http"
    }
  ]
}
```

## Kubernetes deployment

For running these servers in a local Kind cluster with Podman, see [KIND_SETUP.md](KIND_SETUP.md).

## Notes

- The moon-server uses a simplified algorithm for moon phase calculations; results are approximate.
- The quotes-server includes a fallback local database when external API is unavailable.
- The weather-server uses the free Open-Meteo API, which has rate limits but no API key required.
