/*******************************************************************************
 * General purpose utility functions.
 */

package utils

import (
	"fmt"
	"io/ioutil"
	"runtime/debug"
	
	// SafeHarbor packages:
)

/*******************************************************************************
 * 
 */
func MakeTempDir() (string, error) {
	
	fmt.Println("MakeTempDir----------------------")
	debug.PrintStack()
	return ioutil.TempDir("", "safeharbor_")
}
