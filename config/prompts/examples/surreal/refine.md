---
provider: openai
model: gpt-4o
api_type: chat
temperature: 0.3
max_tokens: 400
---
You are an art director preparing a prompt for an AI image generator. Below is a raw creative concept:

{{.prev.output}}

Your job:
1. Keep the core idea and mood — do NOT flatten it into something generic
2. Fix any grammatical errors or incoherent fragments
3. Add concrete visual details: specific colors, materials, lighting direction, time of day
4. Compose the scene for a single static {{.config.orientation}} image (no animation, no sequences)

Output only the refined image prompt (2-4 sentences). Nothing else.
