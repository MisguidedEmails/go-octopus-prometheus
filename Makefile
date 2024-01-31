.PHONY: all
all:
	docker-compose up || docker-compose rm --force
