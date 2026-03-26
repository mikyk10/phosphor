# --- Fetch chrome-headless-shell ---
FROM public.ecr.aws/docker/library/debian:bookworm-slim AS chrome
RUN apt-get update && apt-get install -y --no-install-recommends curl unzip && rm -rf /var/lib/apt/lists/*
ARG CHROME_VERSION=137.0.7151.68
RUN curl -fsSL "https://storage.googleapis.com/chrome-for-testing-public/${CHROME_VERSION}/linux64/chrome-headless-shell-linux64.zip" \
      -o /tmp/chrome.zip \
    && unzip /tmp/chrome.zip -d /opt \
    && rm /tmp/chrome.zip


# --- Development image ---
FROM public.ecr.aws/docker/library/golang:1.26.0-bookworm AS development

WORKDIR /app

# chrome-headless-shell for render stages
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       fonts-noto-cjk \
       libnspr4 libnss3 libatk1.0-0 libatk-bridge2.0-0 \
       libcups2 libdrm2 libxkbcommon0 libxcomposite1 \
       libxdamage1 libxrandr2 libgbm1 libpango-1.0-0 \
       libcairo2 libasound2 libxshmfence1 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=chrome /opt/chrome-headless-shell-linux64 /opt/chrome
ENV CHROME_PATH=/opt/chrome/chrome-headless-shell

COPY go.mod go.sum ./
RUN go mod download

ARG LOCAL_DEV
RUN if [ "${LOCAL_DEV}" = "true" ]; then \
      go install github.com/air-verse/air@latest; fi

CMD ["air"]


# --- Build binary ---
FROM development AS builder
COPY . .
ARG GITHUB_REF_NAME
ARG GITHUB_SHA
ENV GITHUB_REF_NAME=${GITHUB_REF_NAME}
ENV GITHUB_SHA=${GITHUB_SHA}
RUN make


# --- Production image ---
FROM public.ecr.aws/docker/library/debian:bookworm-slim AS runner

WORKDIR /app
ENV TZ=Asia/Tokyo

# chrome-headless-shell runtime dependencies
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       ca-certificates \
       fonts-noto-cjk \
       libnspr4 libnss3 libatk1.0-0 libatk-bridge2.0-0 \
       libcups2 libdrm2 libxkbcommon0 libxcomposite1 \
       libxdamage1 libxrandr2 libgbm1 libpango-1.0-0 \
       libcairo2 libasound2 libxshmfence1 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=chrome /opt/chrome-headless-shell-linux64 /opt/chrome
ENV CHROME_PATH=/opt/chrome/chrome-headless-shell

COPY --from=builder /app/wisp-ai .
COPY prompts prompts

ENTRYPOINT ["./wisp-ai"]
CMD ["web", "run"]
