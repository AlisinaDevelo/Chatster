.PHONY: test test-backend test-frontend build-frontend lint lint-backend lint-frontend docker-up

test: test-backend test-frontend

lint: lint-backend lint-frontend

test-backend:
	cd backend && go test -race ./...

test-frontend:
	cd frontend && npm run test:ci

lint-backend:
	cd backend && golangci-lint run ./...

lint-frontend:
	cd frontend && npm run lint

build-frontend:
	cd frontend && npm run build

docker-up:
	docker compose up --build
