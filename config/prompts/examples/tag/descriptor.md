---
provider: ollama
model: qwen3.5:9b
#provider: openai
#model: gpt-4o
temperature: 0.3
max_tokens: 500
---
Describe this photo the way a person would remember it. Focus on:
- The main subject and what they are doing
- The setting and occasion
- The overall mood and feeling

Ignore minor details that a viewer would not notice or care about, such as exact clothing patterns, background clutter, or interface elements. If this is clearly a UI screenshot (app interface, web page, terminal), just say "screenshot" and stop. Artistic filters on photos do not make them screenshots.

Write in plain English. Write in plain prose, 2-4 sentences. No headers, no bullet points.
