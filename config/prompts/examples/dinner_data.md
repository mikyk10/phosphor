---
api_type: lua
---
local json = require("json")
local http = require("http")

-- Yokohama weather
local weather_raw = http.get("https://api.open-meteo.com/v1/forecast?latitude=35.4437&longitude=139.6380&current=temperature_2m,weather_code&timezone=Asia%2FTokyo")
local weather = json.decode(weather_raw)

local wmo = {
  [0] = "快晴", [1] = "晴れ", [2] = "くもり", [3] = "曇天",
  [45] = "霧", [48] = "霧氷",
  [51] = "小雨", [53] = "雨", [55] = "大雨",
  [61] = "小雨", [63] = "雨", [65] = "大雨",
  [71] = "小雪", [73] = "雪", [75] = "大雪",
  [80] = "にわか雨", [81] = "にわか雨", [82] = "豪雨",
  [95] = "雷雨", [96] = "雹", [99] = "激しい雹"
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
  data.season = "春"
elseif month >= 6 and month <= 8 then
  data.season = "夏"
elseif month >= 9 and month <= 11 then
  data.season = "秋"
else
  data.season = "冬"
end

return json.encode(data)
