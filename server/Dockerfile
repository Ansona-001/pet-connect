FROM golang:1.24-alpine AS build

WORKDIR /src
COPY server/go.mod server/go.sum* ./
RUN go mod download

COPY server/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/petconnect-api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget && addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=build /out/petconnect-api /app/petconnect-api
COPY assets/images /app/demo-assets
USER app
EXPOSE 8080
ENTRYPOINT ["/app/petconnect-api"]
