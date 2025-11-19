SHELL := /bin/bash
SERVICES := backend/services/api-gateway/cmd/service \
						backend/services/auth-service/cmd/service \
						backend/services/user-service/cmd/service \
						backend/services/playlist-service/cmd/service \
						backend/services/realtime-service/cmd/service \
						backend/services/vote-service/cmd/service \
						backend/services/mock-service/cmd/service \
						frontend/cmd/service

.PHONY: up down logs

up:
	docker compose up -d --build

down:
	docker compose down -v

logs:
	docker compose logs -f --tail=200

.PHONY: tidy fmt env

tidy:
	for s in $(SERVICES); do (cd $$s && go mod tidy); done

fmt:
	for s in $(SERVICES); do (cd $$s && go fmt ./...); done

env: #не тестить, пока не работает
	bash deploy/init-env.sh
