# hotpod

A controllable load generation server for testing Kubernetes HPA and
autoscaling behaviors.

Unlike traffic generators (wrk, ab, hey), hotpod is the *target* that receives
traffic and performs configurable work in response.

## Quick Start

```bash
go build -o hotpod ./cmd/hotpod
./hotpod
```

Server listens on `:8080` by default.

## License

MIT
