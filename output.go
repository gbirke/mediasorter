package main

import (
	"fmt"
)

type Verbosity int

const (
	Quiet Verbosity = iota
	Verbose
	Debug
)

type OutputWriter struct {
	Verbosity Verbosity
}

func (o *OutputWriter) Write(msg string, verbosity Verbosity) {
	if verbosity > o.Verbosity {
		return
	}
	fmt.Println(msg)
}

func (o *OutputWriter) Warn(msg string) {
	o.Write(msg, Quiet)
}

func (o *OutputWriter) Info(msg string) {
	o.Write(msg, Verbose)
}

func (o *OutputWriter) Debug(msg string) {
	o.Write(msg, Debug)
}
