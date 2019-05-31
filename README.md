# pnyxdb

An experimental democratic Byzantine Fault Tolerant datastore for consortia.

This program is highly experimental and should not be used in production.

## Installation

Pnyxdb requires Go 1.11+ for module support.
It is highly suggested to clone that project *outside* your GOPATH to install supported dependencies automatically.

```
go get ./...
pnyxdb
```

## Cluster setup

In this short tutorial, we will create a 4 nodes network on a single machine.
For simplicity, we will create 4 directories for our 4 nodes.

```bash
$ mkdir alice bob carol dave
```

*In the following snippets, we will explicitly state in which directory commands should be launched.*

First, let's generate nodes configuration and keyrings.
You may need to setup a `PASSWORD` environment variable for the next commands.
Do not forget to change the `identity` of each node during the prompted questions!

```bash
alice $ pnyxdb init && pnyxdb keys init
bob   $ pnyxdb init && pnyxdb keys init
carol $ pnyxdb init && pnyxdb keys init
dave  $ pnyxdb init && pnyxdb keys init
```

The next step is to modify configuration files to affect different port numbers per node (since they are on the same machine).
For instance, update `bob/config.yaml` to use port `4101` instead of `4100` in `p2p.listen` and `4201` instead of `4200` in `api.listen`.

Now, a connected web of trust must be established between nodes.
At this stage, nodes need to know the other nodes in the network to establish trust.

For instance, if alice wants to trust bob's public key at a high level:

```bash
bob   $ pnyxdb keys export > /tmp/key
alice $ pnyxdb keys import bob --trust high < /tmp/key
alice $ pnyxdb keys ls # check that we know trust bob
```

One node may also want to export another public key in which it has put some trust.
For instance, if carol does only trust alice, alice can export some information about bob to enrich carol's view of the web of trust.

```bash
alice $ pnyxdb keys export > /tmp/bob     # Export bob's public key from alice keyring
alice $ pnyxdb keys sign bob              # Endorse bob's public key
alice $ pnyxdb keys export > /tmp/alice   # Export new public key containing this endorsement

carol $ pnyxdb keys import bob --trust none < /tmp/bob
carol $ pnyxdb keys import alice --trust high < /tmp/alice
carol $ pnyxdb keys show bob
  Identity    : bob
  Trust       : none
  Fingerprint : EF:6F:E2:56:33
  Public key  : 9074D820FA6562C0FB904FDBD9D7A8068A37FCA7F2F5AFD64FF408EF6FE25633
  Status      : Certified
  Approved by : alice (high)
```

After having established the web of trust (each node should 4 `Certified` keys in its keystore), it is time to build some connectivity between nodes.
For now, links must be specified in the `config.yaml` files.

To start a node, simply execute:

```bash
alice $ pnyxdb server start
INFO	Listening	{"type": "P2P", "address": "/ip4/127.0.0.1/tcp/4100/p2p/12D3KooWFynk9nkG9XSGS5EKqHe51H21CR5QYmZgTTrSVDFmgaFV"}
INFO	Listening	{"type": "P2P", "address": "/ip4/172.17.0.1/tcp/4100/p2p/12D3KooWFynk9nkG9XSGS5EKqHe51H21CR5QYmZgTTrSVDFmgaFV"}
INFO	Recovery	{"handler": "ready"}
INFO	Listening	{"type": "API", "address": "127.0.0.1:4200"}
```

Here, P2P endpoints are printed for each available interface.
Copy those addresses, including the hash of public keys, and send them to your peers!
They can be put in the `p2p.peers` section of the configuration file.

Ok, so let's say that you managed to setup a cluster by connecting your nodes.
There is no need to establish full connectivity between every node: PnyxDB internally relies on a very efficient gossip broadcast algorithm that propagates messages to the whole accessible network.

You can connect a client to one of your node by issuing the following command:

```bash
$ pnyxdb client -s 127.0.0.1:4200    # With port maching api.listen configuration option
127.0.0.1:4200>
```

A prompt is available for operations, just type `help` for more information.
For instance, a very basic `set/get` transaction could be:

```bash
127.0.0.1:4200> SET myVar 42
e7dc2197-1c65-4416-9fd2-10a92e0913e3
127.0.0.1:4200> GET myVar
42
127.0.0.1:4200> ADD myVar 12
87d0f4a1-3ebc-41e7-85e6-9d49c67724a4
127.0.0.1:4200> GET myVar
54
```
## License
This project is licensed under the terms of BSD 3-clause Clear license.
by downloading this program, you commit to comply with the license as stated in the LICENSE.md file.
