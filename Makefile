SHELL := /bin/bash
SERVICES := backend/services/api-gateway \
						backend/services/auth-service \
						backend/services/user-service \
						backend/services/playlist-service \
						backend/services/realtime-service \
						backend/services/vote-service \
						backend/services/mock-service \
						backend/services/music-provider-service \
						frontend/cmd/service


.PHONY: help
help:
	@echo ""
	@echo "====================== MUSIC ROOM — MAKE HELP ======================"
	@echo ""
	@echo " Main commands:"
	@echo "   make start        - generate all .env files and start the whole project"
	@echo "   make up           - docker compose up -d --build"
	@echo "   make down         - stop all containers"
	@echo "   make re           - restart (down + up)"
	@echo "   make down-v       - WARNING: remove containers + PostgreSQL volume"
	@echo ""
	@echo " Logs & Status:"
	@echo "   make logs         - tail logs (-f, last 200 lines)"
	@echo "   make ps           - list running containers"
	@echo ""
	@echo " Utilities:"
	@echo "   make ip           - print local IP address"
	@echo "   make url          - show frontend URL"
	@echo "   make env          - generate environment files (.env)"
	@echo ""
	@echo " Go services tooling:"
	@echo "   make tidy         - run 'go mod tidy' for all services"
	@echo "   make fmt          - run 'go fmt ./...' for all services"
	@echo ""
	@echo "===================================================================="
	@echo ""


.PHONY: ip url
LOCAL_IP := $(shell ipconfig getifaddr en0 2>/dev/null || ip route get 1.1.1.1 | awk '{print $$7}' | head -1)

ip:
	@echo $(LOCAL_IP)

url:
	@echo "http://$(LOCAL_IP):5175"


.PHONY: env
env:
	LOCAL_IP=$(LOCAL_IP) bash deploy/create_env.sh


.PHONY: up down logs ps down-v re re-v
up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f --tail=200

down-v:
	docker compose down -v

ps:
	docker compose ps

re: down up

re-v: down-v up


.PHONY: db

db: docker compose exec postgres psql -U "$POSTGRES_USER" "$POSTGRES_DB"


.PHONY: tidy fmt
tidy:
	for s in $(SERVICES); do (cd $$s && go mod tidy); done

fmt:
	for s in $(SERVICES); do (cd $$s && go fmt ./...); done


.PHONY: start
start:
	@$(MAKE) env
	@$(MAKE) up
	@echo "The site is available at: http://$(LOCAL_IP):5175"


.PHONY: test-gateway
test-gateway:
	docker compose -f docker-compose.gateway-test.yaml up --build --abort-on-container-exit
	docker compose -f docker-compose.gateway-test.yaml down --remove-orphans

.PHONY: test-go test-mobile test

test-go:
	@echo "Running Go tests (web/backend)..."
	for s in $(SERVICES); do (cd $$s && go test -cover ./...); done

test-mobile:
	@echo "Running mobile tests..."
	$(MAKE) -C mobile test

test: test-go test-mobile
