/*******************************************************************************
 * SafeHarbor REST server.
 * See https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
*/

package main

import (
	"fmt"
	"os"
	"flag"
)

func main() {
	
	var debug *bool = flag.Bool("debug", false, "Run in debug mode: this enables the clearAll REST method.")
	var help *bool = flag.Bool("help", false, "Provide help instructions.")
	var port *int = flag.Int("port", 0, "The TCP port on which the SafeHarborServer should listen. If not set, then the value is taken from the conf.json file.")
	var adapter *string = flag.String("adapter", "", "Network adapter to use (e.g., eth0)")
	
	flag.Parse()
	
	if flag.NArg() > 0 {
		usage()
		os.Exit(2)
	}
	
	if *help {
		usage()
		os.Exit(0)
	}
	
	fmt.Println("Creating SafeHarbor server...")
	var server *Server = NewServer(*debug, *port, *adapter)
	if server == nil { os.Exit(1) }

	server.start()
}

func usage() {
	
	fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
	flag.PrintDefaults()
}

