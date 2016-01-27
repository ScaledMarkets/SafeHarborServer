/*******************************************************************************
 * Functions for reading JSON from the database and reconstituting objects in memory.
 */



package server

import (
	"fmt"
	"reflect"
	"strings"
	//"errors"
	"time"
	"runtime/debug"
	
	"safeharbor/rest"
	"safeharbor/util"
)

/*******************************************************************************
 * Retrieve the type name that precedes the JSON string. It is in the format,
 *	"type-name" : <json-string>
 * or, as BNF,
	line			::= '"' (no spaces) type_name (no spaces) '"'  ':'  string
	type_name		::= string
 * 
 * Returns the type name, and then the remainder of the string that follows the
 * first colon.
 */
func retrieveTypeName(json string) (typeName string, remainder string, err error) {
	
	var i = strings.Index(json, "\"")
	var s2 = json[i+1:]
	var j = strings.Index(s2, "\"")
	if j == -1 {
		debug.PrintStack()
		fmt.Println("json=" + json)
		return "", "", util.ConstructError(
		fmt.Sprintf("Ill-formatted json: no \" found after pos %d", i)) }
	var s3 = s2[:j]
	
	var k = strings.Index(s2[j:], ":")
	if k == -1 { return "", "", util.ConstructError(
		fmt.Sprintf("Ill-formatted json: no : found after position %d", j)) }
	
	return s3, s2[j+k+1:], nil
}

/*******************************************************************************
 * Parse each json field and return a Value for each.
 * Only built-in types are allowed in the JSON fields, including byte and time.Time.
 * Arrays of these are allowed as well. Recursive.
 * A nil result indicates that no object was found, but no syntax error either.
 * An empty array means that an object was found but it contained no fields.
 * Note: This function is not intended to be a general purpose JSON parser -
 * its use is limited to the needs of this application.
 * BNF of JSON syntax:
	obj_value		::= '{'  field  [ comma_field ]  '}'
	field			::=	'"' (no spaces) field_name (no spaces) '"'  ':'  value
	field_name		::= <char>+
	value			::= array_value | simple_value
	comma_field		::= ','  field  [ comma_field ]
	array_value		::= '['  [ value  [ comma_value ] ]  ']'
	comma_value		::= ','  value  [ comma_value ]
	simple_value	::= number | string_value | bool_value | time_value
	string_value	::= '"' <char>* '"'
	bool_value		::= 'true' | 'false'
	time_value		::= 'time' '"' <char_seq in time format> '"'
 */
func parseJSON(json string) ([]reflect.Value, error) {
	
	var pos int = 0
	return parseJSON_obj_value(json, &pos)
}

/*******************************************************************************
 * Parse a JSON object, delimited by { and }. If none found due to EOF, or if
 * the first token does not match the possible values for an obj_value, return nil.
 * Otherwise, return an array of the field values. If there is a syntax error,
 * return an error.
 */
func parseJSON_obj_value(json string, pos *int) ([]reflect.Value, error) {
	
	var token string = parseJSON_findNextToken(json, pos)
	if token == "" { return nil, nil }
	
	if token != "{" {
		parseJSON_pushTokenBack(token, pos)
		return nil, nil
	}
	
	var err error
	var values []reflect.Value = make([]reflect.Value, 0)
	var fieldValue reflect.Value
	_, fieldValue, err = parseJSON_field(json, pos)
	if err != nil { return values, err }
	if ! fieldValue.IsValid() { return values, nil } // no fields
	
	values = append(values, fieldValue)
	
	// Add additional fields, if any.
	var addlFieldValues []reflect.Value
	addlFieldValues, err = parseJSON_comma_field(json, pos)
	if err != nil { return values, err }
	if addlFieldValues != nil {
		values = append(values, addlFieldValues...)
	}
	
	token = parseJSON_findNextToken(json, pos)
	if token != "}" { return values, parseJSON_tokenError(token, json, pos,
		"while looking for object terminator") }
	
	return values, nil
}

func parseJSON_field(json string, pos *int) (string, reflect.Value, error) {
	
	var value reflect.Value

	var token = parseJSON_findNextToken(json, pos)
	if token == "" { return "", value, nil }
	
	if token != "\"" {
		parseJSON_pushTokenBack(token, pos)
		return "", value, nil
	}
	
	var fieldName string
	var err error
	fieldName, err = parseJSON_field_name(json, pos)
	if err != nil { return "", value, err }
	if fieldName == "" { return "", value, parseJSON_syntaxError(json, pos,
		"Did not find field name") }
	
	token = parseJSON_findNextToken(json, pos)
	if token != "\"" { return fieldName, value, parseJSON_tokenError(
		token, json, pos, "while looking for \" following a field name") }
	
	token = parseJSON_findNextToken(json, pos)
	if token != ":" { return fieldName, value, parseJSON_tokenError(
		token, json, pos, "while looking for colon following a field name") }
	
	value, err = parseJSON_value(json, pos)
	if err != nil { return fieldName, value, err }
	if ! value.IsValid() { return fieldName, value, parseJSON_tokenError(
		token, json, pos, "while looking for object field value") }
	
	return fieldName, value, nil
}

func parseJSON_comma_field(json string, pos *int) ([]reflect.Value, error) {
	
	var token = parseJSON_findNextToken(json, pos)
	if token == "" { return nil, nil }
	
	if token != "," {
		parseJSON_pushTokenBack(token, pos)
		return nil, nil
	}
	
	var err error
	var value reflect.Value
	var values []reflect.Value = make([]reflect.Value, 0)
	_, value, err = parseJSON_field(json, pos)
	if err != nil { return nil, err }
	values = append(values, value)
	
	var addlValues []reflect.Value
	addlValues, err = parseJSON_comma_field(json, pos)
	if err != nil { return values, err }
	if addlValues != nil {
		values = append(values, addlValues...)
	}
	
	return values, nil
}

func parseJSON_field_name(json string, pos *int) (string, error) {

	// Find trailing double-quote.
	var dblQuotePos = strings.Index(json[*pos:], "\"")
	if dblQuotePos == -1 { return "", util.ConstructError(
		fmt.Sprintf("Terminating double quote not found for field name, after pos %d", *pos)) }
	
	// ....to do: recognize escapes, etc.
	
	var startPos = *pos  // beginning of field name
	*pos += dblQuotePos  // update json pos to point to the trailing double-quote
	var fieldName = json[startPos:*pos]
	return fieldName, nil
}

func parseJSON_value(json string, pos *int) (reflect.Value, error) {
	
	var value reflect.Value
	var token = parseJSON_findNextToken(json, pos)
	if token == "" { return value, nil }
	
	if token == "[" {
		parseJSON_pushTokenBack(token, pos)
		return parseJSON_array_value(json, pos)
	} else {
		parseJSON_pushTokenBack(token, pos)
		return parseJSON_simple_value(json, pos)
	}
}

func parseJSON_array_value(json string, pos *int) (reflect.Value, error) {
	
	var value reflect.Value
	var token = parseJSON_findNextToken(json, pos)
	if token == "" { return value, nil }
	
	if token != "[" {
		parseJSON_pushTokenBack(token, pos)
		return value, nil
	}
	
	token = parseJSON_findNextToken(json, pos)
	if token == "]" { // no elements in array
		value = reflect.ValueOf(make([]interface{}, 0))
		return value, nil
	} else {
		parseJSON_pushTokenBack(token, pos)
	}
	
	var err error
	value, err = parseJSON_value(json, pos)
	if err != nil { return value, err }
	if ! value.IsValid() { return value, nil } // no fields
	
	// If there are more elements in the array, they must be of the same underlying
	// simple type as the above value.
	var elementType = value.Type()
	
	var sliceType = reflect.SliceOf(elementType)
	var slice = reflect.MakeSlice(sliceType, 0, 1)
	slice = reflect.Append(slice, value)
	
	var commaValues reflect.Value
	commaValues, err = parseJSON_comma_value(json, pos, elementType)
	if err != nil { return slice, err }
	if commaValues.IsValid() {
		slice = reflect.AppendSlice(slice, commaValues)
	}
	
	token = parseJSON_findNextToken(json, pos)
	if token != "]" { return slice, parseJSON_tokenError(token, json, pos,
		"while looking for array value") }
	
	return slice, nil
}

func parseJSON_simple_value(json string, pos *int) (reflect.Value, error ) {
	
	var value reflect.Value
	var err error
	value, err = parseJSON_number(json, pos)
	if err != nil { return value, err }
	if value.IsValid() { return value, nil }
	
	value, err = parseJSON_string_value(json, pos)
	if err != nil { return value, err }
	if value.IsValid() { return value, nil }
	
	value, err = parseJSON_bool_value(json, pos)
	if err != nil { return value, err }
	if value.IsValid() { return value, nil }
	
	value, err = parseJSON_time_value(json, pos)
	if err != nil { return value, err }
	if value.IsValid() { return value, nil }
	
	return value, parseJSON_syntaxError(json, pos,
		"Invalid value, '" + value.String() + "', while looking for simple value")
}

func parseJSON_comma_value(json string, pos *int,
	elementType reflect.Type) (reflect.Value, error) {
	
	var value reflect.Value
	var token = parseJSON_findNextToken(json, pos)
	if token == "" { return value, nil }
	
	if token != "," {
		parseJSON_pushTokenBack(token, pos)
		return value, nil
	}
	
	var err error
	value, err = parseJSON_value(json, pos)
	if err != nil { return value, err }
	
	var sliceType = reflect.SliceOf(elementType)
	var slice = reflect.MakeSlice(sliceType, 0, 1)
	slice = reflect.Append(slice, value)
	
	var commaValues reflect.Value
	commaValues, err = parseJSON_comma_value(json, pos, elementType)
	if err != nil { return slice, err }
	if commaValues.IsValid() {
		slice = reflect.AppendSlice(slice, commaValues)
	}
	
	return slice, nil
}

func parseJSON_number(json string, pos *int) (reflect.Value, error) {
	
	var value reflect.Value
	var token = parseJSON_findNextToken(json, pos)
	if token == "" { return value, nil }
	
	var number int
	_, err := fmt.Sscanf(token, "%d", &number)
	if err != nil {
		parseJSON_pushTokenBack(token, pos)
		return value, nil
	}
	return reflect.ValueOf(number), err
}

func parseJSON_string_value(json string, pos *int) (reflect.Value, error) {
	
	var value reflect.Value
	var token = parseJSON_findNextToken(json, pos)
	if token == "" { return value, nil }
	
	if token != "\"" {
		parseJSON_pushTokenBack(token, pos)
		return value, nil
	}
	
	// Find the stirng terminator - the next unescaped double quote.
	var posOfNextDblQuote int = -1
	var startPos = *pos
	for { // until we find an unescaped double quote
		if startPos >= len(json) { break }
		var p = strings.Index(json[startPos:], "\"")
		if p == -1 {  // no more double quotes in the json
			break
		} else {  // found a double quote - now see if it is escaped
			var decodedStr = rest.DecodeStringFromJSON(json[startPos:startPos+p+1])
			var p2 = strings.Index(decodedStr, "\"")
			if p2 == -1 {  // the double quote was escaped - skip past it
				startPos += (p+1)
				continue
			} else {  // really found a double-quote
				posOfNextDblQuote = startPos + p
				break
			}
		}
	}
	
	if posOfNextDblQuote == -1 { return value, parseJSON_syntaxError(json, pos,
		"While looking for a string value") }
	
	*pos = posOfNextDblQuote
	var strval = json[startPos: *pos]
	*pos++  // advance one past the trailing double quote
	
	
	strval = rest.DecodeStringFromJSON(strval)
	value = reflect.ValueOf(strval)
	return value, nil
}

func parseJSON_bool_value(json string, pos *int) (reflect.Value, error) {
	
	var value reflect.Value
	var token = parseJSON_findNextToken(json, pos)
	if token == "true" { return reflect.ValueOf(true), nil }
	if token == "false" { return reflect.ValueOf(false), nil }
	parseJSON_pushTokenBack(token, pos)
	return value, nil
}

func parseJSON_time_value(json string, pos *int) (reflect.Value, error) {
	
	var value reflect.Value
	var token = parseJSON_findNextToken(json, pos)
	if token != "time" {
		parseJSON_pushTokenBack(token, pos)
		return value, nil
	}
	
	var posOfFirstDblQuote = strings.Index(json[*pos:], "\"")  // relative to *pos
	if posOfFirstDblQuote == -1 { return value, parseJSON_syntaxError(json, pos,
		"While looking for a time value") }
	
	posOfFirstDblQuote += *pos
	
	var posOfSecondDblQuote = strings.Index(json[posOfFirstDblQuote+1:], "\"")
	if posOfSecondDblQuote == -1 { return value, parseJSON_syntaxError(json, pos,
		"While looking for a time value") }
	
	posOfSecondDblQuote += (posOfFirstDblQuote+1)
	
	var strval = json[posOfFirstDblQuote: posOfSecondDblQuote+1]
	
	var t time.Time
	var err error
	err = t.UnmarshalJSON([]byte(strval)) // scan the time value from strval
	if err != nil { return value, parseJSON_syntaxError(json, pos,
		"While looking for a time value") }
	value = reflect.ValueOf(t)

	*pos = posOfSecondDblQuote+1
	return value, nil
}

/*******************************************************************************
 * If EOF, return "". Update the position value to point to the start of the first
 * position following the token (which might be one past the end of the json, if
 * there is not more content in the string.
 */
func parseJSON_findNextToken(json string, pos *int) (token string) {

	var whitespace = " \t\r\n"
	var trimmedJson = strings.TrimLeft(json[*pos:], whitespace)
	if len(trimmedJson) == 0 {
		*pos = len(json)
		token = ""  // found no token
		return
	}
	
	*pos += (len(json[*pos:]) - len(trimmedJson))  // advance to start of token
	
	var specialJsonChars = "\":'[]{},"
	
	var posAfterToken int
	if strings.IndexAny(trimmedJson[:1], specialJsonChars) == 0 { // token is a special char
		posAfterToken = 1
	} else {
		posAfterToken = strings.IndexAny(trimmedJson[1:], whitespace + specialJsonChars)
		if posAfterToken == -1 {  // token goes through end of line
			posAfterToken = len(trimmedJson)
		} else {
			posAfterToken++  // account for fact that we started counting from position 1
		}
	}
	
	*pos += posAfterToken
	token = trimmedJson[:posAfterToken]
	return
}

func parseJSON_pushTokenBack(token string, pos *int) {
	*pos = *pos - len(token)
}

func parseJSON_syntaxError(json string, pos *int, msg string) error {
	var err = util.ConstructError(fmt.Sprintf("%s: at char no. %d, json=%s",
		msg, (*pos + 1), json))
	fmt.Println(err.Error())
	debug.PrintStack()
	return err
}

func parseJSON_tokenError(token string, json string, pos *int, msg string) error {
	parseJSON_pushTokenBack(token, pos)
	var err = util.ConstructError(fmt.Sprintf("Syntax error at char no. %d: %s %s, json=%s",
		(*pos + 1), token, msg, json))
	fmt.Println(err.Error())
	debug.PrintStack()
	return err
}
