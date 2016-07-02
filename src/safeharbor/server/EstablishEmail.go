package server

import (
	"fmt"
	"time"
//	"net/url"
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
	
	var user User
	var err error
	user, err = dbClient.dbGetUserByUserId(userId)
	if err != nil { return err } // invalid user Id
	if user == nil { return utils.ConstructUserError("Unrecognized user Id") }
	err = user.setUnverifiedEmailAddress(dbClient, emailAddress)  // sets a unvalidated
	if err != nil { return err }
	
	if dbClient.getServer().PerformEmailIdentityVerification {
		// Send email to user, containing the URL to click.
		return ValidateEmail(authSvc, dbClient, emailSvc, userId, emailAddress)
	} else {
		return user.flagEmailAsVerified(dbClient, emailAddress)
	}
}

/*******************************************************************************
 * Send email to the specified address.
 * Embed unforgeable, token containing a digest of the email address and a unique
 * token Id.
 */
func ValidateEmail(authSvc *AuthService, dbClient DBClient, emailSvc *utils.EmailService,
	userId, emailAddress string) error {
	
	var token string
	var err error
	token, _, err = createEmailToken(authSvc, dbClient, userId)
	if err != nil { return err }
	
	var confirmationURL = constructConfirmationURL(dbClient.getServer(), token)
	
	var textMessage = fmt.Sprintf(
		"In your browser, go to %s to confirm your email address", confirmationURL)
	var htmlMessage = fmt.Sprintf(
		"Click <a href=\"%s\">here</a> to confirm your email address", confirmationURL)
	
	return emailSvc.SendEmail(emailAddress, "Verify address", textMessage, htmlMessage)
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
	return fmt.Sprintf(
		"%s/%s?AccountVerificationToken=%s", baseURL, restMethodName, token)
		//"%s/%s?AccountVerificationToken=%s", baseURL, restMethodName, url.QueryEscape(token))
}
