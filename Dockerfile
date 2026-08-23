# --- Development image ---
FROM public.ecr.aws/docker/library/golang:1.26.0-bookworm AS development

WORKDIR /app

# Chromium for render stages (headless HTML → PNG)
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       chromium \
       fonts-noto-cjk \
    && rm -rf /var/lib/apt/lists/*
ENV CHROME_PATH=/usr/bin/chromium

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

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       ca-certificates \
       chromium \
       fonts-noto-cjk \
    && rm -rf /var/lib/apt/lists/*

# Chromium's own sandbox needs to create a user namespace, which Docker's
# default seccomp profile denies to an unprivileged process — and chromedp
# does not pass --no-sandbox itself. Launch Chromium through a wrapper that
# adds the flag, so the container stays the isolation boundary and only this
# image is affected (a local `go run` keeps Chromium's sandbox).
RUN printf '#!/bin/sh\nexec /usr/bin/chromium --no-sandbox "$@"\n' > /usr/local/bin/chromium-wrapper \
    && chmod 0755 /usr/local/bin/chromium-wrapper
ENV CHROME_PATH=/usr/local/bin/chromium-wrapper

# Run as an unprivileged user. HOME must be writable: Chromium writes its
# profile there.
RUN useradd --system --create-home --uid 10001 phosphor
ENV HOME=/home/phosphor

COPY --from=builder --chown=phosphor:phosphor /app/phosphor .
COPY --chown=phosphor:phosphor config config

USER phosphor

ENTRYPOINT ["./phosphor"]
CMD ["web", "run"]
