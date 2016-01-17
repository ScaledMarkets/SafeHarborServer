package server


import (
	"testing"
	"fmt"
	"reflect"
	"strings"
	"errors"
	"time"
	"runtime/debug"
)

/*******************************************************************************
 * 
 */
func Test_JSONDeserialization(testContext *testing.T) {

	var json = "{\"abc\": 123, \"bs\": \"this_is_a_string\", " +
		"\"car\": [\"alpha\", \"beta\"], true}"
	var expected = []string{
		"{",
		"\"",
		"abc",
		"\"",
		":",
		"123",
		",",
		"\"",
		"bs",
		"\"",
		":",
		"\"",
		"this_is_a_string",
		"\"",
		",",
		"\"",
		"car",
		"\"",
		":",
		"[",
		"\"",
		"alpha",
		"\"",
		",",
		"\"",
		"beta",
		"\"",
		"]",
		",",
		"true",
		"}",
	}
	TryJsonDeserTokenizer(testContext, json, expected)
}
	
/*******************************************************************************
 * 
 */
func Test_JSONDeserialization(testContext *testing.T) {
	var json = "\"this is a string\""
	var expected = "this is a string"
	TryJsonDeserString(testContext, json, expected)
}
	
/*******************************************************************************
 * 
 */
func Test_JSONDeserialization(testContext *testing.T) {
	TryJsonDeserSimple(testContext)
}
	
/*******************************************************************************
 * 
 */
func Test_JSONDeserialization(testContext *testing.T) {
	TryJsonDeserNestedType(testContext)
}

/*******************************************************************************
 * 
 */
func TryJsonDeserTokenizer(testContext *testing.T, json string, expected []string) {

	var pos int = 0
	for i, expect := range expected {
		var token string = parseJSON_findNextToken(json, &pos)
		if ! AssertThat(testContext, token == expect,
			fmt.Sprintf("Token #%d, was %s, expected %s", (i+1), token, expect)) { break }
	}
}

/*******************************************************************************
 * 
 */
func TryJsonDeserString(testContext *testing.T, json, expected string) {
	
	var value reflect.Value
	var err error
	var pos int = 0
	value, err = parseJSON_string_value(json, &pos)
	AssertErrIsNil(testContext, err, "")
	AssertThat(testContext, value.IsValid(), "Value is not valid")
}

/*******************************************************************************
 * 
 */
func TryJsonDeserSimple(testContext *testing.T) {

	var client Client = &InMemClient{}
	var abc = &InMemABC{ 123, "this is a string", []string{"alpha", "beta"}, true }
	var jsonString = abc.toJSON()

	var retValue0 interface{}
	var err error
	retValue0, err = GetObject(client, jsonString)
	AssertErrIsNil(testContext, err, "on return from GetObject")
	
	var abc2 ABC
	var isType bool
	abc2, isType = retValue0.(ABC)
	if !isType { fmt.Println("abc2 is NOT an ABC") } else {
		fmt.Println("abc2 IS an ABC")
		fmt.Println(fmt.Sprintf("\tabc2.a=%d", abc2.getA()))
		fmt.Println("\tabc2.bs=" + abc2.getBs())
		//fmt.Println("\tabc2.Car=" + string(abc2.getCar()))
		//fmt.Println("\tabc2.db=" + string(abc2.getDb()))
	}
}

/*******************************************************************************
 * 
 */
func TryJsonDeserNestedType(testContext *testing.T) {

	var client Client = &InMemClient{}
	var def = &InMemDEF{
		ABC: client.NewABC(123, "this is a string", []string{"alpha", "beta"}, true),
		xyz: 456,
	}
	var jsonString = def.toJSON()

	var retValue0 interface{}
	var err error
	retValue0, err = GetObject(client, jsonString)
	AssertErrIsNil(testContext, err, "on return from GetObject")
	
	var def2 DEF
	var isType bool
	def2, isType = retValue0.(DEF)
	if !isType { fmt.Println("def2 is NOT an DEF") } else {
		fmt.Println("def2 IS a DEF")
		fmt.Println(fmt.Sprintf("\tdef2.a=%d", def2.getA()))
		fmt.Println("\tdef2.bs=" + def2.getBs())
		//fmt.Println("\tabc2.Car=" + string(abc2.getCar()))
		//fmt.Println("\tabc2.db=" + string(abc2.getDb()))
		fmt.Println(fmt.Sprintf("\tdef2.xyz=%d", def2.getXyz()))
	}
}

type Client interface {
	NewABC(a int, bs string, car []string, db bool) ABC
}

type ABC interface {
	getA() int
	getBs() string
	getCar() []string
	getDb() bool
	toJSON() string
}

type InMemClient struct {
}

type InMemABC struct {
	a int
	bs string
	car []string
	db bool
}

func (client *InMemClient) NewABC(a int, bs string, car []string, db bool) ABC {
	var abc *InMemABC = &InMemABC{a, bs, car, db}
	return abc
}

func (abc *InMemABC) getA() int {
	return abc.a
}

func (abc *InMemABC)  getBs() string {
	return abc.bs
}

func (abc *InMemABC)  getCar() []string {
	return abc.car
}

func (abc *InMemABC)  getDb() bool {
	return abc.db
}

func (abc *InMemABC) toJSON() string {
	var res = fmt.Sprintf("\"ABC\": {\"a\": %d, \"bs\": \"%s\", \"car\": [", abc.a, abc.bs)
		// Note - need to replace any quotes in abc.bs
	for i, s := range abc.car {
		if i > 0 { res = res + ", " }
		res = res + "\"" + s + "\""  // Note - need to replace any quotes in s
	}
	res = res + fmt.Sprintf("], \"db\": %s}", BoolToString(abc.db))
	return res
}

type DEF interface {
	ABC
	getXyz() int
}

type InMemDEF struct {
	ABC
	xyz int
}

func (client *InMemClient) NewDEF(a int, bs string, car []string, db bool, x int) DEF {
	var def = &InMemDEF{
		ABC: client.NewABC(a, bs, car, db),
		xyz: x,
	}
	return def
}

func (def *InMemDEF) getXyz() int {
	return def.xyz
}

func (def *InMemDEF) toJSON() string {
	var res = fmt.Sprintf("\"DEF\": {\"a\": %d, \"bs\": \"%s\", \"car\": [",
		def.getA(), def.getBs())
		// Note - need to replace any quotes in abc.bs
	for i, s := range def.getCar() {
		if i > 0 { res = res + ", " }
		res = res + "\"" + s + "\""  // Note - need to replace any quotes in s
	}
	res = res + fmt.Sprintf("], \"db\": %s, \"xyz\": %d}",
		BoolToString(def.getDb()), def.xyz)
	return res
}

/*******************************************************************************
 * 
 */
func FailTest(testContext *testing.T) {
	testContext.Fail()
	fmt.Println("Stack trace:")
	debug.PrintStack()
}

/*******************************************************************************
 * 
 */
func AbortAllTests(testContext *testing.T, msg string) {
	fmt.Println("Aborting tests: " + msg)
	os.Exit(1)
}

/*******************************************************************************
 * If the specified condition is not true, then print an error message.
 */
func AssertThat(testContext *testing.T, condition bool, msg string) bool {
	if ! condition {
		fmt.Println(fmt.Sprintf("ERROR: %s", msg))
		FailTest(testContext)
	}
	return condition
}

/*******************************************************************************
 * 
 */
func AssertErrIsNil(testContext *testing.T, err error, msg string) bool {
	if err == nil { return true }
	fmt.Println("Message:", msg)
	fmt.Println("Original error message:", err.Error())
	testContext.FailTest()
	return false
}
