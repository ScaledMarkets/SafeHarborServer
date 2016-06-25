package utils

import (
	"fmt"
	"net/smtp"
	"math"
	"reflect"
	
	// SafeHarbor packages:
)

// http://docs.aws.amazon.com/ses/latest/DeveloperGuide/smtp-connect.html
// For limit increase: https://console.aws.amazon.com/support/home?region=us-east-1#/case/create?issueType=service-limit-increase&limitType=service-code-ses
func (emailSvc *EmailService) SendEmail(emailAddress, subject, textMessage, htmlMessage string) error {
	
	fmt.Println("SendEmail: A")  // debug
	
	var tLSServerName = emailSvc.SES_SMTP_hostname
	var auth smtp.Auth = smtp.PlainAuth("", emailSvc.SenderUserId, emailSvc.SenderPassword, tLSServerName)
	fmt.Println("SendEmail: B")  // debug

	var serverHost = emailSvc.SES_SMTP_hostname
	var toAddress = []string{ emailAddress }
	var hostAndPort = serverHost + ":" + fmt.Sprintf("%d", emailSvc.SES_SMTP_Port)
	fmt.Println("SendEmail: C")  // debug
	
	var fullMsg = []byte(
		"Subject: " + subject + "\r\n") +
		"To: " + emailAddress + "\r\n" +
		"From: " + emailSvc.SenderAddress + "\r\n" +
		"Source: " + emailSvc.SenderAddress + "\r\n" +
		"Sender: " + emailSvc.SenderAddress + "\r\n" +
		"Return-Path: " + emailSvc.SenderAddress + "\r\n" +
		"Content-Type: multipart/alternative; boundary=bcaec520ea5d6918e204a8cea3b4" + "\r\n" +
		"\r\n" +
		"--bcaec520ea5d6918e204a8cea3b4" + "\r\n" +
		"Content-Type: text/plain; charset=utf-8" + "\r\n" +
		"\r\n" +
		textMessage + "\r\n"
		"\r\n" +
		"--bcaec520ea5d6918e204a8cea3b4" + "\r\n" +
		"Content-Type: text/html; charset=utf-8" + "\r\n" +
		"Content-Transfer-Encoding: quoted-printable" "\r\n" +
		"\r\n" +
		htmlMessage + "\r\n"
		"\r\n" +
		"--bcaec520ea5d6918e204a8cea3b4"
	)

	fmt.Println("SendEmail: D")  // debug
	var err = smtp.SendMail(hostAndPort, auth, emailSvc.SenderAddress, toAddress, fullMsg)
	fmt.Println("SendEmail: E")  // debug
	if err == nil { fmt.Println(
		"Sent email from " + emailSvc.SenderAddress + " to " + emailAddress +
		" on host/port " + hostAndPort) } // debug
	if err != nil { fmt.Println("SendEmail: when calling SendMail: " + err.Error()) } // debug
	return err
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
	if ! isType { return nil, ConstructUserError(
		"SES_SMTP_Port is not a number: it is a " + reflect.TypeOf(obj).String()) }
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
