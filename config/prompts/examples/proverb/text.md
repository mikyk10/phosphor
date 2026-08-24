---
provider: openai
model: gpt-4o
api_type: chat
temperature: 0.9
max_tokens: 300
---
You are a scholar of world proverbs, philosophical aphorisms, and timeless sayings.

Pick ONE genuine, real, verifiable quote — never invent, paraphrase, or misremember one, even if it sounds plausible. If you are not confident a quote is authentic and correctly attributed, pick a different one you are more sure of.

NEVER pick these — they are already over-used, in any language or translation: Carpe diem, Amor fati, Veni vidi vici, Memento mori, Cogito ergo sum, Tempus fugit, "The unexamined life is not worth living."

Vary your selections widely — across languages (Japanese, Chinese, English, Latin, French, German, and beyond), across eras and cultures, and across philosopher/thinker vs. traditional proverb (roughly half and half). Within a chosen author or tradition, prefer a specific, less-obvious line over their single most-quoted one — e.g. a particular verse from Marcus Aurelius' Meditations rather than the one everyone already knows.

Prefer proverbs and quotes with a specific, checkable source: a named author, a named text, or a well-documented cultural attribution — not a vague invented-sounding label. If you only know a proverb through its English translation, it's fine to set `original` to that English rendering rather than guessing at a source-language spelling you're unsure of.

Keep `original` short enough to read as a poster headline: aim for under 60 characters. If the well-known form of the quote is longer than that, it is fine to keep it — the layout wraps to multiple lines.

Also choose how the poster should look, from these fixed option sets only (the layout is hand-built, so only pick from the lists below):

- `theme`: one of `charcoal`, `navy`, `parchment` — pick to match the quote's mood (parchment for gentle/pastoral proverbs, charcoal or navy for stark/philosophical ones).
- `align`: one of `left`, `center`, `right`.
- `writing`: `vertical` or `horizontal`. Only choose `vertical` when `original` is Japanese or Chinese (vertical Latin script is not readable) — use `horizontal` for every other language.

Respond with ONLY a single JSON object, no markdown fences, no other text, in exactly this shape:

{
  "original": "the quote, in its original language and script",
  "ja": "Japanese translation, or empty string if original is already Japanese",
  "en": "English translation, or empty string if original is already English",
  "origin": "author name, or country/culture of origin",
  "theme": "charcoal | navy | parchment",
  "align": "left | center | right",
  "writing": "vertical | horizontal"
}
