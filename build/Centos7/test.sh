# Run integration test suite.
# Assumes that SafeHarborServer has been built with the TEST option and deployed
# to a test environment. It also assumes that the test environment has go installed.
# Arguments:
#	$1 - IP address of the test server.
#	$2 - Port of the test server.
#	$3 - The test suite to run, as defined in the test project makefile, e.g., 'basic'.
# Example:
#	./test.sh 127.0.0.1 6000 basic

pushd $PROJECTROOT/../TestSafeHarborServer

# Update the test code and compile it.
sudo git pull
make compile

# Execute tests.
make $3

# Determine code coverage. Requires running report on server - hence the ssh.
# See https://www.elastic.co/blog/code-coverage-for-your-golang-system-tests
# See https://blog.golang.org/cover
ssh -i $SSHTestKeyPath centos@$1 \
	"go tool cover -html=/safeharbor/data/safeharbor.cov -o /safeharbor/data/safeharbor.cov.html"

# Retrieve the HTML coverage report.
scp -i $SSHTestKeyPath centos@$1:$DataVolMountPoint/safeharbor.cov.html .

popd
