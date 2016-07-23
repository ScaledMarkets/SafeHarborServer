# Run integration test suite.
# Run this on the machine that hosts the containers under test.
# Assumes that SafeHarborServer has been built with the TEST option and deployed
# to a test environment. It also assumes that the test environment has go installed.
# Arguments:
#	$1 - IP address of the test server.
#	$2 - Port of the test server.
#	$3 - The test suite to run, as defined in the test project makefile, e.g., 'basic'.
# Example:
#	./test.sh 127.0.0.1 6000 basic

pushd $( dirname "${BASH_SOURCE[0]}" )/../../../TestSafeHarborServer
export TestDir=`pwd`

# Update the test code and compile it.
echo Pulling test source code...
git pull
echo Building tests...
make compile

# Execute tests.
echo Executing test suite $3...
make $3

# Determine code coverage. Requires running report in the SafeHarborServer container.
# We achieve that by attaching to the container.
# See https://www.elastic.co/blog/code-coverage-for-your-golang-system-tests
# See https://blog.golang.org/cover
echo Attaching to SHS container to run coverage report...
docker attach --detach-keys detach safeharborserver
go tool cover -html=/safeharbor/data/safeharbor.cov -o /safeharbor/data/safeharbor.cov.html
detach

echo The HTML coverage report is in $DataVolMountPoint/safeharbor.cov.html

popd
