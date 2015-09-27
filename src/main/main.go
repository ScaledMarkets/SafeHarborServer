/*******************************************************************************
 * SafeHarbor REST server.
 * See https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
*/

package main

import (
	"fmt"
	"os"
)

func main() {
	
	var debug bool = false
	
	if len(os.Args) > 1 {
		if os.Args[1] == "--debug" {
			debug = true
		} else
		if os.Args[1] == "--help" {
			usage()
			os.Exit(0)
		} else
		{
			usage()
			os.Exit(2)
		}
	}
	
	fmt.Println("Creating SafeHarbor server...")
	var server *Server = NewServer(debug)
	if server == nil { os.Exit(1) }

	server.start()
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [--debug]\n", os.Args[0])
}

