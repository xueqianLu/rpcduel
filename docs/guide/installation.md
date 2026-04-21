# Installation

## Prebuilt binaries

Download the archive for your OS/arch from the [latest release](https://github.com/xueqianLu/rpcduel/releases),
extract it, and put `rpcduel` somewhere on your `PATH`.

```bash
RPCDUEL_VERSION=0.2.0
curl -fsSL -o rpcduel.tar.gz \
  "https://github.com/xueqianLu/rpcduel/releases/download/v${RPCDUEL_VERSION}/rpcduel_${RPCDUEL_VERSION}_linux_amd64.tar.gz"
tar -xzf rpcduel.tar.gz
sudo mv rpcduel /usr/local/bin/
rpcduel --help
```

## Docker

```bash
docker run --rm ghcr.io/xueqianlu/rpcduel:latest call --rpc https://rpc.example.com
```

## Go install

```bash
go install github.com/xueqianLu/rpcduel@latest
```

## Build from source

```bash
git clone https://github.com/xueqianLu/rpcduel.git
cd rpcduel
make build      # produces ./bin/rpcduel
```

Requires **Go 1.23+**.

## Verify

```bash
rpcduel --version
rpcduel call --rpc https://rpc.ankr.com/eth eth_blockNumber
```

## Next

* Set up [shell completions and man pages](/advanced/completions)
* Learn the [global flags](/guide/global-flags)
* Try a [basic command](/commands/call) or jump straight to the [data-driven workflow](/data-driven/workflow)
