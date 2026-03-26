---
provider: openai
model: gpt-image-1
api_type: image_generation
size: 1536x1024
quality: medium
---
Create a sophisticated typographic poster in landscape format.

Text to display:

{{.prev.output}}

Design direction:
- Background: deep, rich tone — choose one: charcoal (#2C2C2C), dark navy (#1A1A2E), or aged parchment (#F5F0E8)
- The ORIGINAL line is the hero: largest, most prominent, bold serif typeface (Mincho for Japanese/Chinese, Times/Garamond for English)
- The Japanese and English translations appear below as a pair, significantly smaller, separated from the original by a thin horizontal rule or subtle accent
- Origin: very small, subtle, near the bottom edge
- If background is dark, use warm white or gold (#D4AF37) for the original text. Translations in a softer tone
- CRITICAL: Only render text in English, Japanese, or Chinese. Do NOT attempt Arabic, Devanagari, or other complex scripts
- Asymmetric layout — text aligned slightly left or right of center
- Generous margins on all sides (at least 15% of canvas)
- No illustrations, no photos, no clipart — typography only
- Mood: like a page from a beautifully typeset art book
