package utils

import (
	"fmt"
	"net/smtp"
	"math"
	
	// SafeHarbor packages:
)

// http://docs.aws.amazon.com/ses/latest/DeveloperGuide/smtp-connect.html
func (emailSvc *EmailService) SendEmail(emailAddress string, message string) error {
	
	var tLSServerName = emailSvc.SES_SMTP_hostname
	var auth smtp.Auth = smtp.PlainAuth("", emailSvc.SenderUserId, emailSvc.SenderPassword, tLSServerName)

	var serverHost = emailSvc.SES_SMTP_hostname
	var toAddress = []string{ emailAddress }
	return smtp.SendMail(serverHost + ":" + fmt.Sprintf("%d", emailSvc.SES_SMTP_Port),
		auth, emailSvc.SenderAddress, toAddress, []byte(message))
}

func CreateEmailService(emailConfig map[string]interface{}) (*EmailService, error) {
	
	var exists bool
	var obj interface{}
	var isType bool
	
	var hostname string
	obj, exists = emailConfig["SES_SMTP_hostname"]
	if ! exists { return nil, ConstructUserError("No SES_SMTP_hostname") }
	hostname, isType = obj.(string)
	if ! isType { return nil, ConstructUserError("SES_SMTP_hostname is not a string") }
	
	var fport float64
	obj, exists = emailConfig["SES_SMTP_Port"]
	if ! exists { return nil, ConstructUserError("No SES_SMTP_Port") }
	fport, isType = obj.(float64)
	if ! isType { return nil, ConstructUserError("SES_SMTP_Port is not a number") }
	if math.Ceil(fport) != fport { return nil, ConstructUserError("Fractional number for SES_SMTP_Port") }
	var port int = int(fport)
	
	var senderAddress string
	obj, exists = emailConfig["SenderAddress"]
	if ! exists { return nil, ConstructUserError("No SenderAddress") }
	senderAddress, isType = obj.(string)
	if ! isType { return nil, ConstructUserError("SenderAddress is not a string") }
	
	var senderUserId string
	obj, exists = emailConfig["SenderUserId"]
	if ! exists { return nil, ConstructUserError("No SenderUserId") }
	senderUserId, isType = obj.(string)
	if ! isType { return nil, ConstructUserError("SenderUserId is not a string") }
	
	var senderPassword string
	obj, exists = emailConfig["SenderPassword"]
	if ! exists { return nil, ConstructUserError("No SenderPassword") }
	senderPassword, isType = obj.(string)
	if ! isType { return nil, ConstructUserError("SenderPassword is not a string") }
	
	return &EmailService{
		SES_SMTP_hostname: hostname,
		SES_SMTP_Port: port,
		SenderAddress: senderAddress,
		SenderUserId: senderUserId,
		SenderPassword: senderPassword,
	}, nil
}

type EmailService struct {
	SES_SMTP_hostname string
	SES_SMTP_Port int
	SenderAddress string
	SenderUserId string
	SenderPassword string
}
