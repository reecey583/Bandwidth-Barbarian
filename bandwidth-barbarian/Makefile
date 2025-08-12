build:
	go build -o bb ./cmd/bb

test:
	go test ./...

run-dl:
	./bb dl --url https://speed.hetzner.de/10GB.bin --conns 32 --time 2m --loop --i-understand

run-sink:
	./bb sink --port 8080

run-ul:
	./bb ul --url http://127.0.0.1:8080/upload --conns 16 --time 2m
