// Package plans implements the Go-4 PlanService contract.
//
// The package owns orchestration, result validation, metrics, event application,
// and preservation of the elapsed part of a workday. It deliberately does not
// own persistence, HTTP, geocoding, routing, or assignment optimization. Those
// dependencies are injected through the narrow consumer interfaces declared in
// service.go, so implementations from the Go-1, Go-2, and Go-3 modules can be
// connected without adapters when they use the shared contracts package.
package plans
