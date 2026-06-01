// Package jobqueue schedules local CLI parallel benchmark runs with River.
//
// This package is not the hosted Bench runner control-plane queue. Hosted
// runners use internal/benchsvc and the bench_jobs table. Keep this package
// scoped to bench-cli local parallel execution.
package jobqueue
