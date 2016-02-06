/*******************************************************************************
 * The Persistence struct implements persistence, via redis, and defines the
 * in-memory cache of objects, realms, and users. Implementing these methods provides
 * persistence. If SafeHarbor is ever migrated to another database, only the
 * methods below should need to be re-implemented (in theory).
 * Redis bindings for go: http://redis.io/clients#go
 * Chosen binding: https://github.com/xuyu/goredis
 * Prior binding: https://github.com/alphazero/Go-Redis
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
 * Contains all of the state needed to interact with the persistent store (redis).
 */
type Persistence struct {
	Server *Server
	InMemoryOnly bool
	RedisClient *goredis.Redis
	uniqueId int64
	allObjects map[string]PersistObj
	allUsers map[string]User  // maps user id to user
	allRealmIds []string
}

func NewPersistence(server *Server, redisClient *goredis.Redis) (*Persistence, error) {
	var persist = &Persistence{
		Server: server,
		InMemoryOnly: server.InMemoryOnly,
		RedisClient: redisClient,
	}
	persist.resetInMemoryState()
	
	var err error = persist.init()
	if err != nil { return nil, err }
	
	return persist, nil
}

/*******************************************************************************
 * Delete all persistent data - but do not delete data that is in another repository
 * such as a docker registry.
 */
func (persist *Persistence) resetPersistentState() error {
	
	// Remove the file repository.
	fmt.Println("Removing all files at " + persist.Server.Config.FileRepoRootPath)
	var err error
	err = os.RemoveAll(persist.Server.Config.FileRepoRootPath)
	if err != nil { return err }
	
	// Recreate the file repository, but empty.
	os.Mkdir(persist.Server.Config.FileRepoRootPath, 0770)

	fmt.Println("Repository initialized")
	return nil
}

/*******************************************************************************
 * 
 */
func (persist *Persistence) GetUserObjByUserId(userId string) (User, error) {
....don''t we need a reference to the transaction?
	var user = persist.allUsers[userId]
	if user == nil {
		var userObjId string
		var err error
		userObjId, err = persist.RedisClient.HGet("users", userId)
		if err != nil { return err }
		if userObjId == "" {
			return nil, nil
		}
	}
	return userObjId, nil
}

/*******************************************************************************
 * Create a directory for the Dockerfiles, images, and any other files owned
 * by the specified realm.
 */
func (persist *Persistence) assignRealmFileDir(realmId string) (string, error) {
....don''t we need a reference to the transaction?
	var path = persist.Server.Config.FileRepoRootPath + "/" + realmId
	// Create the directory. (It is an error if it already exists.)
	err := os.MkdirAll(path, 0711)
	return path, err
}

/*******************************************************************************
 * Create a directory for the Dockerfiles, images, and any other files owned
 * by the specified repo. The directory will be created as a subdirectory of the
 * realm''s directory.
 */
func (persist *Persistence) assignRepoFileDir(realmId string, repoId string) (string, error) {
....don''t we need a reference to the transaction?
	fmt.Println("assignRepoFileDir(", realmId, ",", repoId, ")...")
	var err error
	var realm Realm
	realm, err = persist.getRealm(realmId)
	if err != nil { return "", err }
	....var path = realm.getFileDirectory() + "/" + repoId
	var curdir string
	curdir, err = os.Getwd()
	if err != nil { fmt.Println(err.Error()) }
	fmt.Println("Current directory is '" + curdir + "'")
	fmt.Println("Creating directory '" + path + "'...")
	err = os.MkdirAll(path, 0711)
	return path, err
}

/*******************************************************************************
 * Print the database to stdout. Diagnostic.
 */
func (persist *Persistence) printDatabase() {
	fmt.Println("Not implemented yet")
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
 * Write an object to the database - making the object persistent.
 */
func (persist *Persistence) addObject(obj PersistObj) error {
....don''t we need a reference to the transaction?
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
....don''t we need a reference to the transaction?
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
....don''t we need a reference to the transaction?
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
func (persist *Persistence) addRealm(newRealm Realm) error {....don''t we need a reference to the transaction?
	if persist.InMemoryOnly {
		persist.allRealmIds = append(persist.allRealmIds, newRealm.getId())
		return persist.addObject(newRealm)
	} else {
		var err = persist.addObject(newRealm)
		if err != nil { return err }
		var numAdded int64
		....numAdded, err = persist.RedisClient.SAdd("realms", newRealm.getId())
		if err != nil { return err }
		if numAdded == 0 { return util.ConstructError("Unable to add realm " + newRealm.getName()) }
		persist.allRealmIds = append(persist.allRealmIds, newRealm.getId())
		return nil
	}
}

/*******************************************************************************
 * Return a list of the Ids of all of the realms in the database.
 */
func (persist *Persistence) dbGetAllRealmIds() ([]string, error) {....don''t we need a reference to the transaction?
	if persist.InMemoryOnly {
		return persist.allRealmIds, nil
	} else {
		var members []string
		var err error
		....members, err = persist.RedisClient.SMembers("realms")
		if err != nil { return nil, err }
		return members, nil
	}
}

/*******************************************************************************
 * Insert a new User into the databse. This automatically inserts the
 * underlying persistent object.
 */
func (persist *Persistence) addUser(user User) error {....don''t we need a reference to the transaction?
	if persist.InMemoryOnly {
		persist.allUsers[user.getUserId()] = user
		return persist.addObject(user)
	} else {
		var err = persist.addObject(user)
		if err != nil { return err }
		
		// Check if the user already exists in the set.
		var userObjId string
		userObjId, err = persist.GetUserObjByUserId(user.getUserId())
		if err != nil { return err }
		if userObjId != "" {
			return util.ConstructError("User '" + user.getName() + "' is already a member of the set of users")
		}
		
		// Write user to user-id hash.
		var added bool
		added, err = persist.RedisClient.HSet("users", user.getUserId(), user.getId())
		if err != nil { return err }
		if ! added { return util.ConstructError("Unable to add user " + user.getName()) }
		
		// Write user object to database.
		err = persist.addObject(user)
		if err != nil { return err }
		
		return nil
	}
}



/*******************************************************************************
								Internal methods
*******************************************************************************/



/*******************************************************************************
 * Initilize the client object. This can be called later to reset the client''s
 * state (i.e., to erase all objects).
 */
func (persist *Persistence) init() error {
	
	persist.resetInMemoryState()
	var err error = persist.loadCoreData()
	if err != nil { return util.ConstructError("Unable to load database state: " + err.Error()) }
	
	if client.Server.Debug { createTestObjects() }
	
	return nil
}

/*******************************************************************************
 * Load core database state.
 * If the data is not present in the database, it should be created and written out.
 */
func (persist *Persistence) loadCoreData() error {
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
 * Initialize the in-memory state of the database. This is normally called on
 * startup, of if the database connection must be re-established. Persistent
 * state is not modified.
 */
func (persist *Persistence) resetInMemoryState() {
	persist.uniqueId = 100000005
	persist.allRealmIds = make([]string, 0)
	persist.allObjects = make(map[string]PersistObj)
	persist.allUsers = make(map[string]User)
}

/*******************************************************************************
 * For test mode only.
 */
func createTestObjects() {
	fmt.Println("Debug mode: creating realm testrealm")
	var realmInfo *apitypes.RealmInfo
	realmInfo, err = apitypes.NewRealmInfo("testrealm", "Test Org", "For Testing")
	if err != nil {
		fmt.Println(err.Error())
		panic(err)
	}
	var testRealm Realm
	testRealm, err = client.dbCreateRealm(realmInfo, "testuser1")
	if err != nil {
		fmt.Println(err.Error())
		panic(err)
	}
	fmt.Println("Debug mode: creating user testuser1 in realm testrealm")
	var testUser1 User
	testUser1, err = client.dbCreateUser("testuser1", "Test User", 
		"testuser@gmail.com", "Password1", testRealm.getId())
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1);
	}
	fmt.Println("User", testUser1.getName())
	fmt.Println("created user, obj id=" + testUser1.getId())
	fmt.Println("Giving user admin access to the realm.")
	_, err = client.setAccess(testRealm, testUser1, []bool{true, true, true, true, true})
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1);
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
