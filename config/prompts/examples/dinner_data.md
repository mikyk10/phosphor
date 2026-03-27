---
api_type: lua
---
local json = require("json")
local http = require("http")

-- Weather via Open-Meteo (no API key required)
-- Change latitude/longitude/timezone to your location
local weather_raw = http.get("https://api.open-meteo.com/v1/forecast?latitude=35.4437&longitude=139.6380&current=temperature_2m,weather_code&timezone=Asia%2FTokyo")
local weather = json.decode(weather_raw)

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
local temp = string.format("%.0f", weather.current.temperature_2m)

local data = {
  weather = weather_desc,
  temperature = temp,
  date = os.date("%Y-%m-%d"),
  season = "",
}

-- Simple season detection
local month = tonumber(os.date("%m"))
if month >= 3 and month <= 5 then
  data.season = "spring"
elseif month >= 6 and month <= 8 then
  data.season = "summer"
elseif month >= 9 and month <= 11 then
  data.season = "autumn"
else
  data.season = "winter"
end

return json.encode(data)
