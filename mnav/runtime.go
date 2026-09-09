// Copyright (c) 2015-2026 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package mnav

import (
	"fmt"
	"math"
	"runtime/metrics"
	"strconv"
	"strings"
	"time"

	"github.com/minio/madmin-go/v4"
)

// Runtime metric names read in more than one place. The rest are named where they
// are rendered.
//
// The set a node reports is whatever its own Go version exposes, so every read
// here is optional and a missing name renders nothing rather than a zero. The
// scheduler's goroutine-state breakdown and the finalizer counters, for one,
// arrived in Go 1.25 and simply appear once the servers carry them.
const (
	mGoroutines  = "/sched/goroutines:goroutines"
	mGomaxprocs  = "/sched/gomaxprocs:threads"
	mCPUTotal    = "/cpu/classes/total:cpu-seconds"
	mCPUUser     = "/cpu/classes/user:cpu-seconds"
	mCPUGC       = "/cpu/classes/gc/total:cpu-seconds"
	mGCCycles    = "/gc/cycles/total:gc-cycles"
	mHeapInUse   = "/memory/classes/heap/objects:bytes"
	mHeapObjects = "/gc/heap/objects:objects"
	mAllocBytes  = "/gc/heap/allocs:bytes"
	mMemTotal    = "/memory/classes/total:bytes"
	mMutexWait   = "/sync/mutex/wait/total:seconds"
)

// goVals is one sample of the Go runtime metrics.
//
// Every value on the wire is summed across the nodes that reported it, gauges
// included: a 17-node cluster reports its goroutine count as the fleet total, so
// "961,763 goroutines" is 56,574 per process. nodes is what turns any of them
// back into a per-process figure, and nothing here reads a map directly.
type goVals struct {
	uints  map[string]uint64
	floats map[string]float64
	nodes  int
	// up is the reported process uptime and how many nodes reported one. Zero
	// nodes means nobody did and it has to be derived.
	up      time.Duration
	upNodes int
}

func runtimeVals(r *madmin.RuntimeMetrics) goVals {
	v := goVals{uints: r.UintMetrics, floats: r.FloatMetrics, nodes: max(r.N, 1)}
	if r.UptimeNodes > 0 {
		v.up = time.Duration(r.UptimeSecs / float64(r.UptimeNodes) * float64(time.Second))
		v.upNodes = r.UptimeNodes
	}
	return v
}

func segmentVals(s madmin.RuntimeSegment) goVals {
	return goVals{uints: s.UintMetrics, floats: s.FloatMetrics, nodes: max(s.N, 1)}
}

// get reads a value from whichever map holds it, so a caller can name a metric
// without tracking whether the runtime types it as an integer or a float.
func (v goVals) get(key string) (float64, bool) {
	if n, ok := v.uints[key]; ok {
		return float64(n), true
	}
	f, ok := v.floats[key]
	return f, ok
}

// node is get divided back down to one process.
func (v goVals) node(key string) (float64, bool) {
	f, ok := v.get(key)
	return f / float64(v.nodes), ok
}

// uptime is the window every rate over these cumulative counters divides by.
//
// Reported when the nodes carry one. Otherwise derived:
// /cpu/classes/total:cpu-seconds accrues one second per thread per second of wall
// clock since the process started, so dividing it by GOMAXPROCS gives that wall
// clock back -- both summed over the same nodes, so the quotient needs no node
// count. The derivation breaks where GOMAXPROCS changed mid-flight, which Go 1.25
// does on its own when a container's CPU limit moves; the reported value does not.
//
// Either way it is a mean: a node restarted ten minutes ago pulls the figure down
// instead of showing up on its own, which is why every rate taken from it is
// labelled per node rather than presented as a fleet fact.
func (v goVals) uptime() (d time.Duration, reported, ok bool) {
	// Only when every node reporting a counter also reported an uptime. Mid
	// rollout the maps still hold all N nodes' counters while the uptime covers
	// the upgraded subset, and rating one against the other inflates the result
	// by however much longer the old nodes have been up.
	if v.upNodes >= v.nodes && v.up > 0 {
		return v.up, true, true
	}
	cpu, okCPU := v.get(mCPUTotal)
	procs, okProcs := v.get(mGomaxprocs)
	if !okCPU || !okProcs || procs <= 0 || cpu <= 0 {
		return 0, false, false
	}
	return time.Duration(cpu / procs * float64(time.Second)), false, true
}

// uptimeRow states the uptime and where it came from: a derived figure is an
// inference from the CPU accounting, and a reported one that only some of the
// fleet answered is a mean over those nodes, not over the cluster.
func (v goVals) uptimeRow() (string, bool) {
	d, reported, ok := v.uptime()
	if !ok {
		return "", false
	}
	switch {
	case reported && v.nodes > 1:
		return coverDuration(d) + " (mean per node)", true
	case reported:
		return coverDuration(d), true
	case v.upNodes > 0:
		return fmt.Sprintf("%s (derived; %d of %d nodes report one)",
			coverDuration(d), v.upNodes, v.nodes), true
	}
	return coverDuration(d) + " (derived, mean per node)", true
}

// runtimeRows builds leaf data in display order. The leaf renderer sorts on the
// key, so the position has to be part of it.
//
// Every add is conditional on the metric being present, because which ones are
// depends on the servers' Go version: a section states what it would like to show
// and quietly omits what this cluster cannot report.
type runtimeRows struct {
	data map[string]string
	n    int
}

func newRuntimeRows() *runtimeRows {
	return &runtimeRows{data: make(map[string]string)}
}

func (r *runtimeRows) add(label, value string) {
	r.data[fmt.Sprintf("%02d:%s", r.n, label)] = value
	r.n++
}

func (r *runtimeRows) addIf(label, value string, ok bool) {
	if ok {
		r.add(label, value)
	}
}

// total states a summed value as the cluster figure with the per-node mean behind
// it. A single node has no mean worth repeating.
func (r *runtimeRows) total(v goVals, label, key string, render func(float64) string) {
	sum, ok := v.get(key)
	if !ok {
		return
	}
	if v.nodes <= 1 {
		r.add(label, render(sum))
		return
	}
	r.add(label, fmt.Sprintf("%s (%s/node)", render(sum), render(sum/float64(v.nodes))))
}

// perNode states only the per-node value, for a setting rather than a
// measurement: GOGC summed over a cluster reads as nonsense, and no reader
// expects a knob to be a fleet total. It is still a mean, so nodes configured
// differently average into something neither of them is set to.
func (r *runtimeRows) perNode(v goVals, label, key string, render func(float64) string) {
	value, ok := v.node(key)
	if !ok {
		return
	}
	r.add(label, render(value))
}

// counter is total with the per-node rate over the process uptime appended, which
// is the only rate available for a value cumulative since start.
func (r *runtimeRows) counter(v goVals, label, key string, up time.Duration, render func(float64) string, unit string) {
	sum, ok := v.get(key)
	if !ok {
		return
	}
	value := render(sum)
	perNode := sum / float64(v.nodes)
	if v.nodes > 1 {
		value += fmt.Sprintf(" (%s/node)", render(perNode))
	}
	if up > 0 && perNode > 0 {
		value += ", " + fmtRate(perNode/up.Seconds(), render, unit)
	}
	r.add(label, value)
}

// share renders one part of a whole: the cluster total, the per-node mean every
// other row in the section carries, and the split that is the point of the
// breakdown. The share alone would be an answer to a question nobody asked twice.
func (r *runtimeRows) share(v goVals, label, key string, whole float64, render func(float64) string) {
	part, ok := v.get(key)
	if !ok {
		return
	}
	r.add(label, render(part)+qualify(v.nodes, "node", part, whole, render))
}

// cpuShare renders a CPU class against the CPU time available since start.
//
// The classes are cumulative and so is their total, by the same clock, so the
// share is mean utilisation over the process lifetime -- the closest thing to a
// rate these counters can give without a window.
func (r *runtimeRows) cpuShare(v goVals, label, key string) {
	part, ok := v.get(key)
	total, okTotal := v.get(mCPUTotal)
	if !ok {
		return
	}
	if !okTotal {
		total = 0
	}
	r.add(label, fmtSecs(part)+" CPU-time"+qualify(v.nodes, "node", part, total, fmtSecs))
}

// hist renders a histogram row, skipping one the reporting Go version omits.
func (r *runtimeRows) hist(m *madmin.RuntimeMetrics, label, key string, nodes int) {
	h, ok := m.HistMetrics[key]
	if !ok {
		return
	}
	line, ok := histLine(h, nodes)
	r.addIf(label, line, ok)
}

// histLine summarizes a runtime histogram: how many samples, their mean, and the
// tail that a mean hides.
//
// Buckets carry the distribution only, so the mean is midpoint-weighted and the
// percentiles are bucket edges -- upper bounds, except in the open-ended top
// bucket, where only a lower bound exists.
func histLine(h metrics.Float64Histogram, nodes int) (string, bool) {
	if len(h.Counts) == 0 || len(h.Buckets) != len(h.Counts)+1 {
		return "", false
	}
	var count uint64
	var weighted float64
	for i, c := range h.Counts {
		if c == 0 {
			continue
		}
		count += c
		lo, loOK := histEdge(h.Buckets[i])
		hi, hiOK := histEdge(h.Buckets[i+1])
		// An unbounded outer edge has no value to average; the finite side is
		// the only estimate the bucket offers.
		switch {
		case !loOK && !hiOK:
			continue
		case !loOK:
			lo = hi
		case !hiOK:
			hi = lo
		}
		weighted += (lo + hi) / 2 * float64(c)
	}
	if count == 0 {
		return "", false
	}
	out := fmtCount(float64(count))
	if nodes > 1 {
		out += fmt.Sprintf(" (%s/node)", fmtCount(float64(count)/float64(nodes)))
	}
	out += ", avg " + fmtSecs(weighted/float64(count))
	for _, q := range []struct {
		label string
		p     float64
	}{{"p50", 0.5}, {"p99", 0.99}} {
		edge, bounded, ok := histQuantile(h, count, q.p)
		switch {
		case !ok:
		case bounded:
			out += fmt.Sprintf(", %s %s", q.label, fmtSecs(edge))
		default:
			out += fmt.Sprintf(", %s >%s", q.label, fmtSecs(edge))
		}
	}
	return out, true
}

// histEdge reports a bucket boundary and whether it is bounded.
//
// The wire carries no infinities: the producer substitutes -math.MaxFloat64 and
// math.MaxFloat64 for the runtime's open outer edges because JSON cannot encode
// Inf. Testing for Inf here therefore never fires, and averaging in a sentinel
// overflows the duration it is rendered as.
func histEdge(v float64) (float64, bool) {
	if math.IsInf(v, 0) || v <= -math.MaxFloat64 || v >= math.MaxFloat64 {
		return 0, false
	}
	return v, true
}

// histQuantile returns the bucket edge the qth sample falls in. bounded is false
// in the open outer bucket, where the edge is a lower bound.
func histQuantile(h metrics.Float64Histogram, count uint64, q float64) (edge float64, bounded, ok bool) {
	// Ceiling rank: flooring puts p99 of two samples at rank 1, which answers
	// with the first bucket instead of the second sample.
	want := uint64(math.Ceil(float64(count) * q))
	want = min(max(want, 1), count)
	var seen uint64
	for i, c := range h.Counts {
		if c == 0 {
			continue
		}
		seen += c
		if seen >= want {
			if hi, ok := histEdge(h.Buckets[i+1]); ok {
				return hi, true, true
			}
			lo, ok := histEdge(h.Buckets[i])
			return lo, false, ok
		}
	}
	return 0, false, false
}

// RuntimeMetricsNavigator provides navigation for Go Runtime metrics
type RuntimeMetricsNavigator struct {
	runtime *madmin.RuntimeMetrics
	parent  MetricNode
	path    string
}

func (node *RuntimeMetricsNavigator) GetOpts() madmin.MetricsOptions {
	return getNodeOpts(node)
}

// NewRuntimeMetricsNavigator creates a new runtime metrics navigator
func NewRuntimeMetricsNavigator(runtime *madmin.RuntimeMetrics, parent MetricNode, path string) *RuntimeMetricsNavigator {
	return &RuntimeMetricsNavigator{runtime: runtime, parent: parent, path: path}
}

func (node *RuntimeMetricsNavigator) GetChildren() []MetricChild {
	if node.runtime == nil {
		return []MetricChild{}
	}
	return []MetricChild{
		{Name: "gc", Description: "Collector cycles, heap occupancy, allocation rate and pauses"},
		{Name: "memory", Description: "Process footprint split by what the runtime holds it for"},
		{Name: "scheduler", Description: "Goroutines by state, threads, run-queue latency and stop-the-world"},
		{Name: "cpu_classes", Description: "Where CPU time went, as a share of what was available"},
		{Name: "sync", Description: "Time goroutines spent blocked on locks"},
		{Name: "last_hour", Description: "Runtime levels and rates over the last hour, by time segment"},
		{Name: "last_day", Description: "Runtime levels and rates over the last day, by time segment"},
	}
}

func (node *RuntimeMetricsNavigator) GetLeafData() map[string]string {
	if node.runtime == nil {
		return map[string]string{"Status": "Runtime metrics not available"}
	}
	v := runtimeVals(node.runtime)
	up, _, _ := v.uptime()

	r := newRuntimeRows()
	r.add("Nodes", formatNodeCount(v.nodes, 1))
	if row, ok := v.uptimeRow(); ok {
		r.add("Uptime", row)
	}
	r.total(v, "Goroutines", mGoroutines, fmtCount)
	r.total(v, "Runnable", "/sched/goroutines/runnable:goroutines", fmtCount)
	r.total(v, "Threads", "/sched/threads/total:threads", fmtCount)
	r.total(v, "GOMAXPROCS", mGomaxprocs, fmtCount)
	r.total(v, "Heap In Use", mHeapInUse, fmtBytes)
	r.total(v, "Heap Objects", mHeapObjects, fmtCount)
	r.counter(v, "GC Cycles", mGCCycles, up, fmtCount, "cycles")
	r.cpuShare(v, "User CPU", mCPUUser)
	r.total(v, "Mutex Wait", mMutexWait, fmtSecs)
	return r.data
}

func (node *RuntimeMetricsNavigator) GetMetricType() madmin.MetricType {
	return madmin.MetricsRuntime
}

// GetMetricFlags requests no historic window: this node refreshes continuously,
// and asking for them here would pull the hour and the day on every tick to
// render nothing that uses them. They are fetched on navigating into one.
func (node *RuntimeMetricsNavigator) GetMetricFlags() madmin.MetricFlags {
	return 0
}

func (node *RuntimeMetricsNavigator) GetParent() MetricNode {
	return node.parent
}

func (node *RuntimeMetricsNavigator) GetPath() string {
	return node.path
}

func (node *RuntimeMetricsNavigator) ShouldPauseRefresh() bool {
	return false
}

func (node *RuntimeMetricsNavigator) GetChild(name string) (MetricNode, error) {
	if node.runtime == nil {
		return nil, fmt.Errorf("no runtime data available")
	}
	switch name {
	case "gc", "memory", "scheduler", "cpu_classes", "sync":
		return &runtimeSectionNode{
			runtime: node.runtime, kind: name,
			parent: node, path: fmt.Sprintf("%s/%s", node.path, name),
		}, nil
	case "last_hour":
		return &runtimeWindowNode{
			seg: node.runtime.LastHour, flags: madmin.MetricsHourStats, window: "hour",
			parent: node, path: fmt.Sprintf("%s/last_hour", node.path),
		}, nil
	case "last_day":
		return &runtimeWindowNode{
			seg: node.runtime.LastDay, flags: madmin.MetricsDayStats, window: "day",
			parent: node, path: fmt.Sprintf("%s/last_day", node.path),
		}, nil
	}
	return nil, fmt.Errorf("child not found: %s", name)
}

// runtimeSectionNode is one themed page of the current sample. The sections differ
// only in which metrics they render, so they share a node and switch on kind.
type runtimeSectionNode struct {
	runtime *madmin.RuntimeMetrics
	kind    string
	parent  MetricNode
	path    string
}

func (node *runtimeSectionNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *runtimeSectionNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRuntime }
func (node *runtimeSectionNode) GetMetricFlags() madmin.MetricFlags { return 0 }
func (node *runtimeSectionNode) GetParent() MetricNode              { return node.parent }
func (node *runtimeSectionNode) GetPath() string                    { return node.path }
func (node *runtimeSectionNode) ShouldPauseRefresh() bool           { return false }
func (node *runtimeSectionNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *runtimeSectionNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *runtimeSectionNode) GetLeafData() map[string]string {
	if node.runtime == nil {
		return map[string]string{"Status": "Runtime metrics not available"}
	}
	v := runtimeVals(node.runtime)
	r := newRuntimeRows()
	r.add("Nodes", formatNodeCount(v.nodes, 1))
	switch node.kind {
	case "gc":
		node.gcRows(v, r)
	case "memory":
		node.memoryRows(v, r)
	case "scheduler":
		node.schedulerRows(v, r)
	case "cpu_classes":
		node.cpuRows(v, r)
	case "sync":
		node.syncRows(v, r)
	}
	// Only the node count made it, so this Go version reports nothing this
	// section knows how to render.
	if r.n == 1 {
		return map[string]string{"Status": "no " + node.kind + " metrics in this sample"}
	}
	return r.data
}

func (node *runtimeSectionNode) gcRows(v goVals, r *runtimeRows) {
	up, _, _ := v.uptime()
	r.counter(v, "Cycles", mGCCycles, up, fmtCount, "cycles")
	if forced, ok := v.get("/gc/cycles/forced:gc-cycles"); ok && forced > 0 {
		r.add("Forced Cycles", fmtCount(forced))
	}
	r.total(v, "Heap Live", "/gc/heap/live:bytes", fmtBytes)
	r.total(v, "Heap In Use", mHeapInUse, fmtBytes)
	r.total(v, "Heap Objects", mHeapObjects, fmtCount)
	r.total(v, "Heap Goal", "/gc/heap/goal:bytes", fmtBytes)
	// GOGC and GOMEMLIMIT are settings, not measurements, so only the per-node
	// value means anything.
	r.perNode(v, "GOGC", "/gc/gogc:percent", func(f float64) string {
		return fmt.Sprintf("%.0f%%", f)
	})
	if limit, ok := v.node("/gc/gomemlimit:bytes"); ok && limit < math.MaxInt64 {
		r.add("GOMEMLIMIT", fmtBytes(limit))
	}
	r.counter(v, "Allocated", mAllocBytes, up, fmtBytes, "")
	r.counter(v, "Allocated Objects", "/gc/heap/allocs:objects", up, fmtCount, "obj")
	r.counter(v, "Freed", "/gc/heap/frees:bytes", up, fmtBytes, "")
	r.total(v, "Scan Work", "/gc/scan/total:bytes", fmtBytes)
	r.hist(node.runtime, "Pauses", "/gc/pauses:seconds", v.nodes)
	r.cpuShare(v, "GC CPU", mCPUGC)
	// A queue that outruns the runs is a leak in the making; both pairs arrived
	// with Go 1.24/1.25 and are absent on older servers.
	for _, q := range []struct{ label, queued, done string }{
		{"Finalizers", "/gc/finalizers/queued:finalizers", "/gc/finalizers/executed:finalizers"},
		{"Cleanups", "/gc/cleanups/queued:cleanups", "/gc/cleanups/executed:cleanups"},
	} {
		queued, ok := v.get(q.queued)
		done, okDone := v.get(q.done)
		if !ok || !okDone || queued == 0 {
			continue
		}
		r.add(q.label, fmt.Sprintf("%s queued, %s run, %s pending",
			fmtCount(queued), fmtCount(done), fmtCount(queued-done)))
	}
}

func (node *runtimeSectionNode) memoryRows(v goVals, r *runtimeRows) {
	whole, _ := v.get(mMemTotal)
	r.total(v, "Total", mMemTotal, fmtBytes)
	// Ordered by what the reader is after -- live data, then heap the allocator
	// holds but is not using, then the runtime's own overhead -- rather than by
	// metric name.
	for _, row := range []struct{ label, key string }{
		{"Heap In Use", mHeapInUse},
		{"Heap Unused", "/memory/classes/heap/unused:bytes"},
		{"Heap Free", "/memory/classes/heap/free:bytes"},
		{"Heap Released", "/memory/classes/heap/released:bytes"},
		{"Goroutine Stacks", "/memory/classes/heap/stacks:bytes"},
		{"OS Stacks", "/memory/classes/os-stacks:bytes"},
		{"MSpan", "/memory/classes/metadata/mspan/inuse:bytes"},
		{"MCache", "/memory/classes/metadata/mcache/inuse:bytes"},
		{"Metadata Other", "/memory/classes/metadata/other:bytes"},
		{"Profiling", "/memory/classes/profiling/buckets:bytes"},
		{"Other", "/memory/classes/other:bytes"},
	} {
		r.share(v, row.label, row.key, whole, fmtBytes)
	}
}

func (node *runtimeSectionNode) schedulerRows(v goVals, r *runtimeRows) {
	up, _, _ := v.uptime()
	r.total(v, "Goroutines", mGoroutines, fmtCount)
	// The state split is what says whether a large count is a problem: waiting
	// goroutines are parked and cost only memory, runnable ones are queued
	// behind a thread. Go 1.25 and later only.
	for _, row := range []struct{ label, key string }{
		{"↳ Running", "/sched/goroutines/running:goroutines"},
		{"↳ Runnable", "/sched/goroutines/runnable:goroutines"},
		{"↳ Waiting", "/sched/goroutines/waiting:goroutines"},
		{"↳ Not In Go", "/sched/goroutines/not-in-go:goroutines"},
	} {
		r.total(v, row.label, row.key, fmtCount)
	}
	r.counter(v, "Created", "/sched/goroutines-created:goroutines", up, fmtCount, "goroutines")
	r.total(v, "OS Threads", "/sched/threads/total:threads", fmtCount)
	r.total(v, "GOMAXPROCS", mGomaxprocs, fmtCount)
	// The ratio is what says whether the run queues are deep: 30k goroutines on
	// 32 threads is a very different machine from 30k on 512.
	if goroutines, ok := v.get(mGoroutines); ok {
		if procs, okProcs := v.get(mGomaxprocs); okProcs && procs > 0 {
			r.add("Per Thread", fmt.Sprintf("%.1f goroutines", goroutines/procs))
		}
	}
	r.hist(node.runtime, "Run Queue Wait", "/sched/latencies:seconds", v.nodes)
	// Stopping is how long it took to bring the world to a halt; total also
	// covers the work done while it was stopped.
	for _, row := range []struct{ label, key string }{
		{"STW Stopping (GC)", "/sched/pauses/stopping/gc:seconds"},
		{"STW Total (GC)", "/sched/pauses/total/gc:seconds"},
		{"STW Stopping (other)", "/sched/pauses/stopping/other:seconds"},
		{"STW Total (other)", "/sched/pauses/total/other:seconds"},
	} {
		r.hist(node.runtime, row.label, row.key, v.nodes)
	}
	r.counter(v, "Cgo Calls", "/cgo/go-to-c-calls:calls", up, fmtCount, "calls")
}

func (node *runtimeSectionNode) cpuRows(v goVals, r *runtimeRows) {
	if row, ok := v.uptimeRow(); ok {
		r.add("Since Start", row)
	}
	r.total(v, "Available", mCPUTotal, fmtSecs)
	// Each class as a share of what was available, which is the utilisation an
	// operator is after; the cumulative seconds alone only say how long the
	// process has been up.
	for _, row := range []struct{ label, key string }{
		{"User", mCPUUser},
		{"Idle", "/cpu/classes/idle:cpu-seconds"},
		{"GC Total", mCPUGC},
		{"↳ Mark Dedicated", "/cpu/classes/gc/mark/dedicated:cpu-seconds"},
		{"↳ Mark Assist", "/cpu/classes/gc/mark/assist:cpu-seconds"},
		{"↳ Mark Idle", "/cpu/classes/gc/mark/idle:cpu-seconds"},
		{"↳ Pause", "/cpu/classes/gc/pause:cpu-seconds"},
		{"Scavenge", "/cpu/classes/scavenge/total:cpu-seconds"},
		{"↳ Assist", "/cpu/classes/scavenge/assist:cpu-seconds"},
		{"↳ Background", "/cpu/classes/scavenge/background:cpu-seconds"},
	} {
		r.cpuShare(v, row.label, row.key)
	}
}

func (node *runtimeSectionNode) syncRows(v goVals, r *runtimeRows) {
	wait, ok := v.get(mMutexWait)
	if !ok {
		return
	}
	r.total(v, "Mutex Wait", mMutexWait, fmtSecs)
	// Goroutine-seconds lost per second of wall clock: below 1 the contention
	// costs less than one goroutine's worth of time, above it there is a queue.
	if up, _, okUp := v.uptime(); okUp && up > 0 {
		blocked := fmtRate(wait/float64(v.nodes)/up.Seconds(), func(f float64) string {
			return strconv.FormatFloat(f, 'f', 2, 64)
		}, "goroutine-s")
		r.add("Blocked Rate", blocked+" per node")
	}
}

// runtimeWindowRow is a value worth a row in a window. A gauge reads as a level;
// a counter arrives already differenced -- the producer stores each segment's own
// increase -- so it is summed, never differenced again. brief marks the few that
// fit a one-line summary.
//
// Only what the producer actually puts in a RuntimeSegment; the live sample's
// wider metric set is not in the window.
type runtimeWindowRow struct {
	label   string
	key     string
	counter bool
	brief   bool
	unit    string
	render  func(float64) string
}

var runtimeWindowRows = []runtimeWindowRow{
	{"Goroutines", mGoroutines, false, true, "", fmtCount},
	{"Heap In Use", mHeapInUse, false, true, "", fmtBytes},
	{"Goroutine Stacks", "/memory/classes/heap/stacks:bytes", false, false, "", fmtBytes},
	{"GC Cycles", mGCCycles, true, true, "cycles", fmtCount},
	// Counted, not timed: the producer stores how many pauses the segment saw,
	// under the histogram's own name.
	{"GC Pauses", "/gc/pauses:seconds", true, true, "pauses", fmtCount},
}

// windowStat is one row aggregated over a whole window: the range a gauge moved
// through, or the total a counter advanced by.
type windowStat struct {
	min, max, sum, delta float64
	samples, segments    int
}

// runtimeWindow is a window prepared for display: the whole-window aggregate of
// every row.
//
// A counter is already a per-segment delta on the wire -- the producer stores
// each segment's own increase -- so the window total is their sum. Differencing
// adjacent segments again would measure the change in the rate rather than the
// activity, and would read three equal segments as no activity at all.
type runtimeWindow struct {
	seg   *madmin.SegmentedRuntimeMetrics
	stats map[string]windowStat
}

func newRuntimeWindow(seg *madmin.SegmentedRuntimeMetrics) *runtimeWindow {
	w := &runtimeWindow{
		seg:   seg,
		stats: make(map[string]windowStat, len(runtimeWindowRows)),
	}
	for i := range seg.Segments {
		if seg.Segments[i].N == 0 {
			continue
		}
		v := segmentVals(seg.Segments[i])
		for _, row := range runtimeWindowRows {
			cur, ok := v.node(row.key)
			if !ok {
				continue
			}
			s := w.stats[row.key]
			if row.counter {
				s.delta += cur
				s.segments++
				w.stats[row.key] = s
				continue
			}
			if s.samples == 0 || cur < s.min {
				s.min = cur
			}
			if cur > s.max {
				s.max = cur
			}
			s.sum += cur
			s.samples++
			w.stats[row.key] = s
		}
	}
	return w
}

// reported is how many segments carry a sample and how many nodes reported across
// them, so a node count can be divided back out of the sum.
func (w *runtimeWindow) reported() (segments, nodes int) {
	for i := range w.seg.Segments {
		if n := w.seg.Segments[i].N; n > 0 {
			segments++
			nodes += n
		}
	}
	return segments, nodes
}

// wholeSecs is the span the window covers.
func (w *runtimeWindow) wholeSecs() int {
	return w.seg.Interval * len(w.seg.Segments)
}

// summarize renders one row over the whole window: the range a gauge moved
// through, or how far a counter advanced and how fast.
func (w *runtimeWindow) summarize(row runtimeWindowRow) (string, bool) {
	s, ok := w.stats[row.key]
	if !ok {
		return "", false
	}
	if row.counter {
		if s.delta <= 0 {
			return "", false
		}
		out := "+" + row.render(s.delta) + "/node over the window"
		// Over the segments that reported, not the window, so a gap does not
		// dilute the rate.
		if secs := s.segments * w.seg.Interval; secs > 0 {
			out += ", " + fmtRate(s.delta/float64(secs), row.render, row.unit)
		}
		return out, true
	}
	if s.samples == 0 {
		return "", false
	}
	out := row.render(s.sum/float64(s.samples)) + "/node avg"
	if s.min != s.max {
		out += fmt.Sprintf(" (min %s, max %s)", row.render(s.min), row.render(s.max))
	}
	return out, true
}

// windowRows is the aggregate view of the whole span, shared by the window node's
// own leaf and its _ALL child.
func (w *runtimeWindow) windowRows(r *runtimeRows) {
	r.add("Coverage", windowCoverage(w.seg.FirstTime, w.wholeSecs()))
	segments, nodes := w.reported()
	r.add("Segments", fmt.Sprintf("%d of %d reported", segments, len(w.seg.Segments)))
	if segments == 0 {
		return
	}
	r.add("Nodes", formatNodeCount(nodes, segments))
	for _, row := range runtimeWindowRows {
		line, ok := w.summarize(row)
		r.addIf(row.label, line, ok)
	}
}

// describeRuntimeSegment renders one segment on a single line: the levels an
// operator scans down the column for, and the counters only where they moved.
func describeRuntimeSegment(w *runtimeWindow, i int) string {
	v := segmentVals(w.seg.Segments[i])
	var parts []string
	for _, row := range runtimeWindowRows {
		if !row.brief {
			continue
		}
		value, ok := v.node(row.key)
		if !ok {
			continue
		}
		if row.counter {
			if value > 0 {
				parts = append(parts, row.label+" +"+row.render(value))
			}
			continue
		}
		parts = append(parts, row.label+" "+row.render(value))
	}
	if len(parts) == 0 {
		return formatNodeCount(w.seg.Segments[i].N, 1) + ", no runtime values recorded"
	}
	return strings.Join(parts, ", ") + " per node"
}

// runtimeWindowNode is one persisted runtime window -- the hour or the day -- as a
// row per time segment plus one navigable child per segment, and an _ALL entry
// that ranges the gauges and totals the counters over the whole span.
type runtimeWindowNode struct {
	seg    *madmin.SegmentedRuntimeMetrics
	flags  madmin.MetricFlags
	window string
	parent MetricNode
	path   string
}

func (node *runtimeWindowNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *runtimeWindowNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRuntime }
func (node *runtimeWindowNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *runtimeWindowNode) GetParent() MetricNode              { return node.parent }
func (node *runtimeWindowNode) GetPath() string                    { return node.path }
func (node *runtimeWindowNode) ShouldPauseRefresh() bool           { return true }

// hasSegments reports whether the window can be placed on a timeline at all. An
// interval of zero would stamp every segment with the same time.
func (node *runtimeWindowNode) hasSegments() bool {
	return node.seg != nil && node.seg.Interval > 0 && len(node.seg.Segments) > 0
}

func (node *runtimeWindowNode) GetChildren() []MetricChild {
	if !node.hasSegments() {
		return []MetricChild{}
	}
	w := newRuntimeWindow(node.seg)
	children := []MetricChild{{
		Name: "_ALL",
		Description: "Every segment in the window combined. " +
			windowCoverage(node.seg.FirstTime, w.wholeSecs()),
	}}
	owners := segmentSecOwners(node.seg.FirstTime, node.seg.Interval, len(node.seg.Segments))
	withDate := windowCrossesDay(node.seg.FirstTime, node.seg.Interval, len(node.seg.Segments))
	// Newest first, as every other family lists a window: the reader is here to
	// see what just happened, not to read a day from the beginning.
	for i := len(node.seg.Segments) - 1; i >= 0; i-- {
		if node.seg.Segments[i].N == 0 {
			continue
		}
		start := segmentStart(node.seg.FirstTime, node.seg.Interval, i)
		name := segmentKey(start)
		if owners[name] != i {
			continue
		}
		children = append(children, MetricChild{
			Name:        name,
			Description: segmentDescTime(start, withDate) + ", " + describeRuntimeSegment(w, i),
		})
	}
	return children
}

func (node *runtimeWindowNode) GetChild(name string) (MetricNode, error) {
	if !node.hasSegments() {
		return nil, fmt.Errorf("no last-%s runtime segments available", node.window)
	}
	leaf := func(index int) MetricNode {
		return &runtimeSegmentNode{
			win: newRuntimeWindow(node.seg), index: index, flags: node.flags,
			parent: node, path: node.path + "/" + name,
		}
	}
	if name == "_ALL" {
		return leaf(-1), nil
	}
	owners := segmentSecOwners(node.seg.FirstTime, node.seg.Interval, len(node.seg.Segments))
	if i, ok := owners[name]; ok {
		return leaf(i), nil
	}
	return nil, fmt.Errorf("time segment not found: %s", name)
}

func (node *runtimeWindowNode) GetLeafData() map[string]string {
	if status := windowStatus(node.seg, node.window); status != "" {
		return map[string]string{"Status": status}
	}
	// The window aggregate only. Each segment states its own levels and rates in
	// the description of the child that opens it, so repeating them here as rows
	// pushed the totals off the top of a day's worth of them.
	r := newRuntimeRows()
	newRuntimeWindow(node.seg).windowRows(r)
	return r.data
}

// runtimeSegmentNode is one segment of a window, or the whole window when index
// is negative.
type runtimeSegmentNode struct {
	win    *runtimeWindow
	index  int
	flags  madmin.MetricFlags
	parent MetricNode
	path   string
}

func (node *runtimeSegmentNode) GetOpts() madmin.MetricsOptions     { return getNodeOpts(node) }
func (node *runtimeSegmentNode) GetMetricType() madmin.MetricType   { return madmin.MetricsRuntime }
func (node *runtimeSegmentNode) GetMetricFlags() madmin.MetricFlags { return node.flags }
func (node *runtimeSegmentNode) GetParent() MetricNode              { return node.parent }
func (node *runtimeSegmentNode) GetPath() string                    { return node.path }
func (node *runtimeSegmentNode) ShouldPauseRefresh() bool           { return true }
func (node *runtimeSegmentNode) GetChildren() []MetricChild         { return []MetricChild{} }

func (node *runtimeSegmentNode) GetChild(_ string) (MetricNode, error) {
	return nil, fmt.Errorf("no children")
}

func (node *runtimeSegmentNode) GetLeafData() map[string]string {
	w := node.win
	r := newRuntimeRows()
	if node.index < 0 {
		w.windowRows(r)
		return r.data
	}
	seg := w.seg.Segments[node.index]
	if seg.N == 0 {
		return map[string]string{"Status": "no node reported this time segment"}
	}
	v := segmentVals(seg)
	interval := w.seg.Interval
	r.add("Time Segment", windowCoverage(segmentStart(w.seg.FirstTime, interval, node.index), interval))
	r.add("Nodes", formatNodeCount(seg.N, 1))
	for _, row := range runtimeWindowRows {
		perNode, ok := v.node(row.key)
		if !ok {
			continue
		}
		if !row.counter {
			r.total(v, row.label, row.key, row.render)
			continue
		}
		// Already this segment's own increase, so it is stated as such rather
		// than as a level.
		value := "+" + row.render(perNode) + "/node"
		if interval > 0 {
			value += " (" + fmtRate(perNode/float64(interval), row.render, row.unit) + ")"
		}
		r.add(row.label, value)
	}
	return r.data
}
