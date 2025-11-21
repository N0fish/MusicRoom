FROM golang:1.22.0-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o /out/frontend ./cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/frontend /app/frontend
EXPOSE 5175
CMD ["/app/frontend"]
