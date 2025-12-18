FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/playlist-service ./cmd/service

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/playlist-service /app/playlist-service
EXPOSE 3002
CMD ["/app/playlist-service"]
