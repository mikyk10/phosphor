---
api_type: lua
---
local json = require("json")
local http = require("http")

-- Yokohama weather (Open-Meteo, no API key required)
local weather_raw = http.get("https://api.open-meteo.com/v1/forecast?latitude=35.4437&longitude=139.6380&current=temperature_2m,relative_humidity_2m,weather_code&timezone=Asia%2FTokyo")
local weather = json.decode(weather_raw)

-- Public IP
local ip_raw = http.get("https://api.ipify.org?format=json")
local ip_data = json.decode(ip_raw)

local data = {
  temperature = tostring(weather.current.temperature_2m),
  humidity = tostring(weather.current.relative_humidity_2m),
  ip = ip_data.ip,
  weather_code = tostring(weather.current.weather_code),
  updated = os.date("%Y-%m-%d %H:%M %Z")
}

return json.encode(data)
