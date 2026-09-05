# bo-pricing

Source for the `pricing` service — CPU-bound by design.

Part of a three-cluster GitOps lab demonstrating progressive delivery gated on
**error-budget burn rate**. Spec: [`bo-platform/docs/BUILD-PLAN.md`](https://github.com/bo-jr/bo-platform/blob/main/docs/BUILD-PLAN.md) ·
Working rules: [`CLAUDE.md`](./CLAUDE.md)

`POST /quote`, iterated SHA-256 scaled by `qty * CPU_BURN_FACTOR` so latency degrades under load. Source only; manifests are rendered by CI into [`bo-deploy`](https://github.com/bo-jr/bo-deploy).
