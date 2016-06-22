package server

import (
	"fmt"
	"time"
	"safeharbor/utils"
)

const (
		IdentityVerificationTokenLifespanInHours = 72
)

/*******************************************************************************
 * Store EmailAddress in Joe’s account, and flag it as unverified.
 * This method does not commit the transaction.
 */
func EstablishEmail(authSvc *AuthService, dbClient DBClient, emailSvc *utils.EmailService,
	userId string, emailAddress string) error {
	
	fmt.Println("EstablishEmail: A")  // debug
	var user User
	var err error
	user, err = dbClient.dbGetUserByUserId(userId)
	fmt.Println("EstablishEmail: B")  // debug
	if err != nil { return err } // invalid user Id
	fmt.Println("EstablishEmail: C")  // debug
	if user == nil { return utils.ConstructUserError("Unrecognized user Id") }
	fmt.Println("EstablishEmail: D")  // debug
	user.setUnverifiedEmailAddress(emailAddress)  // sets a unvalidated
	fmt.Println("EstablishEmail: E")  // debug
	
	if dbClient.getServer().PerformEmailIdentityVerification {
	fmt.Println("EstablishEmail: F")  // debug
		// Send email to user, containing the URL to click.
		return ValidateEmail(authSvc, dbClient, emailSvc, userId, emailAddress)
	} else {
	fmt.Println("EstablishEmail: G")  // debug
		return user.flagEmailAsVerified(emailAddress)
	}
}

/*******************************************************************************
 * Send email to the specified address.
 * Embed unforgeable, token containing a digest of the email address and a unique
 * token Id.
 */
func ValidateEmail(authSvc *AuthService, dbClient DBClient, emailSvc *utils.EmailService, userId, emailAddress string) error {
	
	fmt.Println("ValidateEmail: A")  // debug
	var token string
	var err error
	token, _, err = createEmailToken(authSvc, dbClient, userId)
	fmt.Println("ValidateEmail: B")  // debug
	if err != nil { return err }
	fmt.Println("ValidateEmail: C")  // debug
	
	var confirmationURL = constructConfirmationURL(dbClient.getServer(), token)
	fmt.Println("ValidateEmail: D")  // debug
	
	var message = fmt.Sprintf(
		"Click <a href=\"%s\">here</a> to confirm your email address",
		confirmationURL)
	fmt.Println("ValidateEmail: E")  // debug
	
	return emailSvc.SendEmail(emailAddress, "Verify address", message)
}

/*******************************************************************************
 * 
 */
func ValidateEmailToken(dbClient DBClient, authSvc *AuthService,
	token string) (userId string, emailAddress string, err error) {
	
	if ! authSvc.validateSessionId(token) {
		return "", "", utils.ConstructUserError("Token is not valid")
	}
	
	// Check if it is in the map of pending tokens.
	var infoObjId string
	var info IdentityValidationInfo
	infoObjId, err = dbClient.getPersistence().getIdentityValidationInfoByToken(token)
	if infoObjId != "" {
		// Retrieve userId and email address.
		info, err = dbClient.getIdentityValidationInfo(infoObjId)
		if err != nil { return "", "", err }
		var user User
		user, err = dbClient.dbGetUserByUserId(info.getUserId())
		if err != nil { return "", "", err }
		if user == nil { return "", "", utils.ConstructUserError("User not found") }
		userId = user.getUserId()
		emailAddress = user.getEmailAddress()
		
	} else {
		return "", "", utils.ConstructUserError("Token not recognized")
	}
	
	// Check if the token is expired.
	var duration = time.Duration(IdentityVerificationTokenLifespanInHours)*time.Hour
	if time.Now().After(info.getCreationTime().Add(duration)) {
		return "", "", utils.ConstructUserError("Token expired")
	}
		
	// Remove from map of pending tokens.
	err = dbClient.getPersistence().remIdentityValidationInfo(token)
	
	return userId, emailAddress, err
}


/*****************************Internal Methods*********************************/


/*******************************************************************************
 * 
 */
func createEmailToken(authSvc *AuthService, dbClient DBClient, userId string) (string, IdentityValidationInfo, error) {
	var token = authSvc.createUniqueSessionId()
	var info IdentityValidationInfo
	var err error
	info, err = dbClient.dbCreateIdentityValidationInfo(userId, time.Now(), token)
	if err != nil { return "", nil, err }
	return token, info, nil
}

/*******************************************************************************
 * 
 */
func constructConfirmationURL(server *Server, token string) string {
	var baseURL string = server.GetBasePublicURL()
	var restMethodName = "validateAccountVerificationToken"
	return fmt.Sprintf("%s/%s?AccountVerificationToken=%s", baseURL, restMethodName, token)
}
