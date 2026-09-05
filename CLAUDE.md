# bo-pricing

Source for the `pricing` service. **Source only** — no manifests live here.

Canonical spec: [`bo-platform/docs/BUILD-PLAN.md`](https://github.com/bo-jr/bo-platform/blob/main/docs/BUILD-PLAN.md)
§4 (layout), Phase 2 (the service), Phase 6 scenario 4 (downstream failure).

## What this service is

`POST /quote`. **CPU-bound by design**: iterated SHA-256 over line items, iteration count
scaled by `qty * CPU_BURN_FACTOR`.

The CPU burn is not decoration. **Latency here must degrade under load rather than
staying flat** — that is what makes k6 load meaningful, what makes the latency SLO able
to fail, and what makes Phase 6 scenario 4 (degrade `pricing` while the canary is on
`storefront`) teach how cascading latency actually presents at the edge.

If `pricing` responds in constant time regardless of load, the scenario is not testing
anything and the CPU knob is miscalibrated.

## The name

**The `bo-` prefix stops at the repo boundary.** Inside the cluster this service is
`pricing`. Never `bo-pricing`. The image is the sole exception:
`ghcr.io/bo-jr/bo-pricing`. The chart takes `image.repository` **with** the prefix,
`name` **without**.

## Required of every service, without exception

- `GET /healthz` (liveness), `GET /readyz` (checks downstream deps)
- `GET /metrics` — `http_requests_total{service,route,status,version}` and
  `http_request_duration_seconds` histogram, same labels
- OTel tracing, W3C traceparent propagation, OTLP export to the local Alloy
- Structured JSON logs to stdout including `trace_id`
- Graceful shutdown on SIGTERM with connection draining

Shared behaviour comes from `bo-service-kit`. Telemetry and chaos code belongs there.

## Chaos knobs

| Var | Effect |
|---|---|
| `FAILURE_RATE` | float 0.0–1.0; that fraction of requests return 500 |
| `EXTRA_LATENCY_MS` | int; sleep injected before responding |
| `CPU_BURN_FACTOR` | int; multiplier on hash iterations — **this service only** |
| `APP_VERSION` | string; must appear as a Prometheus label and a pod label |

Resource limits matter more here than anywhere else: a CPU-bound service at default
requests will get throttled in ways that look like application latency. Set them
explicitly and know what you set.

## What lives here

```
cmd/pricing/main.go
Dockerfile                    # multi-arch, built natively per arch
chart-values.yaml             # values for the shared chart
.github/workflows/ci.yml      # calls the reusable workflow
```

Pin `bo-service-chart` **by exact version**.

## What must never live here

- Rendered manifests — CI output, committed to `bo-deploy`
- A `manifests/` or `base/` directory — the shared chart replaces it

## Metrics discipline

**Never label a Prometheus metric with a commit SHA, image digest, or Rollout hash.**
Unbounded cardinality. Those belong in GitHub Deployments and Discord messages.
## Non-negotiable (inherited from `bo-platform/CLAUDE.md`)

- **No floating tags. Ever.** Not `latest`, `lts`, `stable`, or partial semver (`:1`,
  `:1.2`). Images pinned by **manifest-list digest**, charts by exact semver.
- **Pin the index digest, never a per-arch digest.** A platform-specific digest pulls
  fine on one machine and fails `no match for platform` on the other. This is the most
  likely portability bug in the lab.
- **Cross-platform, always.** Everything must work on `darwin/arm64` (MacBook, the
  runtime target) and `linux/amd64` (Windows/WSL2, build and test only). Images build
  `linux/amd64,linux/arm64`.
- **LF line endings**, enforced by `.gitattributes`. A CRLF `.sh` inside a Linux image
  fails as `bad interpreter: /bin/bash^M`.
- **When something fails, check architecture first** — the usual cause of
  `ImagePullBackOff` and `exec format error` here.
- If reality contradicts the plan, **stop and say so.** Do not improvise around it;
  record the outcome in `bo-platform/docs/DECISIONS.md`.
