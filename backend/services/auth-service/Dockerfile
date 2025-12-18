FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
WORKDIR /app
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/auth-service ./cmd/service

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/auth-service /app/auth-service
EXPOSE 3001
CMD ["/app/auth-service"]
