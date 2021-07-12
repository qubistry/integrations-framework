# Chainlink Integration Framework

[![Go Report Card](https://goreportcard.com/badge/github.com/smartcontractkit/integrations-framework)](https://goreportcard.com/report/github.com/smartcontractkit/integrations-framework)
![Tests](https://github.com/smartcontractkit/integrations-framework/actions/workflows/test.yaml/badge.svg)
![Lint](https://github.com/smartcontractkit/integrations-framework/actions/workflows/lint.yaml/badge.svg)

A framework for interacting with chainlink nodes, environments, and other blockchain systems.
The framework is primarilly intended to facillitate testing chainlink features and stability.

## WIP

This framework is still very much a work in progress, and will have frequent changes, many of which will probably be
breaking.

## How to Test

<<<<<<< HEAD
1. Have a K8s cluster running that you can connect to locally. [Minikube](https://minikube.sigs.k8s.io/docs/)
   works well for local testing.
2. Run `go test ./...`
=======
1. Start a local hardhat network. You can easily do so by using our
 [docker container](https://hub.docker.com/r/smartcontract/hardhat-network). You could also deploy
 [your own local version](https://hardhat.org/hardhat-network/), if you are so inclined.
   ```
   docker run --rm -it -p 8545:8545 smartcontract/hardhat-network
   ```
2. Start few local chainlink nodes, utilizing our `docker-compose` setup
   [here](https://github.com/smartcontractkit/chainlink-node-compose)
   (set your Docker-Preferences->Resourses->RAM to 6Gb min)
   ```
   docker compose up
   ```
   clean db with `docker compose down` if needed
3. Run `make install-deps`
4. Run a test mode
    ```
    make test
    make test_race
    make test_nightly
    ```
   test_race - race detector on, no parallel
   
   test_nightly - run tests 20 times in a row, no parallel
>>>>>>> 02d066a16154b729b298fe57bf1eb97c83abd7e9

## Example Usage

You can see our tests for some basic usage examples. The most complete can be found in `contracts/contracts_test.go`
