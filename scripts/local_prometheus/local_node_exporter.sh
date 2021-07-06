#!/usr/bin/env bash
NODE_EXPORTER_BIN=/usr/local/bin/node_exporter
NODE_EXPORTER_DIR=node_exporter-1.1.2.darwin-amd64
NODE_EXPORTER_TAR=${NODE_EXPORTER_DIR}.tar.gz
if [[ -f "$NODE_EXPORTER_BIN" ]];
  then
     echo "$NODE_EXPORTER_BIN exist"
     node_exporter
  else
     curl -L https://github.com/prometheus/node_exporter/releases/download/v1.1.2/${NODE_EXPORTER_TAR} --output ${NODE_EXPORTER_TAR}
     tar zxvf ${NODE_EXPORTER_TAR}
     cp ${NODE_EXPORTER_DIR}/node_exporter ${NODE_EXPORTER_BIN}
     rm -rf ${NODE_EXPORTER_TAR}
     rm -rf ${NODE_EXPORTER_DIR}
     node_exporter
fi