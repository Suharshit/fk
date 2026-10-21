# Local copy of the AWS layout. Needs VirtualBox and Vagrant.
NODES = {
  "lb-1"  => { ip: "192.168.56.10", mem: 512  },
  "app-1" => { ip: "192.168.56.11", mem: 1024 },
  "app-2" => { ip: "192.168.56.12", mem: 1024 },
  "ops-1" => { ip: "192.168.56.20", mem: 3072 },  # Puppet server + Nagios
}

Vagrant.configure("2") do |config|
  config.vm.box = "bento/ubuntu-24.04"
  NODES.each do |name, opts|
    config.vm.define name do |node|
      node.vm.hostname = name
      node.vm.network "private_network", ip: opts[:ip]
      node.vm.provider "virtualbox" do |vb|
        vb.memory = opts[:mem]
        vb.cpus   = 1
      end
    end
  end
end
