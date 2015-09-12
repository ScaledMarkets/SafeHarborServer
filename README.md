# SafeHarborServer
Server that provides REST API for the SafeHarbor system.
## Design and REST API
See https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
## To Build Code
1. Go to the build/Centos directory.
2. Run vagrant up

## To Deploy
1. Go to the deploy/<target-OS> directory.
2. Run <code>vagrant up</code>
3. Log into the server using vagrant ssh.
4. Run make -f certs.mk
5. Edit conf.json
6. Edit auth_config.yml

## To Start

## To Stop
