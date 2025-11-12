SHELL := /bin/bash

.PHONY: up down logs

up:
	docker compose up -d --build

down:
	docker compose down -v

logs:
	docker compose logs -f --tail=200
