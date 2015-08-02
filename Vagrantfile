# Create all the machines and docker containers needed to run the SafeHarbor server.
# This includes the Cesanta authorization server.

@vmboxurl = "http://opscode-vm-bento.s3.amazonaws.com/vagrant/virtualbox/opscode_centos-7.0_chef-provisionerless.box"

@CesantaPort = "5001"
@CesantaConfDir = "/home/vagrant/auth_server/config"
@CesantaSSLDir = "/home/vagrant/auth_server/ssl"
@CesantaServerName = "docker_auth"
@CesantaDockerImage = "cesanta/docker_auth"
@LocalKeyPath = "#{@CesantaServerName}.key"
@LocalPemPath = "#{@CesantaServerName}.pem"
@LocalCertPath = "#{@CesantaServerName}.crt"

@SafeHarborPort = "6000"
@SafeHarborDir = "/home/vagrant/safeharbor"
@SafeHarborServerName = "SafeHarbor"
@SafeHarborExecutable="SafeHarborServer"
@SafeHarborConfEnvVarName="SAFEHARBOR_CONFIGURATION_PATH"
@SafeHarborConfPath = "conf.json"
#@SafeHarborDockerImage = ....
#@SafeHarborExecName= ....
#@SafeHarborExecPath="#{@SafeHarborDir}/#{@SafeHarborExecName}"

Vagrant.configure(2) do |config|

	# Configure the OS.
	config.vm.box = "centos7"
	config.vm.box_url = @vmboxurl
	config.vm.network "forwarded_port", guest: @CesantaPort, host: @CesantaPort
	config.vm.network "forwarded_port", guest: @SafeHarborPort, host: @SafeHarborPort
	
	# Create directories needed for installing Cesanta.
	config.vm.provision "shell", inline: <<-SHELL
		if ! [ -d #{@CesantaConfDir} ]; then mkdir -p #{@CesantaConfDir}; fi
		if ! [ -d #{@CesantaSSLDir} ]; then mkdir -p #{@CesantaSSLDir}; fi
		sudo chown vagrant:vagrant #{@CesantaConfDir}
		sudo chown vagrant:vagrant #{@CesantaSSLDir}
	SHELL

	# Copy the Cesanta configuration file and credentials to the VM.
	config.vm.provision "file", source: "auth_config.yml", destination: @CesantaConfDir
	config.vm.provision "file", source: @LocalKeyPath, destination: @CesantaConfDir
	config.vm.provision "file", source: @LocalCertPath, destination: @CesantaConfDir
	
	# Create directories needed for installing SafeHarbor.
	config.vm.provision "shell", inline: <<-SHELL
		if ! [ -d #{@SafeHarborDir} ]; then mkdir -p #{@SafeHarborDir}; fi
		sudo chown vagrant:vagrant #{@SafeHarborDir}
	SHELL
	
	# Copy the SafeHarbor executable and config file.
	#config.vm.provision "file", source: @SafeHarborExecName, destination: @SafeHarborDir
	#config.vm.provision "file", source: "conf.json", destination: @SafeHarborDir

	# Install patch needed by RHEL7/Centos7 to make the vagrant docker provisioner work.
	config.vm.provision "shell", inline: <<-SHELL
		yum-config-manager --enable ol7_addons
		groupadd docker
		usermod -a -G docker vagrant
	SHELL

	# Install docker if needed and run the images for Cesanta and SafeHarbor.
	config.vm.provision "docker" do |d|
		
		d.run @CesantaServerName, image: @CesantaDockerImage,
			args: "-p #{@CesantaPort}:#{@CesantaPort} -v /var/log/docker_auth:/logs -v #{@CesantaConfDir}:/config:ro -v #{@CesantaSSLDir}:/ssl",
			cmd: "/config/auth_config.yml"
		
		#d.run @SafeHarborServerName, image: @SafeHarborDockerImage,
		#	args: "-p #{@SafeHarborPort}:#{@SafeHarborPort} -e #{@SafeHarborConfEnvVarName}=#{@SafeHarborConfPath}",
		#	cmd: "#{@SafeHarborExecPath}"
	end

end
