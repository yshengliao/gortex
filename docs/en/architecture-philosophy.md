# Architecture Philosophy & Design Decisions

> **⚠️ Project Status**
> This project is for research and record-keeping purposes. It is a local replica, assisted by AI, of **architectural philosophies from past commonly used "Platform Engineering / Infrastructure Frameworks"**.
> Its design does not aim to solve a single business problem, but rather to explore and preserve design patterns commonly found in large-scale infrastructure.

## Why use a "Kitchen-sink" (Heavily Encapsulated) Design?

In the modern Go ecosystem, the community generally prefers combining small, standard libraries (e.g., Go 1.22+ native `ServeMux` or `chi`). For typical microservice development, this aligns perfectly with the YAGNI (You Aren't Gonna Need It) principle.

However, when the context shifts to **Platform Engineering** for large-scale infrastructure, the goal shifts from being "lightweight" to enforcing "standardization" and providing "high observability." Gortex replicates exactly this mindset:

1. **Standardization**:
   By using a custom `Context`, a tightly integrated `Config`, and a unified `Logger`, the framework ensures that code across different departments and microservices adheres to a consistent standard when handling requests, logging, and reading configurations. This significantly reduces the cognitive load for DevOps when debugging across different projects.
   Furthermore, the multi-source configuration loading (supporting a mix of YAML, environment variables, and `.env` overwrites) is specifically designed to adapt to various isolated Kubernetes (K8s) environments. Developers can read configuration files during local testing, while maintaining the same Docker image in production by having K8s inject environment variables—both sharing the exact same parsing logic.

2. **Observability Out-of-the-box**:
   To trace complex microservice chains in the past, the framework deeply integrated Distributed Tracing (such as Jaeger and OpenTelemetry). It also includes built-in metrics collection for the `httpclient` connection pool, along with diagnostic endpoints like `/_routes` and `/_monitor`. Business developers do not need to reconfigure Prometheus or Tracing middleware every time they create a new project; all this complexity is absorbed by the framework's foundation layer.

3. **Dependency Consolidation**:
   By consolidating core components like the router, parameter binding, and middleware within the framework itself, dealing with security vulnerabilities or adjusting global behavior (e.g., modifying default CORS policies, adjusting global timeouts) becomes much simpler. You only need to bump the framework version, rather than tracking down which third-party library each individual project decided to use.

## Trade-offs & Technical Debt

While this "keep complexity in the framework" design is friendly to business developers, it means the framework itself carries an extremely high maintenance burden:

* **Decoupling from the Standard Library**: The custom `Context` makes it impossible to seamlessly drop in community middleware designed for native `http.Handler`, requiring adapter layers.
* **Maintenance Cost**: Maintaining an in-house Segment-trie router and HTTP Client connection pool means handling extreme edge cases and performance bottlenecks natively (e.g., preventing Goroutine leaks or handling malicious connections).

## Conclusion

Gortex is not intended to compete with open-source frameworks like Gin, Echo, or the standard library. It serves as a Reference Implementation to record and demonstrate: **When system development shifts from "lightweight single microservices" to "unified infrastructure governance across services," what design decisions and trade-offs must the framework layer make.**
