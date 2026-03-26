# --- Development image ---
FROM public.ecr.aws/docker/library/golang:1.26.0-bookworm AS development

WORKDIR /app

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

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/wisp-ai .
COPY config config

ENTRYPOINT ["./wisp-ai"]
CMD ["web", "run"]
