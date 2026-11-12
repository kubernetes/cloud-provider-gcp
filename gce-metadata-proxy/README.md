# gce-metadata-proxy

`gce-metadata-proxy` gives Pods a small, fixed set of Compute Engine metadata server
(`169.254.169.254`, or `fd20:ce::254` on IPv6-only nodes) endpoints and refuses everything else. It
runs on every node as a host-network DaemonSet, and node provisioning redirects Pod traffic for the
metadata server to it.

On a GCE cluster that is not GKE (kOps, Cluster API, kubeadm), every Pod can otherwise ask the
metadata server for the node's service-account token and act as the node. This proxy closes that
off. Pods that need Google Cloud credentials should use
[Workload Identity Federation with Kubernetes](https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-kubernetes),
which issues credentials for the Pod's own Kubernetes service account.

## What Pods can read

The proxy serves `GET` requests for exactly these paths. It fetches each value from the metadata
server itself: it forwards one request header, `Metadata-Flavor`, returns two response headers,
`Content-Type` and `Metadata-Flavor`, and drops the query string.

| Path under `/computeMetadata/v1/` | Default | Why |
|---|---|---|
| `/` (the root itself) | always on | Client libraries detect GCE by fetching the root and checking for the `Metadata-Flavor: Google` response header |
| `project/project-id` | on | Project discovery |
| `project/numeric-project-id` | on | Project discovery |
| `instance/zone` | on | Zone and region discovery |
| `instance/id` | off | Node identity. Telemetry agents use it to label data by host |
| `instance/name` | off | Node identity |
| `instance/hostname` | off | Node identity |
| `instance/machine-type` | off | Node identity |

Set the `GCE_METADATA_PROXY_PATHS` environment variable to turn paths on and off. It takes a
comma-separated list of `+path` and `-path` entries relative to `/computeMetadata/v1/`, for example
`+instance/machine-type,+instance/zone,-project/numeric-project-id`. You can only name paths from
the table; the proxy refuses to start if you name anything else. Adding a path to the table is a
code change on purpose.

The proxy answers `403` to everything else: service-account tokens and identity documents, instance
attributes such as `kube-env`, directory listings, `?recursive=true`, and the legacy `v1beta1` and
`0.1` APIs.

## Deploying

The proxy only listens. The node must redirect Pod traffic to it, and should do so at boot, before
any Pod can start, so that the protection does not depend on scheduling order. GCE serves the
metadata server at `169.254.169.254` to IPv4 and dual-stack instances and at `fd20:ce::254` to
IPv6-only instances, so install the rules for the families the node has:

```sh
# IPv4 and dual-stack nodes
ip addr replace 169.254.169.252/32 dev lo scope host
iptables -w -t nat -I PREROUTING -p tcp -d 169.254.169.254 --dport 80 \
  -j DNAT --to-destination 169.254.169.252:988
iptables -w -I FORWARD -p tcp -d 169.254.169.254 --dport 443 -j REJECT

# IPv6-only nodes
ip -6 addr replace fd20:ce::252/128 dev lo scope host
ip6tables -w -t nat -I PREROUTING -p tcp -d fd20:ce::254 --dport 80 \
  -j DNAT --to-destination '[fd20:ce::252]:988'
ip6tables -w -I FORWARD -p tcp -d fd20:ce::254 --dport 443 -j REJECT
```

Only forwarded traffic traverses `nat PREROUTING` and `FORWARD`, so these rules affect Pods that
have their own network namespace and leave Pods on the host network (and node daemons) alone. Until
the proxy starts on a node, the kernel refuses redirected connections rather than letting them reach
the real server. kOps installs these rules when you set `spec.cloudProvider.gce.podCredentials`.
`deploy/gce-metadata-proxy.yaml` contains the DaemonSet.

By default the proxy listens on both `169.254.169.252:988` and `[fd20:ce::252]:988`. It skips an
address the node does not have, logs that it did so, and fails only if it can bind neither. A
request that arrives over IPv4 goes to the IPv4 metadata server and one that arrives over IPv6 goes
to the IPv6 one, so one DaemonSet serves a fleet that mixes stack types.

The port 443 rules block the metadata server's HTTPS endpoint, which Shielded VMs offer. That
endpoint uses mutual TLS with a root certificate that Compute Engine generates for each VM, and the
client certificate lives on the node under `/run/google-mds-mtls/`. A Pod cannot complete the
handshake without that certificate, and the proxy cannot serve the endpoint either, because it has
no key signed by the VM's root. Rejecting the port makes the outcome explicit rather than depending
on a Pod never obtaining the node's certificate.

CNIs that bypass the host's netfilter tables for Pod traffic (Cilium with eBPF host routing) need
the redirect in their own policy instead.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `--addr` | `169.254.169.252:988,[fd20:ce::252]:988` | Comma-separated listen addresses. The proxy skips an address the node does not have |
| `--upstream-ipv4` | `http://169.254.169.254` | Metadata server for requests that arrive over IPv4 |
| `--upstream-ipv6` | `http://[fd20:ce::254]` | Metadata server for requests that arrive over IPv6 |

The proxy gives the metadata server 5 seconds to answer, ignores HTTP proxy environment variables,
and returns redirects to the Pod instead of following them.

## Building

```sh
make test
make build                      # bin/gce-metadata-proxy
make image IMAGE_REPO=... IMAGE_TAG=...   # multi-arch image via buildx
                                          # defaults: gcr.io/<gcloud project>, short git sha
```

The build produces a static binary (`CGO_ENABLED=0`) and the image runs as non-root. The Pod grants
`NET_BIND_SERVICE` so the proxy can bind port 988.
