/*******************************************************************************
 * SafeHarbor REST server.
 * See https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
 *
 * Copyright Scaled Markets, Inc.
*/

package main

import (
	"fmt"
	"os"
	"flag"
	
	"safeharbor/server"
)

func main() {
	
	var useStubForAr multipleValues
	flag.Var(&useStubForAr, "stub", "Use a stub for the specified scanner")
	
	var help *bool = flag.Bool("help", false, "Provide help instructions.")
	var debug *bool = flag.Bool("debug", false, "Run in debug mode: this enables the clearAll REST method.")
	var nocache *bool = flag.Bool("nocache", false, "Always refresh objects from the database.")
	var stubScanners *bool = flag.Bool("stubs", false, "Use stubs for scanners.")
	var noauthor *bool = flag.Bool("noauthorization", false, "Disable authorization: access control lists are ignored.")
	var allowToggleEmailVerification *bool = flag.Bool("toggleemail", false, "Enable the enableEmailVerification REST function. If enabled, then the server starts with email verification disabled.")
	var publicHostname *string = flag.String("host", "", "The public host name or IP address of the server or load balancer, for reaching SafeHarborServer across the Internet.")
	var port *int = flag.Int("port", 0, "The TCP port on which the SafeHarborServer should listen. If not set, then the value is taken from the conf.json file.")
	var adapter *string = flag.String("adapter", "", "Network adapter to use (e.g., eth0). If not set, then the value is taken from the conf.json file.")
	var secretSalt *string = flag.String("secretkey", "", "Secret value to make session hashes unpredictable.")
	var inMemoryOnly *bool = flag.Bool("inmem", false, "Do not persist the data")
	var noRegistry *bool = flag.Bool("noregistry", false, "Do not use docker registry for managing images - use docker daemon instead.")
	var logfilepath *string = flag.String("logfile", "", "Write all stdout and stderr to file instead of console")

	flag.Parse()
	
	if flag.NArg() > 0 {
		usage()
		os.Exit(2)
	}
	
	if *help {
		usage()
		os.Exit(0)
	}
	
	if *secretSalt == "" {
		fmt.Println("Must specify a random value for -secretkey")
		os.Exit(2)
	}
	
	var stubMap = make(map[string]*struct{})
	var emptyObject *struct{}
	for _, stubName := range useStubForAr {
		stubMap[stubName] = emptyObject
	}
	
	fmt.Println("Creating SafeHarbor server...")
	var svr *server.Server
	var err error
	svr, err = server.NewServer(*debug, *nocache, *stubScanners, stubMap,
		*noauthor, *allowToggleEmailVerification,
		*publicHostname, *port, *adapter, *secretSalt, *inMemoryOnly, *noRegistry)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	if svr == nil {
		os.Exit(1)
	}

	if *logfilepath != "" {
		var logfile *os.File
		var err error
		logfile, err = os.OpenFile(*logfilepath, os.O_RDWR | os.O_APPEND | os.O_CREATE, 0660)   
		if err != nil {          
			fmt.Println("While opening log file:")
			fmt.Println(err.Error())     
			os.Exit(2)
		}
		
		fmt.Println("Logging to " + *logfilepath)
		os.Stdout = logfile
		os.Stderr = logfile
		
		defer logfile.Close()
	}
	
	svr.Start()
}

func usage() {
	
	fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
	flag.PrintDefaults()
}

type multipleValues []string

func (values *multipleValues) Set(newValue string) error {
	
	*values = append(*values, newValue)
	return nil
}

func (values *multipleValues) String() string {
	return fmt.Sprint(*values)
}
	