# Certificate related tasks.
#
# NOTE: This is not a silent makefile: the user is prompted for cert fields.
#
# make -f certs.mk cacert	# Creates a CA cert for scaledmarkets.
#
# make -f certs.mk shcert	# Creates cert for SafeHarborServer.
#
# make -f certs.mk clean	# Removes the cert and keys for the auth server.
#
# make -f certs.mk cleanall	# Removes all certs and keys.


# Certificate related values:
SafeHarborServerName = SafeHarborServer
SafeHarborServerIPAddr = 127.0.0.1
LocalPrivateKeyPath = $(SafeHarborServerName).key
LocalPemPath = $(SafeHarborServerName).pem
LocalCertPath = $(SafeHarborServerName).crt


# -------------------- Nothing below here should need to change ----------------


.DELETE_ON_ERROR:
.ONESHELL:
.SUFFIXES:
.DEFAULT_GOAL: all

SHELL = /bin/sh

CURDIR=$(shell pwd)

.PHONY: cacert extfile authcert showcert
.DEFAULT: all

all: authcert

# Make a self-signed "CA" cert. This is needed for signing the server certs.
cacert: scaledmarkets.key scaledmarkets.pem
	sudo openssl req -x509 -nodes -newkey rsa:2048 \
		-keyout scaledmarkets.key -out scaledmarkets.pem
	sudo openssl x509 -outform der -in scaledmarkets.pem -out scaledmarkets.crt

# This is needed because if a TLS server is identified by IP address, then that
# IP address must be defined in the cert as a Subject Alternative Name (SAN).
extfile: extfile.cnf
	(echo subjectAltName = IP:$(SafeHarborServerIPAddr)) > extfile.cnf

# Make a server cert for a SafeHarbor server.
# The user will be prompted for values.
# https://stackoverflow.com/questions/22666163/golang-tls-with-selfsigned-certificate
# https://serverfault.com/questions/611120/failed-tls-handshake-does-not-contain-any-ip-sans
# https://github.com/elastic/logstash-forwarder/issues/221
# Note: On Mac, openssl.cnf is in /System/Library/OpenSSL
shcert: extfile cacert
	sudo openssl req -nodes -keyout $(LocalPrivateKeyPath) -out req.pem -newkey rsa:2048
	sudo openssl x509 -req -days 365 -in req.pem -out $(LocalPemPath) \
		-CA scaledmarkets.pem -CAkey scaledmarkets.key -CAcreateserial \
		-extfile extfile.cnf  # specifies subjectAltName
	sudo openssl x509 -outform der -in $(LocalPemPath) -out $(LocalCertPath)
	sudo chmod 740 $(LocalPrivateKeyPath)
	sudo chmod 740 $(LocalPemPath)
	# use 'password' as PEM passphrase.

showcert:
	openssl x509 -in $(LocalPemPath) -noout -text

# Remove the cert and keys for the SafeHarbor server.
clean:
	rm -f $(SafeHarborServerName).*

# Remove all certs and keys (incl. the scaledmarkets CA ones).
cleanall: clean
	rm -f scaledmarkets.*
