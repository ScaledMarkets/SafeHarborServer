[ZED](https://www.owasp.org/index.php/OWASP_Zed_Attack_Proxy_Project) is an extremely powerful attack tool. It is intended to be used to find vulnerabilities in websites and REST/SOAP services, so that the vulnerabilities can be fixed before public deployment. This is called *penetration ("pen") testing*. Strictly speaking, true pen testing is done by a human, but it is also very useful to perform automated pen testing via a script, to have an ongoing assessment of the application's security profile. That is what we will be doing.
### How ZED Is Used - Concepts
ZED can be used in a myriad of ways. However, for our purposes, the way that we will be using it initially is as a headless (script-driven) attack tool, performing a standard scan while not logged into the application. To do this, the application must be running, and ZED must be started and pointed at the application. ZED will run for awhile, collect results, and then generate a report.
### Installing ZAP
ZAP must be installed on the client machine - in our case, the CI (Jenkins) server. The script `tools/ZAP/deploy.sh` performs the installation.
### How To Perform a Penetration Test - Procedure
Once installed, ZAP can be used to perform penetration tests. To perform an automated pen test, deploy and run our application, and then run one of the pen testing scripts in our various application projects, such as
```
users-microservice/run-pen-test.sh
```
(Note: The load on the client (Jenkins) from a pen test is moderate - but probably not enough to justify usnig a separate VM as a client. However, if the load proves to be a problem by slowing down Jenkins, we can containerize ZAP and set resource limits on the container so that it does not slow down Jenkins.)
