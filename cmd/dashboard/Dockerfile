# Dockerfile for the Kaimi web dashboard (Cloud Run Service).
#
# Build from the repository root (the build context must include go.mod and
# the config/ directory):
#
#   docker build -f cmd/dashboard/Dockerfile -t kaimi-dashboard .
#
# Unlike cmd/pipeline (a batch Job), this is a long-running HTTP server. Deploy
# it as a Cloud Run Service. It serves the opportunity store and the Zone 2
# proposal surfaces. Configuration is via flags/env:
#   --host 0.0.0.0   (required on Cloud Run; also via HOST env)
#   $PORT            (Cloud Run injects the listen port; honored automatically)
#   --store          (point at the JSON store; on Cloud Run mount the GCS queue
#                     bucket as a volume and pass --store=/mnt/store)
#   --live-writer / --live-review  (real Gemini agents; need GCP_PROJECT_ID +
#                     Vertex AI access on the service account)

# --- Build stage -------------------------------------------------------------
FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO disabled for a fully static binary that runs on distroless.
RUN CGO_ENABLED=0 go build -o /out/dashboard ./cmd/dashboard

# --- Runtime stage -----------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=build /out/dashboard /app/dashboard

# The live writer grounds on the capability profile read from disk; bake the
# repo config into the image (override with --profile if needed).
COPY --from=build /src/config /app/config

ENTRYPOINT ["/app/dashboard"]
