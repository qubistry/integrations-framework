#### Local prometheus scraper + node exporter
Just for debugging some metrics in volume tests, run:
```
./local_prometheus.sh
./local_node_exporter.sh
```
Visit UI
```
http://localhost:9090 - Prometheus UI
http://localhost:6711/metrics - first Chainlink node metrics
http://localhost:9100/metrics - node exporter metrics
```
Default docker-compose targets are in `prometheus.yml`

Also watch default config for Prometheus client in `./config/config.yaml`