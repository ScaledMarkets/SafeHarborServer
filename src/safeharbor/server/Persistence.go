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
	"errors"
	"strconv"
	//"reflect"
	//"os"
	//"time"
	//"runtime/debug"	
	
	"redis"
	
	"safeharbor/apitypes"
	//"safeharbor/docker"
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
		error: errors.New(msg),
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
 * Chosen binding: https://github.com/alphazero/Go-Redis
 * Alternative binding: https://github.com/hoisie/redis
 */
type Persistence struct {
	InMemoryOnly bool
	RedisClient redis.Client
	uniqueId int64
	allObjects map[string]PersistObj
	allUsers map[string]User  // maps user id to user
	allRealmIds []string
}

func NewPersistence(inMemoryOnly bool, redisClient redis.Client) (*Persistence, error) {
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
	if persist.InMemoryOnly {
		persist.allRealmIds = make([]string, 0)
		persist.allObjects = make(map[string]PersistObj)
		persist.allUsers = make(map[string]User)
	} else {
		//....
	}
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
		err = persist.RedisClient.Set("UniqueId", []byte(fmt.Sprintf("%d", persist.uniqueId)))
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
 * Flush an object''s state to the database.
 */
func (persist *Persistence) writeObj(objId string, json string) error {
	if persist.InMemoryOnly {
	} else {
		// Serialize (marshall) the object to JSON, and store it in redis using the
		// object's Id as the key. When the object is written out, it will be
		// written as,
		//    "<typename>": { <object fields> }
		// so that getPersistentObject will later be able to make the JSON to the
		// appropriate go type, using reflection.
		var err = persist.RedisClient.Set("obj/" + objId, []byte(json))
		if err != nil { return err }
	}
	return nil
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
 * Insert a new object into the database - making the object persistent.
 */
func (persist *Persistence) addObject(obj PersistObj, id, json string) error {
	if persist.InMemoryOnly {
		persist.allObjects[obj.getId()] = obj
	} else {
	}
	return persist.writeObj(id, json)
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
		var deleted bool
		var err error
		deleted, err = persist.RedisClient.Del("obj/" + obj.getId())
		if err != nil { return err }
		if ! deleted { return errors.New("Unable to delete object with Id " + obj.getId()) }
		return nil
	}
}

/*******************************************************************************
 * Insert a new Realm into the database. This automatically inserts the
 * underlying persistent object.
 */
func (persist *Persistence) addRealm(newRealm Realm) error {
	if persist.InMemoryOnly {
		persist.allRealmIds = append(persist.allRealmIds, newRealm.getId())
		return persist.addObject(newRealm, newRealm.getId(), newRealm.asJSON())
	} else {
		var err = persist.addObject(newRealm, newRealm.getId(), newRealm.asJSON())
		if err != nil { return err }
		var added bool
		added, err = persist.RedisClient.Sadd("realms", []byte(newRealm.getId()))
		if err != nil { return err }
		if ! added { return errors.New("Unable to add realm " + newRealm.getName()) }
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
		var byteArAr [][]byte
		var err error
		byteArAr, err = persist.RedisClient.Smembers("realms")
		if err != nil { return nil, err }
		var results []string = make([]string, 0)
		for _, byteAr := range byteArAr {
			results = append(results, string(byteAr))
		}
		return results, nil
	}
}

/*******************************************************************************
 * Insert a new User into the databse. This automatically inserts the
 * underlying persistent object.
 */
func (persist *Persistence) addUser(user User) error {
	if persist.InMemoryOnly {
		persist.allUsers[user.getUserId()] = user
		return persist.addObject(user, user.getId(), user.asJSON())
	} else {
		var err = persist.addObject(user, user.getId(), user.asJSON())
		if err != nil { return err }
		var added bool
		added, err = persist.RedisClient.Sadd("users", []byte(user.getId()))
		if err != nil { return err }
		if ! added { return errors.New("Unable to add user " + user.getName()) }
		return nil
	}
}
