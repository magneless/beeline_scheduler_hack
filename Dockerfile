FROM golang:1.26 AS build
WORKDIR /src
ARG GOPROXY=https://proxy.golang.org,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /scheduler ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /scheduler /app/scheduler
COPY datasets/original /app/datasets/original
COPY docs/contracts/examples /app/docs/contracts/examples
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/scheduler"]
