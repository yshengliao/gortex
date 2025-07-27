# Gortex Best Practices Guide

Welcome to the Gortex framework best practices documentation. These guides provide comprehensive information on how to effectively use Gortex features in production applications.

## 📚 Available Guides

### 1. [Context Handling](./context-handling.md)
Learn how to properly manage context lifecycle, propagate cancellation signals, and avoid common context-related pitfalls.

**Topics covered:**
- Context lifecycle management
- Cancellation signal propagation
- Common context leak patterns
- Working with goroutines
- Timeout best practices
- Real-world HTTP request tracing example

### 2. [Observability Setup](./observability-setup.md)
Comprehensive guide to setting up metrics, tracing, and logging for production monitoring.

**Topics covered:**
- Metrics collector configuration and optimization
- Tracing strategy and sampling
- Structured logging setup
- Monitoring dashboard design
- Alert rules configuration
- Prometheus & Grafana integration

### 3. [API Documentation](./api-documentation.md)
Best practices for documenting your APIs using Gortex's built-in documentation features.

**Topics covered:**
- Struct tag design patterns
- Documentation versioning strategies
- Custom themes and branding
- API grouping and organization
- CI/CD integration
- Documentation as code practices

## 🚀 Quick Start

If you're new to Gortex, we recommend reading the guides in this order:

1. Start with **Context Handling** to understand the fundamental async patterns
2. Move to **Observability Setup** to learn how to monitor your application
3. Finally, read **API Documentation** to learn how to document your APIs effectively

## 💡 Key Principles

Throughout these guides, we emphasize several key principles:

- **Production-Ready**: All examples and patterns are designed for production use
- **Performance-Conscious**: Recommendations consider performance implications
- **Security-First**: Security best practices are highlighted throughout
- **Maintainable**: Focus on patterns that scale with your team and codebase

## 📝 Contributing

Found an issue or have a suggestion? Please contribute to these guides:

1. Submit issues for unclear documentation
2. Propose new best practices based on your experience
3. Share real-world examples that might help others

## 🔗 Additional Resources

- [Gortex GitHub Repository](https://github.com/yshengliao/gortex)
- [Example Applications](../../examples/)
- [API Reference](https://pkg.go.dev/github.com/yshengliao/gortex)

---

Last updated: 2025-01-26