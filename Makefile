SHELL := /bin/bash
SERVICES := backend/services/api-gateway/cmd/service \
						backend/services/auth-service/cmd/service \
						backend/services/user-service/cmd/service \
						backend/services/playlist-service/cmd/service \
						backend/services/realtime-service/cmd/service \
						backend/services/vote-service/cmd/service \
						backend/services/mock-service/cmd/service \
						frontend/cmd/service


.PHONY: ip url
LOCAL_IP := $(shell ipconfig getifaddr en0 2>/dev/null || ip route get 1.1.1.1 | awk '{print $$7}' | head -1)

ip:
	@echo $(LOCAL_IP)

url:
	@echo "http://$(LOCAL_IP):5175"


.PHONY: env
env:
	LOCAL_IP=$(LOCAL_IP) bash deploy/create_env.sh


.PHONY: up down logs ps rmbd re
up:
	docker compose up -d --build

down:
	docker compose down -v

logs:
	docker compose logs -f --tail=200

rmbd:
	docker compose down -v

ps:
	docker compose ps

re: down up


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
