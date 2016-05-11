/*******************************************************************************
 * General purpose utility functions.
 */

package utils

import (
	"io/ioutil"
	
	// SafeHarbor packages:
)

/*******************************************************************************
 * 
 */
func MakeTempDir() (string, error) {
	
	return ioutil.TempDir("", "safeharbor_")
}
