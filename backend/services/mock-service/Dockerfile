FROM golang:1.22.0-alpine AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/service ./cmd/service

FROM alpine:3.19
WORKDIR /app
COPY --from=build /out/service /app/service
EXPOSE 3006
CMD ["/app/service"]

