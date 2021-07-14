# Volume tests
Setup hardhat with hardhat.config.json from that dir
```
docker run --rm -it -p 8545:8545 -v $(pwd)/suite/volume/hardhat.config.js:/usr/app/hardhat.config.js smartcontract/hardhat-network
```
Run compose
```
docker compose down --remove-orphans && docker compose up
```
Run tests
```
make test_volume
```
Watch prom and node exporter here
```
http://localhost:9090 - prom
http://localhost:9100/metrics - node exporter
```