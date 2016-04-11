/*******************************************************************************
 * SafeHarbor REST server.
 * See https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
*/

package main

import (
	"fmt"
	"os"
	"flag"
	
	//"safeharbor/apitypes"
	"safeharbor/server"
)

func main() {
	
	var oldStdout *os.File = os.Stdout
	var logfile *os.File
	var err error
	logfile, err = os.OpenFile("safeharbor.log", os.O_RDWR | os.O_APPEND | os.O_CREATE, 0660)   
	if err != nil {          
		fmt.Println("While opening log file:")
		fmt.Println(err.Error())     
		os.Exit(2)
	}
	os.Stdout = logfile
	
	var debug *bool = flag.Bool("debug", false, "Run in debug mode: this enables the clearAll REST method.")
	var nocache *bool = flag.Bool("nocache", false, "Always refresh objects from the database.")
	var stubScanners *bool = flag.Bool("stubs", false, "Use stubs for scanners.")
	var noauthor *bool = flag.Bool("noauthorization", false, "Disable authorization: access control lists are ignored.")
	var help *bool = flag.Bool("help", false, "Provide help instructions.")
	var publicHostname *string = flag.String("host", "", "The public host name or IP address of the server")
	var port *int = flag.Int("port", 0, "The TCP port on which the SafeHarborServer should listen. If not set, then the value is taken from the conf.json file.")
	var adapter *string = flag.String("adapter", "", "Network adapter to use (e.g., eth0)")
	var secretSalt *string = flag.String("secretkey", "", "Secret value to make session hashes unpredictable.")
	var inMemoryOnly *bool = flag.Bool("inmem", false, "Do not persist the data")
	var noRegistry *bool = flag.Bool("noregistry", false, "Do not use docker registry - use local docker instead")
	
	flag.Parse()
	
	if flag.NArg() > 0 {
		usage()
		logfile.Close()
		os.Exit(2)
	}
	
	if *help {
		usage()
		logfile.Close()
		os.Exit(0)
	}
	
	if *secretSalt == "" {
		fmt.Println("Must specify a random value for -secretkey")
		logfile.Close()
		os.Exit(2)
	}
	
	fmt.Println("Creating SafeHarbor server...")
	var svr *server.Server
	svr, err = server.NewServer(*debug, *nocache, *stubScanners, *noauthor,
		*publicHostname, *port, *adapter, *secretSalt, *inMemoryOnly, *noRegistry)
	if err != nil {
		fmt.Println(err.Error())
		logfile.Close()
		os.Exit(1)
	}
	if svr == nil {
		logfile.Close()
		os.Exit(1)
	}

	svr.Start()
	
	logfile.Close()
	os.Stdout = oldStdout
}

func usage() {
	
	fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
	flag.PrintDefaults()
}

