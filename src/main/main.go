/*******************************************************************************
 * SafeHarbor REST server.
 * See https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
*/

package main

import (
	"fmt"
)

func main() {
	
	fmt.Println("Creating SafeHarbor server...")
	NewServer()
	fmt.Println("...started. Press ^C to terminate.")
}
