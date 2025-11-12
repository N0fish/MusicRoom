FROM golang:1.22-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o /out/server ./cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/server /app/server
# env можно править через compose:
# ENV API_URL=http://localhost:8080
# ENV WS_URL=ws://localhost:3004/ws
EXPOSE 5175
CMD ["/app/server"]
