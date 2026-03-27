---
api_type: render
size: 800x480
---
{{$ctx := json .stages.context.output}}{{$d := json .prev.output}}<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  html { font-size: calc(100vw / 80); }
  body {
    width: 100vw; height: 100vh; overflow: hidden;
    font-family: 'Noto Sans CJK JP', sans-serif;
    background: #424242; color: #E3E3E3;
    display: flex; flex-direction: column;
  }

  .header {
    padding: 1.8rem 2.8rem 0.8rem;
    display: flex; justify-content: space-between; align-items: baseline;
  }
  .header .title { font-size: 1.6rem; letter-spacing: 0.1em; opacity: 0.5; text-transform: uppercase; }
  .header .weather { font-size: 1.3rem; color: #696388; }

  .main-dish {
    padding: 0.8rem 2.8rem 0.6rem;
  }
  .main-dish .name {
    font-size: 4.8rem; font-weight: bold; line-height: 1.1;
    color: #D7C76A;
  }
  .main-dish .desc {
    font-size: 1.6rem; margin-top: 0.6rem; opacity: 0.7;
  }

  .sides {
    display: flex; gap: 1.2rem;
    padding: 1.2rem 2.8rem;
    flex: 1;
  }
  .side-card {
    flex: 1;
    background: rgba(227,227,227,0.08);
    border-radius: 1rem;
    padding: 1.4rem 1.6rem;
    display: flex; flex-direction: column; justify-content: center;
  }
  .side-card .side-label {
    font-size: 1rem; text-transform: uppercase; letter-spacing: 0.1em;
    opacity: 0.4; margin-bottom: 0.3rem;
  }
  .side-card .side-name {
    font-size: 2.2rem; font-weight: bold;
  }
  .side-card:nth-child(1) .side-name { color: #BD6571; }
  .side-card:nth-child(2) .side-name { color: #609754; }

  .reason {
    padding: 0.8rem 2.8rem 0.6rem;
    font-size: 1.2rem;
    color: #696388;
    font-style: italic;
  }

  .footer {
    padding: 0.5rem 2.8rem;
    font-size: 0.9rem; opacity: 0.25;
    display: flex; justify-content: space-between;
    border-top: 1px solid rgba(227,227,227,0.1);
  }
</style>
</head>
<body>
  <div class="header">
    <span class="title">Tonight's Dinner</span>
    <span class="weather">{{index $ctx "weather"}} {{index $ctx "temperature"}}°C</span>
  </div>

  <div class="main-dish">
    <div class="name">{{index $d "main"}}</div>
    <div class="desc">{{index $d "main_desc"}}</div>
  </div>

  <div class="sides">
    <div class="side-card">
      <span class="side-label">Side 1</span>
      <span class="side-name">{{index $d "side1"}}</span>
    </div>
    <div class="side-card">
      <span class="side-label">Side 2</span>
      <span class="side-name">{{index $d "side2"}}</span>
    </div>
  </div>

  <div class="reason">{{index $d "reason"}}</div>

  <div class="footer">
    <span>Phosphor Dinner</span>
    <span>{{index $ctx "date"}}</span>
  </div>
</body>
</html>
