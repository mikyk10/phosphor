---
api_type: render
size: 1536x1024
---
{{$d := json .prev.output}}<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  html { font-size: calc(100vw / 60); }
  body {
    width: 100vw; height: 100vh; overflow: hidden;
    font-family: 'Noto Serif', 'Noto Serif CJK JP', Georgia, 'Times New Roman', serif;
    display: flex; align-items: center; justify-content: center;
  }

  body[data-theme="charcoal"]  { background: #2C2C2C; color: #F4EEDC; --accent: #D4AF37; --muted: rgba(244,238,220,0.55); --rule: rgba(244,238,220,0.25); }
  body[data-theme="navy"]      { background: #1A1A2E; color: #F4EEDC; --accent: #D4AF37; --muted: rgba(244,238,220,0.55); --rule: rgba(244,238,220,0.25); }
  body[data-theme="parchment"] { background: #F5F0E8; color: #2C2C2C; --accent: #8B6F3E; --muted: rgba(44,44,44,0.55);   --rule: rgba(44,44,44,0.2); }

  .poster {
    width: 100%; height: 100%;
    display: flex; flex-direction: column; justify-content: center;
    align-items: flex-start; text-align: left;
    padding: 8vh 12vw;
  }

  body[data-writing="horizontal"][data-align="left"]   .poster { align-items: flex-start; text-align: left; }
  body[data-writing="horizontal"][data-align="center"] .poster { align-items: center;     text-align: center; }
  body[data-writing="horizontal"][data-align="right"]  .poster { align-items: flex-end;   text-align: right; }

  .hero {
    font-size: clamp(2.2rem, 6.5vw, 5.2rem);
    font-weight: 600; line-height: 1.25;
    max-width: 46rem;
    overflow-wrap: break-word;
  }

  .side { max-width: 40rem; }

  .pair {
    margin-top: 1.6rem; padding-top: 1.2rem;
    border-top: 1px solid var(--rule);
    display: flex; flex-direction: column; gap: 0.4rem;
  }
  .pair span { font-size: 1.3rem; font-style: italic; color: var(--muted); }

  .origin {
    margin-top: 2.2rem;
    font-size: 1rem; letter-spacing: 0.08em; text-transform: uppercase;
    color: var(--muted);
  }

  /* Vertical writing mode: only used for Japanese/Chinese originals. The
     hero column reads right-to-left, so it sits on the right with the
     translations/origin panel to its left (row-reverse puts the first HTML
     child on the right without needing separate LTR/RTL positioning rules). */
  body[data-writing="vertical"] .poster {
    flex-direction: row-reverse; align-items: center; justify-content: center;
    gap: 5vw; padding: 6vh 8vw;
  }
  body[data-writing="vertical"] .hero {
    writing-mode: vertical-rl; text-orientation: mixed;
    font-size: clamp(2.4rem, 8vh, 6rem);
    max-width: none; max-height: 72vh;
    line-height: 1.7; letter-spacing: 0.05em;
  }
  body[data-writing="vertical"] .side {
    display: flex; flex-direction: column; align-items: flex-start; text-align: left;
    max-width: 16rem;
  }
  body[data-writing="vertical"] .pair {
    border-top: none; border-left: 1px solid var(--rule);
    padding-top: 0; padding-left: 1.2rem; margin-top: 0;
  }
  body[data-writing="vertical"] .origin { margin-top: 1.6rem; }
</style>
</head>
<body data-theme="{{index $d "theme"}}" data-align="{{index $d "align"}}" data-writing="{{index $d "writing"}}">
  <div class="poster">
    <div class="hero">{{index $d "original"}}</div>
    <div class="side">
      {{$ja := index $d "ja"}}{{$en := index $d "en"}}
      {{if or $ja $en}}
      <div class="pair">
        {{if $ja}}<span>{{$ja}}</span>{{end}}
        {{if $en}}<span>{{$en}}</span>{{end}}
      </div>
      {{end}}
      <div class="origin">{{index $d "origin"}}</div>
    </div>
  </div>
</body>
</html>
