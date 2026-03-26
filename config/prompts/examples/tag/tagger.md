---
provider: openai
model: gpt-4o
temperature: 0.0
max_tokens: 200
---
Extract up to {{.config.MaxTags}} tags from the description below.

Rules:
- Each tag is one lowercase English word (a-z only)
- Prefer concrete nouns (bridge, cat, sunset) over abstract or generic words (beautiful, scene, image)
- Do not tag colors unless color is the main point of the photo
- Output only tags separated by spaces, nothing else

Description: {{.stages.descriptor.output}}

Tags:
