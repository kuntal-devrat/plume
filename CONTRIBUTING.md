# Contributing to Plume

Thank you for your interest in contributing to Plume! Plume is a high-performance 2D layout and vector graphics engine written in pure Go with zero Cgo dependencies.

## Design Principles

1. **Zero External Dependencies & Zero Cgo**: Plume must always build with `CGO_ENABLED=0` across Linux, macOS, Windows, and WebAssembly (`GOOS=js GOARCH=wasm`).
2. **Strict W3C Conformance**: Layout behavior must adhere strictly to the W3C CSS Flexible Box Layout and CSS Grid Layout specifications.
3. **Zero Heap Allocations in Hot Paths**: Layout passes and raster scanline loops must leverage pre-allocated scratch arenas and reuse memory to minimize GC pressure.
4. **Mechanical Sympathy**: Use data-oriented structures (SoA / flat slices) and SIMD acceleration (e.g. AVX2 blend routines) where appropriate, with pure-Go fallbacks.

## Development Setup

### Prerequisites

- Go 1.21 or higher installed.
- Git.

### Cloning and Building

```bash
git clone https://github.com/plume-engine/plume.git
cd plume
go test ./...
```

### Running the Example

Verify that the graphics and layout pipeline works end-to-end:

```bash
go run ./examples
```

This renders a 1280x800 dashboard scene and generates `output.png`.

## Testing & Conformance

### Unit Tests and Race Detector

All contributions must pass all package unit tests with the Go race detector enabled:

```bash
go test -v -race ./...
```

### W3C Web Platform Tests (WPT)

The layout engine includes an automated suite of official W3C Flexbox and CSS Grid conformance tests:

```bash
go test -v ./layout -run TestWPT
go test -v ./layout -run TestGrid
```

Any changes to `layout/` must not regress W3C conformance scores.

### Benchmarks

To verify that changes do not introduce performance regressions or unnecessary heap allocations:

```bash
go test -bench=. -benchmem ./...
```

### WebAssembly Build

Ensure that the WebAssembly target continues to compile:

```bash
GOOS=js GOARCH=wasm go build -o /dev/null ./cmd/wasm
```

## Pull Request Guidelines

1. **Keep PRs focused**: Address a single feature, bug fix, or performance improvement per PR.
2. **Add tests**: Include unit tests covering new functionality or bug reproductions.
3. **Format your code**: Run `gofmt -s -w .` and `go vet ./...` before submitting.
4. **Maintain documentation**: Keep doc comments clear, idiomatic, and up to date for all exported types, methods, and packages.
