# Using Docker

- [Using Docker](#using-docker)
  - [Introduction](#introduction)
  - [Docker volumes](#docker-volumes)
  - [Known error messages when starting the btcd container](#known-error-messages-when-starting-the-btcd-container)
  - [Examples](#examples)
    - [Preamble](#preamble)
    - [Full node without RPC port](#full-node-without-rpc-port)
    - [Full node with RPC port](#full-node-with-rpc-port)
    - [Full node with RPC port running on TESTNET](#full-node-with-rpc-port-running-on-testnet)

## Introduction

With Docker you can easily set up _btcd_ to run your Bitcoin full node. You can find the official _btcd_ Docker images on Docker Hub [btcsuite/btcd](https://hub.docker.com/r/btcsuite/btcd). The Docker source file of this image is located at [Dockerfile](https://github.com/filechain/filechain/blob/master/Dockerfile).

This documentation focuses on running Docker container with _docker-compose.yml_ files. These files are better to read and you can use them as a template for your own use. For more information about Docker and Docker compose visit the official [Docker documentation](https://docs.docker.com/).

## Docker volumes

**Special diskspace hint**: The following examples are using a Docker managed volume. The volume is named _btcd-data_ This will use a lot of disk space, because it contains the full Bitcoin blockchain. Please make yourself familiar with [Docker volumes](https://docs.docker.com/storage/volumes/).

The _btcd-data_ volume will be reused, if you upgrade your _docker-compose.yml_ file. Keep in mind, that it is not automatically removed by Docker, if you delete the btcd container. If you don't need the volume anymore, please delete it manually with the command:

```bash
docker volume ls
docker volume rm btcd-data
```

For binding a local folder to your _btcd_ container please read the [Docker documentation](https://docs.docker.com/). The preferred way is to use a Docker managed volume.

## Known error messages when starting the btcd container

We pass all needed arguments to _btcd_ as command line parameters in our _docker-compose.yml_ file. It doesn't make sense to create a _btcd.conf_ file. This would make things too complicated. Anyhow _btcd_ will complain with following log messages when starting. These messages can be ignored:

```bash
Error creating a default config file: open /sample-btcd.conf: no such file or directory
...
[WRN] BTCD: open /root/.btcd/btcd.conf: no such file or directory
```

## Examples

### Preamble

All following examples uses some defaults:

- container_name: btcd
  Name of the docker container that is be shown by e.g. `docker ps -a`

- hostname: btcd **(very important to set a fixed name before first start)**
  The internal hostname in the docker container. By default, docker is recreating the hostname every time you change the _docker-compose.yml_ file. The default hostnames look like _ef00548d4fa5_. This is a problem when using the _btcd_ RPC port. The RPC port is using a certificate to validate the hostname. If the hostname changes you need to recreate the certificate. To avoid this, you should set a fixed hostname before the first start. This ensures, that the docker volume is created with a certificate with this hostname.

- restart: unless-stopped
  Starts the _btcd_ container when Docker starts, except that when the container is stopped (manually or otherwise), it is not restarted even after Docker restarts.

To use the following examples create an empty directory. In this directory create a file named _docker-compose.yml_, copy and paste the example into the _docker-compose.yml_ file and run it.

```bash
mkdir ~/btcd-docker
cd ~/btcd-docker
touch docker-compose.yaml
nano docker-compose.yaml (use your favourite editor to edit the compose file)
docker-compose up (creates and starts a new btcd container)
```

With the following commands you can control _docker-compose_:

`docker-compose up -d` (creates and starts the container in background)

`docker-compose down` (stops and delete the container. **The docker volume btcd-data will not be deleted**)

`docker-compose stop` (stops the container)

`docker-compose start` (starts the container)

`docker ps -a` (list all running and stopped container)

`docker volume ls` (lists all docker volumes)

`docker logs btcd` (shows the log )

`docker-compose help` (brings up some helpful information)

### Full node without RPC port

Let's start with an easy example. If you just want to create a full node without the need of using the RPC port, you can use the following example. This example will launch _btcd_ and exposes only the default p2p port 8333 to the outside world:

```yaml
version: "2"

services:
  btcd:
    container_name: btcd
    hostname: btcd
    image: btcsuite/btcd:latest
    restart: unless-stopped
    volumes:
      - btcd-data:/root/.btcd
    ports:
      - 8333:8333

volumes:
  btcd-data:
```

### Full node with RPC port

To use the RPC port of _btcd_ you need to specify a _username_ and a very strong _password_. If you want to connect to the RPC port from the internet, you need to expose port 8334(RPC) as well.

```yaml
version: "2"

services:
  btcd:
    container_name: btcd
    hostname: btcd
    image: btcsuite/btcd:latest
    restart: unless-stopped
    volumes:
      - btcd-data:/root/.btcd
    ports:
      - 8333:8333
      - 8334:8334
    command:
      [
        "--rpcuser=[CHOOSE_A_USERNAME]",
        "--rpcpass=[CREATE_A_VERY_HARD_PASSWORD]",
      ]

volumes:
  btcd-data:
```

### Full node with RPC port running on TESTNET

To run a node on testnet, you need to provide the _--testnet_ argument. The ports for testnet are 18333 (p2p) and 18334 (RPC):

```yaml
version: "2"

services:
  btcd:
    container_name: btcd
    hostname: btcd
    image: btcsuite/btcd:latest
    restart: unless-stopped
    volumes:
      - btcd-data:/root/.btcd
    ports:
      - 18333:18333
      - 18334:18334
    command:
      [
        "--testnet",
        "--rpcuser=[CHOOSE_A_USERNAME]",
        "--rpcpass=[CREATE_A_VERY_HARD_PASSWORD]",
      ]

volumes:
  btcd-data:
```
