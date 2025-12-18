FROM golang:1.22.0-alpine AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/mock-service ./cmd/service

FROM alpine:3.19
WORKDIR /app
COPY --from=build /out/mock-service /app/mock-service
EXPOSE 3006
CMD ["/app/mock-service"]

