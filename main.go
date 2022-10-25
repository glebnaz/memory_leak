package main

import (
	"fmt"
	"runtime"
)

func main() {
	n := 20_000_000
	m := make(map[int][128]byte)
	printAlloc()

	for i := 0; i < n; i++ { // Adds 10 million elements
		m[i] = [128]byte{}
	}
	printAlloc()

	for i := 0; i < n; i++ { // Deletes 10 million elements
		delete(m, i)
	}

	runtime.GC() // Triggers a manual GC
	printAlloc()
	runtime.KeepAlive(m) // Keeps a reference to m so that the map isn’t collected
}

func printAlloc() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("%d mb\n", m.Alloc/(1024*1024))
}
