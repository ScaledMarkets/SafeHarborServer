package utils

import (
	"fmt"
	"net/smtp"
	
	// SafeHarbor packages:
)


// http://docs.aws.amazon.com/ses/latest/DeveloperGuide/smtp-connect.html
const (
	SES_SMTP_hostname = "email-smtp.us-west-2.amazonaws.com"  // us-west-2
		// "email-smtp.us-east-1.amazonaws.com"  // US East
	SES_SMTP_Port = 465  // can be 25, 465 or 587
	SenderAddress = "cliff_test@cliffberg.com"
	SenderUserId = "AKIAI2FOYVEKGEZXKX6A"
	SenderPassword = "Amcjxs1E9+mFH06zM38SoyeOMfmG5sy77OC3y6ifhSJ3"
)

func SendEmail(emailAddress string, message string) error {
	
	var tLSServerName = SES_SMTP_hostname
	var auth smtp.Auth = smtp.PlainAuth("", SenderUserId, SenderPassword, tLSServerName)

	var serverHost = SES_SMTP_hostname
	var toAddress = []string{ emailAddress }
	return smtp.SendMail(serverHost + ":" + fmt.Sprintf("%d", SES_SMTP_Port),
		auth, SenderAddress, toAddress, []byte(message))
}
