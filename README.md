# SafeHarborServer
Server that provides REST API for the SafeHarbor system.
## Design and REST API
See https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
## To Build Code
1. Go to the <code>build/Centos</code> directory.
2. Run <code>vagrant up</code>

## To Deploy
1. Go to the <code>deploy/</code>(target-OS) directory.
2. Edit <code>safeharbor.conf</code>.
3. Run <code>./deploy.sh</code>
4. Log into the server using <code>vagrant ssh</code>.
5. Run <code>make -f certs.mk</code>
6. Edit <code>conf.json</code>
7. Edit <code>auth_config.yml</code>
8. Log out of the server.

## To Start
<code>./start.sh</code>

## To Stop
<code>./stop.sh</code>
