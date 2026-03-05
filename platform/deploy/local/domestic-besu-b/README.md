# Hyperledger Besu Test Network

The `startDev.sh` script sets up and starts a local Hyperledger Besu network using the QBFT consensus mechanism, generating the necessary files and configuring and starting 4 nodes. It can receive the param `-n X` to define the number of starting nodes of the network.

For example:
`startDev.sh -n 8`

---

The `addNewNode.sh` adds a new validator node to the network. It can receive the param `-n X` to define the number of new nodes to be added.

For example:
`addNewNode.sh -n 2`

External dependencies include:

* [Hyperledger Besu](https://besu.hyperledger.org/private-networks/get-started/install/binary-distribution) (downloaded during script execution).
  * [Java JDK](https://www.oracle.com/java/technologies/downloads/).
* [Docker](https://docs.docker.com/engine/) and [Docker Compose](https://docs.docker.com/compose/).
* [jq](https://jqlang.org/download/).
* [curl](https://curl.se/download.html).