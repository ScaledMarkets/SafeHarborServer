/*******************************************************************************
 * General purpose utility functions.
 */

package utils

import (
	//"fmt"
	"io/ioutil"
	"os"
	//"runtime/debug"
	
	// SafeHarbor packages:
)

/*******************************************************************************
 * 
 */
func MakeTempDir() (string, error) {
	
	return ioutil.TempDir("", "safeharbor_")
}

func MakeTempFile(dirpath, prefix string) (*os.File, error) {
	return ioutil.TempFile(dirpath, prefix)
}
