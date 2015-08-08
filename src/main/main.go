/*
SafeHarbor REST server.
See https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
*/

package main

import (
	"fmt"
	//"io"
	"os"
	"net"
	"net/http"
)

func main() {
	
	fmt.Println("Starting SafeHarbor server")
	
	
	
	// Read configuration. (Defined in a JSON file.)
	fmt.Println("Reading configuration")
	var config *Configuration
	var err error
	config, err = getConfiguration()
	if err != nil {
		panic(err)
		//fmt.Println(err.Error())
		//os.Exit(1)
	}
	
	// Instantiate SafeHarbor server, with the configuration that we just read.
	fmt.Println("Instantiating server")
	var server *Server = NewServer(*config)
	
	// Instantiate an HTTP server with the SafeHarbor server as the handler.
	// See https://golang.org/pkg/net/http/#Server
	var httpServer *http.Server = &http.Server{
		Handler: server.getHandler(),
	}

	// Instantiate a TCP socker listener.
	fmt.Println("Creating socket listener")
	var tcpListener net.Listener
	tcpListener, err = newTCPListener(config.ipaddr, config.port)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1);
	}
	defer tcpListener.Close()
	
	
	
	//TEST
	// scope="repository:husseingalal/hello:push"
    // - match: {account: "admin", type: "myrepo", name="hello-world"}
    //    actions: ["push"]
	fmt.Println("Testing authorized")
	var service string = "Auth Service"
	var scope string = "hello-world:push"
	var account string = "admin"
	if server.authorized(service, scope, account) {
		fmt.Println("Authorized")
	} else {
		fmt.Println("Unauthorized")
	}
	
	
	//....To do: Install a ^C signal handler, to gracefully shut down.
	
	// Start listening for incoming HTTP requests.
	// Creates a new service goroutine for each incoming connection on tcpListener.
	// Each service goroutine reads requests and then calls httpServer.Handler
	// to reply to them. See https://golang.org/pkg/net/http/#Server.Serve
	fmt.Println("Starting service")
	if err := httpServer.Serve(tcpListener); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

/*******************************************************************************
 Read the configuration file and return a new Configuration struct.
 */
func getConfiguration() (*Configuration, error) {
	configurationPath := os.Getenv("SAFEHARBOR_CONFIGURATION_PATH")

	if configurationPath == "" {
		return nil, fmt.Errorf("configuration path unspecified")
	}

	var file *os.File
	var err error
	file, err = os.Open(configurationPath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	config, err := NewConfiguration(file)
	if err != nil {
		return nil, err
	}
	
	return config, nil
}

/*******************************************************************************
 * Return a new keep-alive TCP socker listener for the specified IP:port address.
 */
func newTCPListener(ipaddr, port string) (net.Listener, error) {
	tcpListener, err := net.Listen("tcp", fmt.Sprintf("%[1]s:%[2]s", ipaddr, port))
	if err != nil {
		return nil, err
	}

	return tcpKeepAliveListener{tcpListener.(*net.TCPListener)}, nil
}

/*******************************************************************************
 * 
 */
type tcpKeepAliveListener struct {
	*net.TCPListener
}


