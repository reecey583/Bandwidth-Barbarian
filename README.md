# bandwidth barbarian

A tiny Go CLI to slam your connection on macOS or Linux.

## install

```bash
go build -o bb ./cmd/bb
```

## use

Download mode against a legit test file
```bash
./bb dl --url https://speed.hetzner.de/10GB.bin --conns 64 --time 5m --loop --i-understand
```

Run a local sink to test uploads
```bash
./bb sink --port 8080
./bb ul --url http://127.0.0.1:8080/upload --conns 32 --time 10m
```

The tool prints live Mbps and writes `bb-report.json` when done.

Be nice. Only target endpoints you control or explicit test mirrors.
