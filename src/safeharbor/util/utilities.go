/*******************************************************************************
 * General purpose utility functions.
 */

package util

import (
	"fmt"
	"errors"
	"runtime/debug"	
	
	// SafeHarbor packages:
)

func ConstructError(msg string) error {
	fmt.Println(msg)
	debug.PrintStack()
	return errors.New(msg)
}

/*******************************************************************************
 * 
 */
func PrintError(err error) error {
	fmt.Println(err.Error())
	debug.PrintStack()
	return err
}
