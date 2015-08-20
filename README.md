# SafeHarborServer
Server that provides REST API for the SafeHarbor system.
## Design and REST API
See https://drive.google.com/open?id=1r6Xnfg-XwKvmF4YppEZBcxzLbuqXGAA2YCIiPb_9Wfo
## To Build Code Without Creating New Certificates
1. Install go.
2. Clone the SafeHarborServer repo.
3. In the SafeHarborServer directory, execute "make compile".
4. Obtain the .pem and .key files for both certificates (docker_auth and scaledmarkets) and put these files in the SafeHarborServer directory.

## To Run Server
./run.sh
