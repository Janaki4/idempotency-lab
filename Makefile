.PHONY: up down psql

up:
	docker compose up -d --wait

down:
	docker compose down -v

psql:
	docker compose exec db psql -U lab -d lab
