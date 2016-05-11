/*******************************************************************************
 * General purpose utility functions.
 */

package utils

import (
	"io/ioutil"
	"runtime/debug"
	
	// SafeHarbor packages:
)

/*******************************************************************************
 * 
 */
func MakeTempDir() (string, error) {
	
	debug.PrintStack()
	return ioutil.TempDir("", "safeharbor_")
}
