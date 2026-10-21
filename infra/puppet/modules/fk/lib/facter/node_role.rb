# Reads the role Ansible wrote to /etc/node_role (app, lb or ops).
Facter.add(:node_role) do
  setcode do
    File.exist?('/etc/node_role') ? File.read('/etc/node_role').strip : 'unknown'
  end
end
