FROM ghcr.io/vroom-project/vroom-docker:v1.15.0 AS vroom

FROM debian:trixie-slim AS vroom-runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates libstdc++6 libssl3t64 libglpk40 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=vroom /usr/local/bin/vroom /usr/local/bin/vroom
RUN vroom --version

FROM golang:1.27.0-bookworm AS build
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
WORKDIR /workspace/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download && go mod verify
COPY backend/ ./
COPY docs/contracts/examples/ /workspace/docs/contracts/examples/
COPY datasets/original/ /workspace/datasets/original/
COPY datasets/additional_days/ /workspace/datasets/additional_days/
RUN go build -o /scheduler ./cmd/server

FROM vroom-runtime AS test
RUN apt-get update \
    && apt-get install -y --no-install-recommends gcc libc6-dev \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /usr/local/go/ /usr/local/go/
COPY --from=build /go/ /go/
COPY --from=build /workspace/ /workspace/
ENV PATH="/usr/local/go/bin:${PATH}" \
    GOPATH=/go GOTOOLCHAIN=local CGO_ENABLED=1 GOFLAGS=-tags=vroom
WORKDIR /workspace/backend
CMD ["go", "test", "-race", "-count=1", "./..."]

FROM vroom-runtime AS runtime
ENV GEO_CACHE_DIR=/var/cache/scheduler
WORKDIR /app
COPY --from=build /scheduler /app/scheduler
COPY datasets/original/ /app/datasets/original/
COPY datasets/additional_days/ /app/datasets/additional_days/
COPY docs/contracts/examples/ /app/docs/contracts/examples/
RUN mkdir -p /var/cache/scheduler && chown 65532:65532 /var/cache/scheduler
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/scheduler"]
