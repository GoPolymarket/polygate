.PHONY: build run docker-build docker-run

build:
	go build -o bin/polygate-server ./cmd/server

run: build
	./bin/polygate-server

docker-build:
	docker build -t polygate:latest .

docker-run:
	docker-compose up -d

clean:
	rm -rf bin/
