/*******************************************************************************
 * General purpose utility functions.
 */

package utils

/*******************************************************************************
 * 
 */
func Contains(value string, originalList []string) bool {
	for _, s := range originalList {
		if s == value { return true }
	}
	return false
}

/*******************************************************************************
 * 
 */
func AddUniquely(value string, originalList []string) []string {
	if Contains(value, originalList) { return originalList }
	return append(originalList, value)
}

/*******************************************************************************
 * Utility to remove a value from an array of strings. It is assumed that the
 * value is not present in the array more than one time.
 */
func RemoveFrom(value string, originalList []string) []string {
	var newList []string = make([]string, len(originalList))
	copy(newList, originalList)
	for index, s := range originalList {
		if s == value {
			newList = RemoveAt(index, newList)
			return newList
		}
	}
	return newList
}


/*******************************************************************************
 * Utility to remove a value from a specified location in an array of strings.
 */
func RemoveAt(position int, originalList []string) []string {
	var firstPart []string = []string{}
	if position > 0 {
		firstPart = append(firstPart, originalList[0:position]...)
	}
	if position >= (len(originalList)-1) { // nothing to append
		return firstPart
	} else {
		return append(firstPart, originalList[position+1:]...)
	}
}
