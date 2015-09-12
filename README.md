# SafeHarborServer
Server that provides REST API for the SafeHarbor system.
## Design and REST API
See https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
## To Build Code
1. Go to the <code>build/Centos</code> directory.
2. Run <code>vagrant up</code>

## To Deploy
1. Go to the <code>deploy/</code>(target-OS) directory.
2. Run <code>vagrant up</code>
3. Log into the server using <code>vagrant ssh</code>.
4. Run <code>make -f certs.mk</code>
5. Edit <code>conf.json</code>
6. Edit <code>auth_config.yml</code>

## To Start
<code>./start.sh</code>

## To Stop
<code>./stop.sh</code>
