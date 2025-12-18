FROM golang:1.22.0-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/user-service ./cmd/service

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/user-service /app/user-service
COPY --from=build /app/avatars /app/avatars
EXPOSE 3005
CMD ["/app/user-service"]
