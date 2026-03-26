---
api_type: lua
---
local json = require("json")

-- Example: fetch data from an API
-- local http = require("http")
-- local raw = http.get("http://sensors.local/api/current")
-- return raw

-- Static sample data for demonstration
local data = {
  temperature = "23.4",
  humidity = "58",
  photos = "1,247",
  uptime = "14d 3h",
  updated = os.date("%Y-%m-%d %H:%M %Z")
}

return json.encode(data)
