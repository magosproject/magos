.PHONY: install build dev start lint format fix

install:
	npm install

build:
	npm run build

dev:
	npm run dev

start:
	npm run start

lint:
	npm run lint
	npm run typecheck

format:
	npm run format

fix:
	@make format
	npm run lint:fix
