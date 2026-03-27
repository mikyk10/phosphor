---
api_type: render
size: 800x480
---
{{$d := json .prev.output}}<!DOCTYPE html>
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
    display: grid;
    grid-template-columns: 1fr 1fr;
    grid-template-rows: auto 1fr auto;
    gap: 0;
  }

  /* ── Header ── */
  .header {
    grid-column: 1 / -1;
    display: flex; justify-content: space-between; align-items: baseline;
    padding: 2rem 2.8rem 1rem;
  }
  .header .location { font-size: 2.4rem; font-weight: bold; letter-spacing: 0.05em; }
  .header .date { font-size: 1.4rem; color: #D7C76A; }

  /* ── Hero weather card ── */
  .hero {
    grid-column: 1 / -1;
    display: flex; align-items: center; justify-content: space-between;
    padding: 1rem 2.8rem 1.8rem;
  }
  .hero-left .temp {
    font-size: 7rem; font-weight: bold; line-height: 1;
    letter-spacing: -0.04em;
  }
  .hero-left .temp .unit { font-size: 3rem; font-weight: 400; opacity: 0.6; }
  .hero-left .condition {
    font-size: 1.8rem; margin-top: 0.4rem;
    color: #D7C76A;
  }

  /* ── Metric strip ── */
  .metrics {
    grid-column: 1 / -1;
    display: flex;
    border-top: 1px solid rgba(227,227,227,0.15);
  }
  .metric {
    flex: 1;
    padding: 1.4rem 2rem;
    display: flex; flex-direction: column; align-items: center;
    border-right: 1px solid rgba(227,227,227,0.1);
  }
  .metric:last-child { border-right: none; }
  .metric .m-label { font-size: 1rem; text-transform: uppercase; letter-spacing: 0.1em; opacity: 0.5; margin-bottom: 0.3rem; }
  .metric .m-value { font-size: 2.2rem; font-weight: bold; }
  .metric .m-unit { font-size: 1rem; opacity: 0.5; }

  /* ── Color accents ── */
  .accent-red    { color: #BD6571; }
  .accent-blue   { color: #696388; }
  .accent-green  { color: #609754; }
  .accent-yellow { color: #D7C76A; }

  /* ── Footer ── */
  .footer {
    grid-column: 1 / -1;
    padding: 0.6rem 2.8rem;
    font-size: 0.9rem; opacity: 0.3;
    display: flex; justify-content: space-between;
    border-top: 1px solid rgba(227,227,227,0.1);
  }
</style>
</head>
<body>
  <div class="header">
    <span class="location">{{index $d "location"}}</span>
    <span class="date">{{index $d "date"}}</span>
  </div>

  <div class="hero">
    <div class="hero-left">
      <div class="temp">{{index $d "temperature"}}<span class="unit">°C</span></div>
      <div class="condition">{{index $d "weather"}}</div>
    </div>
  </div>

  <div class="metrics">
    <div class="metric">
      <span class="m-label">Humidity</span>
      <span class="m-value accent-blue">{{index $d "humidity"}}</span>
      <span class="m-unit">%</span>
    </div>
    <div class="metric">
      <span class="m-label">Wind</span>
      <span class="m-value accent-green">{{index $d "wind"}}</span>
      <span class="m-unit">km/h</span>
    </div>
    <div class="metric">
      <span class="m-label">IP</span>
      <span class="m-value accent-yellow" style="font-size:1.6rem">{{index $d "ip"}}</span>
      <span class="m-unit">&nbsp;</span>
    </div>
  </div>

  <div class="footer">
    <span>Phosphor Dashboard</span>
    <span>Updated {{index $d "updated"}}</span>
  </div>
</body>
</html>
