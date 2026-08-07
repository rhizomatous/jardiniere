# concessions

things jardinière wanted to do, couldn't, and does differently instead.

each entry records what was intended, what turned out to be true, what we do
about it, and what would have to change for the original plan to become
possible again. the point is that a later revisit starts from evidence rather
than from re-running the same experiments.

this is not a list of bugs or of work not yet done — `docs/next/plan.md` covers
those. it is a list of places where a deliberate, defensible choice cost us
something real.

## host-side egress proxy, under a bring-your-own runtime

**status:** live, since phase 3.
**forced by:** "bring your own runtime" in `docs/next/plan.md`.

### what we wanted

The plan's phase 3 says:

> sandboxes join an internal network with no route out; the proxy is the only
> egress

with the proxy daemon-resident, alongside the policy engine, the connection
log, and (in phase 4) the OS keychain. The plan claims this gets us network
parity with `sbx`:

> **network** — parity with `sbx` is achievable, because policy enforcement
> lives in the host proxy rather than the isolation layer

### what is actually true

The first half works. The second half does not, on macOS.

Measured against OrbStack (Docker 29.4.0) on macOS, with a listener bound on
the host and a container attached to a `docker network create --internal`
network:

| from an `--internal` network | result |
| --- | --- |
| resolve and fetch `example.com` | fails — DNS itself does not resolve |
| connect to the bridge gateway (`192.168.117.1`) | `Connection refused` |
| resolve `host.docker.internal` | no answer |
| connect to `host.docker.internal`'s address (`0.250.250.254`) | `Network unreachable` |

The same address is reachable from a non-internal network, so the route exists
and `--internal` is what removes it.

Two things follow. The isolation half is genuine: an internal network really
does leave a sandbox with no way out, DNS included. But a host-resident proxy
is not reachable from inside one, so it cannot be the sandbox's egress.

`Connection refused` on the gateway is the tell. The packet arrived somewhere
and was actively refused, which means the gateway is not the macOS host at all
— it is a bridge interface inside the Linux VM that OrbStack runs containers
in. Our daemon is a macOS process. There is a VM boundary between the two that
`--internal` cuts.

### why `sbx` does not have this problem

`sbx` is genuinely host-side-proxy, per `docs/next/research-docker-sandboxes.md`:

> `sandboxd` — host daemon. owns sandbox state, lifecycle, the egress proxy [...]

It gets away with it because each of its sandboxes is a **microVM it creates
itself**. It owns the hypervisor and therefore the virtual NIC, so it can hand
a guest a network whose only route is a host-side listener. Nothing sits in
between.

We are a guest in someone else's VM. Docker Desktop and OrbStack own the Linux
VM our containers live in, and we do not get to configure its routing. This is
the direct cost of not shipping a runtime — which is a requirement, not an
oversight. `sbx` requires its own hypervisor and is Apple-silicon-only on macOS
and KVM-only on Linux; we deliberately run on whatever is already installed.

### what we do instead

The proxy, the policy engine, and the connection log all stay in the daemon,
where the plan wants them. A small dual-homed relay container bridges the gap:

```
  sandbox              relay                    host
  (internal only)      (internal + bridge)      (jardd)

    agent ─────────────► forwards only ────────► proxy ────► internet
                         to the host proxy         │
                                                   ├─ policy + connection log
                                                   └─ keychain (phase 4)
```

The sandbox reaches nothing but the relay. The relay reaches nothing but the
host proxy. Egress enforcement still lives in one host-side place.

Keeping the proxy on the host is not merely tidiness. Phase 4 injects
credentials read from the OS keychain, so that raw values never enter the
sandbox — and a container cannot read the macOS keychain. A proxy that had been
pushed into a container would have to be undone to build phase 4.

### what it costs

- **an extra moving part.** A relay container and a small image to publish and
  keep current, neither of which the plan accounted for.
- **a hop.** Every request crosses the VM boundary twice rather than once.
  Immaterial next to the network round trip, but it is not nothing.
- **a second thing that can be down.** "The daemon is running" is no longer
  sufficient for a sandbox to have egress.
- **it is not the isolation layer doing the work.** The guarantee rests on
  docker's network configuration being what we asked for. A runtime that
  implements `--internal` loosely weakens it, and we would not notice.

### what would let us revisit

The **microVM backend**, already in the plan's deferred list:

> per-sandbox VM with its own kernel, as `sbx` has. via Lima, or natively on
> vz + Firecracker

Owning the hypervisor is exactly what removes this concession. A sandbox in a
VM we created can be given a NIC routed straight at the host proxy, the relay
disappears, and the topology becomes the one phase 3 originally described.

Worth checking at that point, and not before: whether Linux can already skip
the relay today. There the bridge gateway is a real host interface rather than
something inside a VM, so an internal network can very likely reach a host
proxy directly. We use one topology on both platforms because two is twice the
surface to get wrong, and bugs collect in whichever half is used less — but if
the Linux path ever needs to be native, it is probably a short change.
