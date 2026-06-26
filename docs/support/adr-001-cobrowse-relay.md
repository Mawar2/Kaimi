# ADR-001 — In-product support: co-browse architecture

**Last updated:** 2026-06-26
**Status:** Proposed — gates scaffold for epic #281; must be approved by Malik before any co-browse code is written.
**Related:** Epic #281; children #282–#286.

## Context

Federal BD is high-stakes — a single mistake can cost a real bid. We want an in-product
support surface where a BlueMeta human can **see the customer's Kaimi screen** to help them
in the moment. This ADR decides the architecture for that **co-browse** capability.

Two constraints dominate the decision:

1. **Our core promise is privacy-first, BYO-infra.** Each customer runs their own Kaimi in
   their own GCP project; their solicitation data and proposals never leave their boundary.
   Any "let support see my screen" feature must not quietly contradict that promise.
2. **Co-browse needs a live, long-lived channel** (not request/response), and the natural
   implementation holds per-session state in memory — which interacts badly with autoscaling
   if we're not deliberate.

This ADR covers **Phase A** (one-way live view) and records what is deferred. It does **not**
cover the support chatbot or human chat-takeover ("PHASE mode"); those are tracked separately.

## Decision

1. **DOM-mirroring, not pixel/screen capture.** We mirror the **Kaimi web app's DOM** via an
   rrweb-style recorder and replay it in a support viewer. Support sees **only the Kaimi app**,
   never the rest of the customer's desktop. This is both lower-bandwidth and far less invasive
   than full screen share.

2. **The relay lives inside the customer's own deployment.** There is **no central BlueMeta
   relay** that could see every customer. The customer's Kaimi instance *is* the relay. This
   keeps the data plane inside the customer's GCP boundary, preserving BYO-infra privacy.

3. **Customer-minted, short-lived, revocable share token.** The customer explicitly starts
   sharing, which mints an opaque, high-entropy, single-viewer token (default ~15-min TTL).
   The support viewer connects *to the customer's instance* with that token. The customer can
   revoke at any time; revoke/expiry closes the in-flight viewer. Every mint / connect /
   disconnect / revoke / expire is **audited**.

4. **Masking on by default.** The recorder masks text/input content unless explicitly allowed,
   so we don't transmit bid content support doesn't need to see.

5. **One container, not a separate service.** The relay ships as a new Go package
   (`internal/cobrowse/`) mounted on the **existing dashboard container** — not its own
   deployable unit. A separate service buys no privacy isolation that matters (same GCP
   project, same data plane) and adds per-customer provisioning cost, violating
   "provision lazily."

6. **Handle the stateful-WebSocket-vs-autoscaling hazard with config, not architecture.**
   Relay rooms are in-memory, so the publisher (customer) and subscriber (support) must land on
   the **same instance**. We use **Cloud Run session affinity + low `max-instances`**. This is
   ample for single-tenant BD shops with rare support sessions. **`max-instances` must not be
   raised without first externalizing room state** (e.g. Firestore listener / pub-sub), or
   co-browse silently breaks. This constraint is recorded on the epic and in deployment config.

7. **WebRTC / full-desktop share and interactive control are deferred to Phase C.** Those
   genuinely need their own infra — a TURN/STUN server (e.g. coturn) for NAT traversal, which
   is a real standalone, separately-scaled, separately-secured service. That is the point at
   which a separate container/service earns itself — and it gets its own ADR. Until then:
   `// TODO(phase-C)` markers only, no speculative build.

## Alternatives considered

- **Central BlueMeta relay (multi-tenant).** Simpler viewer auth and one place to operate, but
  it puts a BlueMeta-controlled server in the path of every customer's live workspace —
  directly contradicting BYO-infra privacy. **Rejected.**
- **Full WebRTC screen share now.** Most powerful for support, but far more invasive (whole
  desktop), much larger eng + security lift, and needs TURN infra. **Deferred to Phase C.**
- **Separate co-browse microservice per deployment.** No meaningful isolation gain over a
  package in the dashboard container, more to provision/secure per customer. **Rejected for
  Phase A/B**; revisit only for Phase C's TURN service.
- **High `max-instances` + sticky load balancing without affinity.** Fragile; cross-instance
  room misses are silent. **Rejected** in favor of explicit session affinity + low max-instances
  until/unless room state is externalized.

## Consequences

**Positive**
- Co-browse stays inside the customer's privacy boundary; the BYO-infra promise holds.
- Small surface: one Go package + vendored rrweb assets, riding existing dashboard auth/routing.
- The relay/room abstraction is reusable for PHASE-mode chat takeover later.

**Negative / watch-outs**
- **Hard scaling ceiling until room state is externalized** — `max-instances` is effectively
  capped for the dashboard service while co-browse is in-memory. Documented loudly so nobody
  trips it.
- rrweb is a vendored, version-pinned third-party JS asset (no CDN per our no-external-assets
  constraint) — adds an asset to maintain.
- `internal/cobrowse/` adds a new dependency (`coder/websocket`) — justified on #282, pinned.
- Security-sensitive surface (token lifecycle, audit, masking, time-boxing) requires extra
  review on every related PR.

## Open items to confirm against `main` before scaffold
- Exact dashboard static-asset bundling + router mount points (where the recorder bootstrap and
  `/cobrowse/*` routes hang).
- The existing store/audit sink to reuse for the audit log (do not invent a new one).
