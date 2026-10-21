# @summary Settings every managed node gets.
class fk::base {
  file { '/etc/motd':
    ensure  => file,
    content => "This server is managed by Puppet. Role: ${facts['node_role']}. Manual changes will be reverted.\n",
  }
}
