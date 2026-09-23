FROM golang:1.27.0-bookworm

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
# Baseline tests resolve these fixtures relative to backend/internal/planner/baseline.
COPY docs/contracts/examples/ /workspace/docs/contracts/examples/

RUN go build ./...

CMD ["go", "test", "-race", "-count=1", "./..."]
