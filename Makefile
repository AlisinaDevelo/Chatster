.PHONY: test test-backend test-frontend build-frontend

test: test-backend test-frontend

test-backend:
	cd backend && go test -race ./...

test-frontend:
	cd frontend && npm run test:ci

build-frontend:
	cd frontend && npm run build
