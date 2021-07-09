# Volume tests
Setup prometheus + node exporter:
```
cd ./scripts/local_prometheus
./local_prometheus.sh
./local_node_exporter.sh
```
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