# SafeHarborServer
Server that provides REST API for the SafeHarbor container security scanning system.
See also the [Safe Harbor command line client](https://github.com/ScaledMarkets/safeharborcmdclient).

## What is SafeHarborServer for?

<ul>
<li>Enables you to add container image scanning for multiple scanners to your
	dev/test/deploy pipeline without having to learn the nuances of each scanner.</li>
<li>You can run any or all of the scanners that are supported by SafeHarborServer.</li>
<li>You can add an additional scanner, using the ScanProvider API.</li>
<li>You can define access control lists to give access to your container images
	to partners in your organization or in other organizations, at an individual
	level, a team level, or an organization level.</li>
<li>You can examine the scan history of an image.</li>
<li>You can define and save a re-usable scan profile.</li>
</ul>

You can still use the value added features of each scanner. E.g., Twistlock has
powerful scan results examination features, and you can still use those features
for scans that are triggered by SafeHarborServer. The native scanner platforms
are not bypassed - they are connected to by SafeHarborServer.

## Scan Providers

The container scanners that are currently supported are,

<ul>
<li>Clair</li>
<li>Twistlock</li>
</ul>

Under development:

<ul>
<li>OpenScap</li>
<li>Lynis</li>
</ul>

You can add another scanner by implementing the
[ScanProvider API](https://github.com/ScaledMarkets/SafeHarborServer/blob/master/src/safeharbor/providers/ScanProvider.go).
At present, to add a scan provider, you must also add code to the
[Server](https://github.com/ScaledMarkets/SafeHarborServer/blob/master/src/safeharbor/server/Server.go)
module and recompile SafeHarborServer, but we have plans to create a provider API
that will not require recompilation.

## Design and REST API
See https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
## To Build Code
1. Go to the <code>build/Centos</code> directory.
2. Run <code>vagrant up</code>

## To Deploy
1. Go to the <code>deploy/</code>(target-OS) directory.
2. Run <code>make -f ../../certs.mk</code> (if you have not already done this)
3. Edit <code>safeharbor.conf</code> (usually does not need to change)
4. Run <code>./deploy.sh</code>
5. Log into the server using <code>vagrant ssh</code>.
6. Edit <code>conf.json</code> (usually does not need to change)
7. Edit <code>auth_config.yml</code> (usually does not need to change)
8. Log out of the server.

## To Start
<code>./start.sh</code>

## To Stop
<code>./stop.sh</code>
 trigger
