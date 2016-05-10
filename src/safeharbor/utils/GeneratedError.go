/*******************************************************************************
 * General purpose utility functions.
 */

package utils

import (
	"fmt"
	"errors"
	"runtime/debug"	
	
	// SafeHarbor packages:
)

type GeneratedError interface {
	error
}

type UserError interface {
	GeneratedError
}

type ServerError interface {
	GeneratedError
}

type GeneratedErrorStruct struct {
	error
}

func NewGeneratedError(msg string) *GeneratedErrorStruct {
	fmt.Println(msg)
	debug.PrintStack()
	return &GeneratedErrorStruct{
		error: errors.New(msg),
	}
}

type UserErrorStruct struct {
	GeneratedErrorStruct
}

type ServerErrorStruct struct {
	GeneratedErrorStruct
}

func IsUserErr(err error) bool {
	if err == nil { return false }
	var isType bool
	_, isType = err.(UserError)
	return isType
}

func IsServerErr(err error) bool {
	if err == nil { return false }
	var isType bool
	_, isType = err.(ServerError)
	return isType
}

func ConstructUserError(msg string) *UserErrorStruct {
	return &UserErrorStruct{
		GeneratedErrorStruct: *NewGeneratedError(msg),
	}
}

func ConstructServerError(msg string) *ServerErrorStruct {
	return &ServerErrorStruct{
		GeneratedErrorStruct: *NewGeneratedError(msg),
	}
}

func GenerateError(httpStatusCode int, httpStatus string) GeneratedError {
	
	if httpStatusCode >= 500 { return ConstructServerError(httpStatus) }
	if httpStatusCode >= 400 { return ConstructUserError(httpStatus) }
	if httpStatusCode >= 300 { return ConstructServerError(httpStatus) }
	return nil
}

/*******************************************************************************
 * 
 */
func PrintError(err error) error {
	fmt.Println(err.Error())
	debug.PrintStack()
	return err
}
