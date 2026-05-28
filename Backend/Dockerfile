FROM golang:1.26.3-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/beeba-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/beeba-worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/beeba-migrate ./cmd/migrate

FROM alpine:3.22

RUN addgroup -S beeba && adduser -S beeba -G beeba
WORKDIR /app

COPY --from=build /out/beeba-api /app/beeba-api
COPY --from=build /out/beeba-worker /app/beeba-worker
COPY --from=build /out/beeba-migrate /app/beeba-migrate
COPY migrations /app/migrations
COPY openapi /app/openapi

USER beeba
EXPOSE 8080

ENTRYPOINT ["/app/beeba-api"]
