/*******************************************************************************
 * Functions needed to implement the handlers in Handlers.go.
 */

package main

import (
	"mime/multipart"
	"fmt"
	"errors"
	"os"
	"strconv"
)

/*******************************************************************************
 * Create a filename that is unique within the specified directory. Derive the
 * file name from the specified base name.
 */
func createUniqueFilename(dir string, basename string) (string, error) {
	var filepath = dir + "/" + basename
	for i := 0; i < 1000; i++ {
		var p string = filepath + strconv.FormatInt(int64(i), 10)
		if ! fileExists(p) {
			return p, nil
		}
	}
	return "", errors.New("Unable to create unique file name in directory " + dir)
}

/*******************************************************************************
 * 
 */
func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		fmt.Println("fileExists:")
		fmt.Println("\t", path)
	} else {
		fmt.Println("fileExists - NOT:")
		fmt.Println("\t", path)
		fmt.Println(err.Error())
	}
	return (err == nil)
}

/*******************************************************************************
 * Write the specified map to stdout. This is a diagnostic method.
 */
func printMap(m map[string][]string) {
	fmt.Println("Map:")
	for k, v := range m {
		fmt.Println(k, ":")
		for i := range v {
			fmt.Println("\t", v[i])
		}
	}
}

/*******************************************************************************
 * Write the specified map to stdout. This is a diagnostic method.
 */
func printFileMap(m map[string][]*multipart.FileHeader) {
	fmt.Println("FileHeader Map:")
	for k, headers := range m {
		fmt.Println("Name:", k, "FileHeaders:")
		for i := range headers {
			fmt.Println("Filename:", headers[i].Filename)
			printMap(headers[i].Header)
			fmt.Println()
		}
	}
}
