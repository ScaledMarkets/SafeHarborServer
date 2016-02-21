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
	"errors"
	"strconv"
	"reflect"
	"os"
	//"time"
	"runtime/debug"	
	
	"goredis"
	
	//"safeharbor/apitypes"
	//"safeharbor/docker"
	"safeharbor/util"
)

const (
	ObjectIdPrefix = "obj/"
	RealmHashName = "realms"
	UserHashName = "users"
)

/*******************************************************************************
 * Contains all of the state needed to interact with the persistent store (redis).
 */
type Persistence struct {
	Server *Server
	InMemoryOnly bool
	RedisClient *goredis.Redis
	uniqueId int64
	
	
	// Only use this for in-memory only testing.
	allObjects map[string]PersistObj  // maps object id to PersistObj
	allUserIds map[string]string  // maps user id to User obj Id
	realmMap map[string]string  // maps realm name to Realm obj Id
}

func NewPersistence(server *Server, redisClient *goredis.Redis) (*Persistence, error) {
	var persist = &Persistence{
		Server: server,
		InMemoryOnly: server.InMemoryOnly,
		RedisClient: redisClient,
	}
	server.persistence = persist
	persist.resetInMemoryState()
	
	var err error = persist.init()
	if err != nil { return nil, err }
	
	return persist, nil
}

type GoRedisTransactionWrapper struct {
	Persistence *Persistence
	GoRedisTransaction *goredis.Transaction
}

func (txn *GoRedisTransactionWrapper) commit() error {
	var err error
	var t *goredis.Transaction = getRedisTransaction(txn)
	_, err = t.Exec()
	t.Close()
	
	if txn.Persistence.Server.NoCache {
		txn.Persistence.clearCache()
	}
	
	return err
}

func (txn *GoRedisTransactionWrapper) abort() error {
	var err error
	var t *goredis.Transaction = getRedisTransaction(txn)
	err = t.Discard()
	t.Close()
	return err
}

func (persist *Persistence) NewTxnContext() (TxnContext, error) {
	var goRedisTxn *goredis.Transaction
	var err error
	
	
	//if persist == nil { return nil, errors.New("Unexpected") }
	if persist.RedisClient == nil { return nil, util.ConstructError("Redis not configured") }
	goRedisTxn, err = persist.RedisClient.Transaction()
	if err != nil { return nil, err }
	return &GoRedisTransactionWrapper{
		Persistence: persist,
		GoRedisTransaction: goRedisTxn,
	}, nil
}

/*******************************************************************************
 * Delete all persistent data - but do not delete data that is in another repository
 * such as a docker registry.
 */
func (persist *Persistence) resetPersistentState() error {
	
	fmt.Println()
	fmt.Println("---------------RESET PERSISTENT STATE-----------------")
	debug.PrintStack()
	fmt.Println()
	
	// Remove the file repository.
	fmt.Println("Removing all files at " + persist.Server.Config.FileRepoRootPath)
	var err error
	err = os.RemoveAll(persist.Server.Config.FileRepoRootPath)
	if err != nil { return err }
	
	// Recreate the file repository, but empty.
	os.Mkdir(persist.Server.Config.FileRepoRootPath, 0770)
	
	// Clear redis.
	err = persist.RedisClient.FlushAll()
	if err != nil { return err }

	fmt.Println("Repository initialized")
	return nil
}

/*******************************************************************************
 * Note: We assume that a user''s user-id is not changed once it has been set.
 */
func (persist *Persistence) GetUserObjIdByUserId(txn TxnContext, userId string) (string, error) {

	if persist.InMemoryOnly {
		return persist.allUserIds[userId], nil
	} else {
		var err error
		var bytes []byte
		bytes, err = persist.RedisClient.HGet(UserHashName, userId)
		if err != nil { return "", err }
		if (bytes == nil) || (len(bytes) == 0) { return "", nil }
		var userObjId = string(bytes)
		return userObjId, nil
	}
}

/*******************************************************************************
 * 
 */
func (persist *Persistence) setUserId(txn TxnContext, userId string) error {
	return errors.New("Cannot change a user id")
}

/*******************************************************************************
 * 
 */
func (persist *Persistence) setRealmName(txn TxnContext, name string) error {
	return errors.New("Cannot change a realm name")
}

/*******************************************************************************
 * Note: We assume that a realm''s name is not changed once it has been set.
 */
func (persist *Persistence) GetRealmObjIdByRealmName(txn TxnContext, realmName string) (string, error) {

	if persist.InMemoryOnly {
		return persist.realmMap[realmName], nil
	} else {
		var realmObjId string
		var bytes []byte
		var err error
		bytes, err = persist.RedisClient.HGet(RealmHashName, realmName)
		if err != nil { debug.PrintStack() }
		if err != nil { return "", err }
		if (bytes == nil) || (len(bytes) == 0) { return "", nil }
		realmObjId = string(bytes)
		return realmObjId, nil
	}
}

/*******************************************************************************
 * Create a directory for the Dockerfiles, images, and any other files owned
 * by the specified realm.
 */
func (persist *Persistence) assignRealmFileDir(txn TxnContext, realmId string) (string, error) {

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
func (persist *Persistence) assignRepoFileDir(txn TxnContext, realm Realm, repoId string) (string, error) {

	var err error
	var path = realm.getFileDirectory() + "/" + repoId
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
 * If the object is already in the database, create it.
 */
func (persist *Persistence) updateObject(txn TxnContext, obj PersistObj) error {

	if persist.InMemoryOnly {
		persist.allObjects[obj.getId()] = obj
	} else {
		// Verify that object does not exist in the database.
		var exists bool
		var err error
		exists, err = persist.RedisClient.Exists(ObjectIdPrefix + obj.getId())
		if err != nil { return err }
		if exists {
			return errors.New("Object with Id " + obj.getId() + " already exists")
		}
		
		// Serialize (marshall) the object to JSON, and store it in redis using the
		// object's Id as the key. When the object is written out, it will be
		// written as,
		//    "<typename>": { <object fields> }
		// so that getPersistentObject will later be able to map the JSON to the
		// appropriate go type, using reflection.
		
		err = getRedisTransaction(txn).Command("SET",
			ObjectIdPrefix + obj.getId(), obj.asJSON())
		if err != nil { debug.PrintStack() }
		if err != nil { return err }
	}
	return nil
}

/*******************************************************************************
 * Remove the specified object from the database. After this, the object is no
 * longer persistent.
 */
func (persist *Persistence) deleteObject(txn TxnContext, obj PersistObj) error {

	if persist.InMemoryOnly {
		persist.allObjects[obj.getId()] = nil
	} else {
		var err error
		err = getRedisTransaction(txn).Command("DEL", ObjectIdPrefix + obj.getId())
		if err != nil { return err }
		persist.allObjects[obj.getId()] = nil
	}
	return nil
}

/*******************************************************************************
 * Return the persistent object that is identified by the specified unique id.
 * An object''s Id is assigned to it by the function that creates the object.
 * The factory is the object that has the Reconstitute methods needed to
 * construct the persistent object.
 */
func (persist *Persistence) getObject(txn TxnContext, factory interface{}, id string) (PersistObj, error) {

	if persist.InMemoryOnly {
		return persist.allObjects[id], nil
	} else {
		
		// Set a watch on the object so that if it changes, the transaction will
		// fail. Note that we cannot retrieve the value as part of the transaction,
		// and so we are relying on the fact that the watch is set before we read
		// the value.
		var err error
		err = getRedisTransaction(txn).Watch(ObjectIdPrefix + id)
		if err != nil { debug.PrintStack() }
		if err != nil { return nil, err }
		
		// Read JSON from the database, using the id as the key; then deserialize
		// (unmarshall) the JSON into an object. The outermost JSON object will be
		// a field name - that field name is the name of the go object type; reflection
		// will be used to identify the go type, and set the fields in the type using
		// values from the hashmap that is built by the unmarshalling.
		
		var bytes []byte
		
		// Read the value of the object from the database. This is done outside
		// of the transaction, because Redis does not allow one to read a value
		// as part of a transaction and then act on that value within the
		// transaction.
		bytes, err = persist.RedisClient.Get(ObjectIdPrefix + id)
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
		
		return persistObj, nil
	}
}

/*******************************************************************************
 * Insert a new Realm into the database. This automatically inserts the
 * underlying persistent object.
 */
func (persist *Persistence) addRealm(txn TxnContext, newRealm Realm) error {
	if persist.InMemoryOnly {
		persist.realmMap[newRealm.getName()] = newRealm.getId()
		return persist.updateObject(txn, newRealm)
	} else {
		// Check if the realm already exists in the hash.
		var realmObjId string
		var err error
		realmObjId, err = persist.GetRealmObjIdByRealmName(txn, newRealm.getName())
		if err != nil { return err }
		if realmObjId != "" {
			return util.ConstructError(
				"A realm with name '" + newRealm.getName() + "' already exists")
		}

		// Write realm to realm hash.
		var added bool
		added, err = persist.RedisClient.HSet(RealmHashName, newRealm.getName(), newRealm.getId())
		if err != nil { debug.PrintStack() }
		if err != nil { return err }
		if ! added { return util.ConstructError("Unable to add realm " + newRealm.getName()) }
		
		persist.realmMap[newRealm.getName()] = newRealm.getId()
		err = persist.updateObject(txn, newRealm)
		if err != nil { return err }
		
		return nil
	}
}

/*******************************************************************************
 * Return a map of the Name/Ids of all of the realms in the database.
 */
func (persist *Persistence) dbGetAllRealmIds(txn TxnContext) (map[string]string, error) {
	if persist.InMemoryOnly {
		return persist.realmMap, nil
	} else {
		var realmMap map[string]string
		var err error
		realmMap, err = persist.RedisClient.HGetAll(RealmHashName)
		if err != nil { debug.PrintStack() }
		if err != nil { return nil, err }
		return realmMap, nil
	}
}

/*******************************************************************************
 * Insert a new User into the databse. This automatically inserts the
 * underlying persistent object.
 */
func (persist *Persistence) addUser(txn TxnContext, user User) error {
	if persist.InMemoryOnly {
		persist.allUserIds[user.getUserId()] = user.getId()
		return persist.updateObject(txn, user)
	} else {
		var err = persist.updateObject(txn, user)
		if err != nil { return err }
		
		// Check if the user already exists in the set.
		var userObjId string
		userObjId, err = persist.GetUserObjIdByUserId(txn, user.getUserId())
		if err != nil { return err }
		if userObjId != "" {
			return util.ConstructError(
				"A user with name '" + user.getName() + "' already exists")
		}
		
		// Write user to user-id hash.
		var added bool
		added, err = persist.RedisClient.HSet(UserHashName, user.getUserId(), user.getId())
		if err != nil { return err }
		if ! added { return util.ConstructError("Unable to add user " + user.getName()) }
		
		// Write user object to database.
		err = persist.updateObject(txn, user)
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
	
	/*
	if persist.Server.Debug {
		var client *InMemClient
		client, err = NewInMemClient(persist.Server)
		if err != nil { return err }
		client.createTestObjects()
		client.commit()
	}
	*/
	
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
	persist.clearCache()
}

/*******************************************************************************
 * Clear all objects in the in-memory cache, so that future requests will have
 * to reload the data. Do not reset the uniqueId value.
 */
func (persist *Persistence) clearCache() {
	persist.realmMap = make(map[string]string)
	persist.allObjects = make(map[string]PersistObj)
	persist.allUserIds = make(map[string]string)
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

/*******************************************************************************
 * 
 */
func getRedisTransaction(txn TxnContext) *goredis.Transaction {
	return txn.(*GoRedisTransactionWrapper).GoRedisTransaction
}
