/*******************************************************************************
 * The Persistence struct implements persistence. It is extended by the Client struct,
 * in InMemory.go, which implements the Client interface from DBClient.go. Below that,
 * the remaining types (structs) implement the various persistent object types
 * from DBClient.go.
 */

package server

import (
	"fmt"
	"sync/atomic"
	//"errors"
	"strconv"
	"reflect"
	//"os"
	//"time"
	//"runtime/debug"	
	
	"goredis"
	
	"safeharbor/apitypes"
	//"safeharbor/docker"
	"safeharbor/util"
)

/*******************************************************************************
 * Implements DataError.
 */
type PersistDataError struct {
	error
}

var _ DataError = &PersistDataError{}

func NewPersistDataError(msg string) *PersistDataError {
	return &PersistDataError{
		error: util.ConstructError(msg),
	}
}

func (dataErr *PersistDataError) asFailureDesc() *apitypes.FailureDesc {
	return apitypes.NewFailureDesc(dataErr.Error())
}

/*******************************************************************************
 * Contains all persistence functionality. Implementing these methods provides
 * persistence.
 *
 * Redis bindings for go: http://redis.io/clients#go
 * Chosen binding: https://github.com/xuyu/goredis
 * Prior binding: https://github.com/alphazero/Go-Redis
 */
type Persistence struct {
	InMemoryOnly bool
	RedisClient *goredis.Redis
	uniqueId int64
	allObjects map[string]PersistObj
	allUsers map[string]User  // maps user id to user
	allRealmIds []string
}

func NewPersistence(inMemoryOnly bool, redisClient *goredis.Redis) (*Persistence, error) {
	var persist = &Persistence{
		InMemoryOnly: inMemoryOnly,
		RedisClient: redisClient,
	}
	persist.resetInMemory()
	return persist, nil
}

/*******************************************************************************
 * Initialize the in-memory state of the database. This is normally called on
 * startup, of if the database connection must be re-established. Persistent
 * state is not modified.
 */
func (persist *Persistence) resetInMemory() {
	persist.uniqueId = 100000005
	persist.allRealmIds = make([]string, 0)
	persist.allObjects = make(map[string]PersistObj)
	persist.allUsers = make(map[string]User)
}

/*******************************************************************************
 * Load core database state. Database data is not cached, except for this core data.
 * If the data is not present in the database, it should be created and written out.
 */
func (persist *Persistence) load() error {
	fmt.Println("Loading core database state...")
	var id int64
	var err error
	id, err = persist.readUniqueId()  // returns 0 if database is "virgin"
	if err != nil { return err }
	if id == 0 {
		err = persist.RedisClient.Set(
			"UniqueId", fmt.Sprintf("%d", persist.uniqueId), 0, 0, false, false)
		if err != nil { return err }
	} else {
		persist.uniqueId = id
	}
	
	fmt.Println("...completing loading database state.")
	return nil
}

/*******************************************************************************
 * Create a globally unique id, to be used to uniquely identify a new persistent
 * object. The creation of the id must be done atomically.
 */
func (persist *Persistence) createUniqueDbObjectId() (string, error) {
	
	var id int64
	if persist.InMemoryOnly {
		id = atomic.AddInt64(&persist.uniqueId, 1)
	} else {
		var err error
		id, err = persist.RedisClient.Incr("UniqueId")
		if err != nil { return "", err }
	}
	
	persist.uniqueId = id
	return fmt.Sprintf("%d", id), nil
}

/*******************************************************************************
 * Return the current value of the unique object Id generator.
 */
func (persist *Persistence) readUniqueId() (int64, error) {
	if persist.InMemoryOnly {
		return persist.uniqueId, nil
	} else {
		var bytes []byte
		var err error
		bytes, err = persist.RedisClient.Get("UniqueId")
		if err != nil { return 0, err }
		var str = string(bytes)
		if str == "" { return 0, nil }
		var id int64
		id, err = strconv.ParseInt(str, 10, 64)
		if err != nil { return 0, err }
		return id, nil
	}
}

/*******************************************************************************
 * Obtain a lock on the specified object, blocking until either the lock is
 * obtained, or until the timeout period elapses. If the latter occurs, return
 * an error. If the current thread already has a lock on the specified object,
 * merely return.
 *
 * Possible algorithm:
	getLock(obj, timeLimit) {
		elapsedTime = 0
		startTime = GetCurTime()
		for {
			old_pid1 = obj.getPid()
			old_pid2 = obj.getsetPid(my_pid)	// Try to get the lock.
			if old_pid2 != old_pid1 {
												// did not get it
				obj.setPid(old_pid2)
				continue
			}	
			if obj.getPid() == my_pid			// See if we got the lock.
				return success
			elapsedTime = GetCurTime() - startTime
			if elapsedTime + 100ms > timeLimit
				return error
			wait(100ms)
		}
	}
 */
func (persist *Persistence) waitForLockOnObject(obj PersistObj, timeoutSeconds int) error {
	if persist.InMemoryOnly {
		return nil
	} else {
		return nil  //....
	}
}

/*******************************************************************************
 * Release any and all locks on the specified object. If there are no locks on
 * the object, merely return.
 */
func (persist *Persistence) releaseLock(obj PersistObj) {
	if persist.InMemoryOnly {
	} else {
		//....
	}
}

/*******************************************************************************
 * Write an object to the database - making the object persistent.
 */
func (persist *Persistence) addObject(obj PersistObj) error {
	if persist.InMemoryOnly {
		persist.allObjects[obj.getId()] = obj
	} else {
		// Update cache.
		persist.allObjects[obj.getId()] = obj
		
		// Serialize (marshall) the object to JSON, and store it in redis using the
		// object's Id as the key. When the object is written out, it will be
		// written as,
		//    "<typename>": { <object fields> }
		// so that getPersistentObject will later be able to map the JSON to the
		// appropriate go type, using reflection.
		var err = persist.RedisClient.Set(
			"obj/" + obj.getId(), obj.asJSON(), 0, 0, false, false)
		if err != nil { return err }
	}
	return nil
}

/*******************************************************************************
 * Remove the specified object from the database. After this, the object is no
 * longer persistent.
 */
func (persist *Persistence) deleteObject(obj PersistObj) error {
	if persist.InMemoryOnly {
		persist.allObjects[obj.getId()] = nil
		return nil
	} else {
		var numDeleted int64
		var err error
		numDeleted, err = persist.RedisClient.Del("obj/" + obj.getId())
		if err != nil { return err }
		if numDeleted == 0 { return util.ConstructError("Unable to delete object with Id " + obj.getId()) }
		persist.allObjects[obj.getId()] = nil
		return nil
	}
}

/*******************************************************************************
 * Return the persistent object that is identified by the specified unique id.
 * An object''s Id is assigned to it by the function that creates the object.
 * The factory is the object that has the Reconstitute methods needed to
 * construct the persistent object.
 */
func (persist *Persistence) getObject(factory interface{}, id string) (PersistObj, error) {

	if persist.InMemoryOnly {
		return persist.allObjects[id], nil
	} else {
		
		// First see if we have it in memory.
		if persist.allObjects[id] != nil { return persist.allObjects[id], nil }
		
		// Read JSON from the database, using the id as the key; then deserialize
		// (unmarshall) the JSON into an object. The outermost JSON object will be
		// a field name - that field name is the name of the go object type; reflection
		// will be used to identify the go type, and set the fields in the type using
		// values from the hashmap that is built by the unmarshalling.
		
		var bytes []byte
		var err error
		bytes, err = persist.RedisClient.Get("obj/" + id)
		if err != nil { return nil, err }
		if bytes == nil { return nil, nil }
		if len(bytes) == 0 { return nil, nil }
		
		var obj interface{}
		_, obj, err = ReconstituteObject(factory, string(bytes))
		if err != nil { return nil, err }
		
		var persistObj PersistObj
		var isType bool
		persistObj, isType = obj.(PersistObj)
		if ! isType { return nil, util.ConstructError("Object is not a PersistObj") }
		
		// Add to in-memory cache.
		persist.allObjects[id] = persistObj;
		
		return persistObj, nil
	}
}

/*******************************************************************************
 * Insert a new Realm into the database. This automatically inserts the
 * underlying persistent object.
 */
func (persist *Persistence) addRealm(newRealm Realm) error {
	if persist.InMemoryOnly {
		persist.allRealmIds = append(persist.allRealmIds, newRealm.getId())
		return persist.addObject(newRealm)
	} else {
		var err = persist.addObject(newRealm)
		if err != nil { return err }
		var numAdded int64
		numAdded, err = persist.RedisClient.SAdd("realms", newRealm.getId())
		if err != nil { return err }
		if numAdded == 0 { return util.ConstructError("Unable to add realm " + newRealm.getName()) }
		persist.allRealmIds = append(persist.allRealmIds, newRealm.getId())
		return nil
	}
}

/*******************************************************************************
 * Return a list of the Ids of all of the realms in the database.
 */
func (persist *Persistence) dbGetAllRealmIds() ([]string, error) {
	if persist.InMemoryOnly {
		return persist.allRealmIds, nil
	} else {
		var members []string
		var err error
		members, err = persist.RedisClient.SMembers("realms")
		if err != nil { return nil, err }
		return members, nil
	}
}

/*******************************************************************************
 * Insert a new User into the databse. This automatically inserts the
 * underlying persistent object.
 */
func (persist *Persistence) addUser(user User) error {
	if persist.InMemoryOnly {
		persist.allUsers[user.getUserId()] = user
		return persist.addObject(user)
	} else {
		var err = persist.addObject(user)
		if err != nil { return err }
		
		// Check if the user already exists in the set.
		var isMem bool
		isMem, err = persist.RedisClient.SIsMember("users", user.getId())
		if isMem {
			return util.ConstructError("User '" + user.getName() + "' is already a member of the set of users")
		}
		
		var numAdded int64
		numAdded, err = persist.RedisClient.SAdd("users", user.getId())
		if err != nil { return err }
		if numAdded == 0 { return util.ConstructError("Unable to add user " + user.getName()) }
		persist.allUsers[user.getUserId()] = user
		return nil
	}
}

/*******************************************************************************
 * Construct an object as defined by the specified JSON string. Returns the
 * name of the object type and the object, or an error. The factory has
 * a ReconstituteXYZ method for constructing the object.
 */
func ReconstituteObject(factory interface{}, json string) (string, interface{}, error) {
	
	var typeName string
	var remainder string
	var err error
	typeName, remainder, err = retrieveTypeName(json)
	if err != nil { return typeName, nil, err }
	
	var methodName = "Reconstitute" + typeName
	var method = reflect.ValueOf(factory).MethodByName(methodName)
	if err != nil { return typeName, nil, err }
	if ! method.IsValid() { return typeName, nil, util.ConstructError(
		"Method " + methodName + " is unknown") }
	
	var actArgAr []reflect.Value
	actArgAr, err = parseJSON(remainder)
	if err != nil { return typeName, nil, err }

	var methodType reflect.Type = method.Type()
	var noOfFormalArgs int = methodType.NumIn()
	if noOfFormalArgs != len(actArgAr) {
		return typeName, nil, util.ConstructError(fmt.Sprintf(
			"Number of actual args (%d) does not match number of formal args (%d)",
			len(actArgAr), noOfFormalArgs))
	}
	
	// Check that argument types of the actuals match the types of the formals.
	var actArgArCopy = make([]reflect.Value, len(actArgAr))
	copy(actArgArCopy, actArgAr) // make shallow copy of actArgAr
	for i, actArg := range actArgArCopy {
		if ! actArg.IsValid() { fmt.Println(fmt.Sprintf("\targ %d is a zero value", i)) }
		
		// Problem: Empty JSON lists were created as []interface{}. However, if the
		// formal arg type is more specialized, e.g., []string, then the call
		// via method.Call(args) will fail. Therefore, if an actual arg is an empty
		// list, we need to replace it with an actual that is a list of the
		// type required by the formal arg. Also, some types, e.g., []int, must
		// be converted to the required formal type, e.g., []uint8.
		var argKind = actArg.Type().Kind()
		if (argKind == reflect.Array) || (argKind == reflect.Slice) {
			// Replace actArg with an array of the formal type.
			var replacementArrayValue = reflect.Indirect(reflect.New(methodType.In(i)))
			actArgAr[i] = replacementArrayValue
			
			if actArg.Len() > 0 {
				actArgAr[i] = reflect.MakeSlice(methodType.In(i), actArg.Len(), actArg.Len())
			}
			actArg = actArgAr[i]
			//reflect.Copy(
			for j := 0; j < actArg.Len(); j++ {
				actArg.Index(j).Set(actArg.Index(j).Convert(methodType.In(i).Elem()))
			}
		}
		
		// Check that arg types match.
		if ! actArg.Type().AssignableTo(methodType.In(i)) {
			return typeName, nil, util.ConstructError(fmt.Sprintf(
				"For argument #%d, type of actual arg, %s, " +
				"is not assignable to the required type, %s. JSON=%s",
				(i+1), actArg.Type().String(), methodType.In(i).String(), json))
		}
	}
	
	var retValues []reflect.Value = method.Call(actArgAr)
	var retValue0 interface{} = retValues[0].Interface()
	return typeName, retValue0, nil
}
