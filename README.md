# go-api-gateway

It is a work-in-progress API Gateway built in Go, designed for learning and experimentation. It includes various essential features commonly found in production API Gateways.

Features:

-   [x] URL Rewriting
-   [x] Rate Limiting
-   [ ] Throttling
-   [x] Authentication (ref: https://konghq.com/blog/engineering/jwt-kong-gateway)
-   [ ] Caching
-   [x] TLS Termination
-   [ ] Orchestration/Aggregation (Ambitious feature) (really cool tho)
-   [ ] Protocol Translation
-   [x] IP Whitelisting
-   [x] Circuit Breaker
-   [x] Logging
-   [x] Tracing

## Getting Started

To get started with go-api-gateway, clone the repository and follow the setup instructions below.

### Prerequisites

Go (version 1.22 or later)
Docker (for containerization and deployment)

### Installation

1. Install dependencies

```sh
go mod tidy
```

2. Build and run the gateway

```sh
make run
```

### Configuration

Configuration options can be found in the `config.yaml` file. Customize it as needed for your environment/use case.

### Contributing

Feel free to contribute to this project by adding more features, improving existing ones, or fixing bugs. Let's build something amazing together!
