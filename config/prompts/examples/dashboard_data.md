---
api_type: lua
---
local json = require("json")
local http = require("http")

-- Weather via Open-Meteo (no API key required)
-- Change latitude/longitude/timezone to your location
local weather_raw = http.get("https://api.open-meteo.com/v1/forecast?latitude=35.4437&longitude=139.6380&current=temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m&timezone=Asia%2FTokyo")
local weather = json.decode(weather_raw)

-- Public IP
local ip_raw = http.get("https://api.ipify.org?format=json")
local ip_data = json.decode(ip_raw)

-- WMO weather code to description
local wmo = {
  [0] = "Clear Sky", [1] = "Mostly Clear", [2] = "Partly Cloudy", [3] = "Overcast",
  [45] = "Foggy", [48] = "Icy Fog",
  [51] = "Light Drizzle", [53] = "Drizzle", [55] = "Heavy Drizzle",
  [61] = "Light Rain", [63] = "Rain", [65] = "Heavy Rain",
  [71] = "Light Snow", [73] = "Snow", [75] = "Heavy Snow",
  [80] = "Light Showers", [81] = "Showers", [82] = "Heavy Showers",
  [95] = "Thunderstorm", [96] = "Hail Storm", [99] = "Heavy Hail"
}
local code = weather.current.weather_code
local weather_desc = wmo[code] or ("WMO " .. tostring(code))

local data = {
  temperature = string.format("%.1f", weather.current.temperature_2m),
  humidity = tostring(weather.current.relative_humidity_2m),
  wind = string.format("%.0f", weather.current.wind_speed_10m),
  weather = weather_desc,
  ip = ip_data.ip,
  location = "Yokohama",
  updated = os.date("%H:%M"),
  date = os.date("%a, %b %d")
}

return json.encode(data)
