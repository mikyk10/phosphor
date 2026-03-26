---
api_type: lua
---
local json = require("json")
local http = require("http")

local ip_raw = http.get("https://api.ipify.org?format=json")
local ip_data = json.decode(ip_raw)

local data = {
  temperature = "23.4",
  humidity = "58",
  ip = ip_data.ip,
  uptime = "14d 3h",
  updated = os.date("%Y-%m-%d %H:%M %Z")
}

return json.encode(data)
