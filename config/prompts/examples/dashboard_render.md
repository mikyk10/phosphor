---
api_type: render
size: 800x480
---
{{$d := json .prev.output}}<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=800">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  html { font-size: calc(100vw / 80); }
  body {
    width: 100vw; height: 100vh; overflow: hidden;
    font-family: 'Noto Sans CJK JP', sans-serif;
    background: #E3E3E3; color: #424242;
    display: flex; flex-direction: column; padding: 2.4rem;
  }
  h1 { font-size: 2.8rem; margin-bottom: 1.6rem; }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1.6rem; flex: 1; }
  .card { border-radius: 1.2rem; padding: 2rem; display: flex; flex-direction: column; justify-content: center; }
  .card .label { font-size: 1.4rem; margin-bottom: 0.4rem; opacity: 0.7; }
  .card .value { font-size: 3.6rem; font-weight: bold; }
  .card-red    { background: #BD6571; color: #E3E3E3; }
  .card-blue   { background: #696388; color: #E3E3E3; }
  .card-green  { background: #609754; color: #E3E3E3; }
  .card-yellow { background: #D7C76A; color: #424242; }
  .footer { margin-top: 1.6rem; font-size: 1.2rem; color: #696388; text-align: right; }
</style>
</head>
<body>
  <h1>Dashboard</h1>
  <div class="grid">
    <div class="card card-red">
      <span class="label">Temperature</span>
      <span class="value">{{index $d "temperature"}}°C</span>
    </div>
    <div class="card card-blue">
      <span class="label">Humidity</span>
      <span class="value">{{index $d "humidity"}}%</span>
    </div>
    <div class="card card-green">
      <span class="label">Photos</span>
      <span class="value">{{index $d "photos"}}</span>
    </div>
    <div class="card card-yellow">
      <span class="label">Uptime</span>
      <span class="value">{{index $d "uptime"}}</span>
    </div>
  </div>
  <div class="footer">Last updated: {{index $d "updated"}}</div>
</body>
</html>
