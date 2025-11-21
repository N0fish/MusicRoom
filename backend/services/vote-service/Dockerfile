FROM golang:1.22.0-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/vote-service ./cmd/service

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/vote-service /app/vote-service
EXPOSE 3003
CMD ["/app/vote-service"]
