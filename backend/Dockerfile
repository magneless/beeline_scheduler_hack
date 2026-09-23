FROM golang:1.27.0-bookworm AS build

ENV CGO_ENABLED=1 \
    GOTOOLCHAIN=local \
    GOFLAGS=-tags=ortools \
    CGO_LDFLAGS="-L/opt/ortools -Wl,-rpath,/opt/ortools" \
    LD_LIBRARY_PATH=/opt/ortools

WORKDIR /workspace/backend

COPY backend/scripts/setup-ortools.sh ./scripts/setup-ortools.sh
RUN bash ./scripts/setup-ortools.sh \
    && mkdir -p /opt/ortools \
    && cp -a .cache/ortools/lib/. /opt/ortools/ \
    && rm -rf .cache

COPY backend/go.mod backend/go.sum ./
RUN go mod download && go mod verify

COPY backend/ ./
COPY docs/contracts/examples/ /workspace/docs/contracts/examples/
COPY datasets/original/ /workspace/datasets/original/
RUN go build -o /scheduler ./cmd/server

FROM build AS test
CMD ["go", "test", "-race", "-count=1", "./..."]

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates libstdc++6 libssl3 \
    && rm -rf /var/lib/apt/lists/*
ENV LD_LIBRARY_PATH=/opt/ortools
WORKDIR /app
COPY --from=build /opt/ortools/ /opt/ortools/
COPY --from=build /scheduler /app/scheduler
COPY datasets/original/ /app/datasets/original/
COPY docs/contracts/examples/ /app/docs/contracts/examples/
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/scheduler"]
