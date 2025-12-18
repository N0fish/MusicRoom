FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api-gateway ./cmd/service

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/api-gateway /app/api-gateway
COPY --from=build /app/openapi.yaml /app/openapi.yaml
EXPOSE 8080
CMD ["/app/api-gateway"]
