FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/realtime-service ./cmd/service

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/realtime-service /app/realtime-service
EXPOSE 3004
CMD ["/app/realtime-service"]
