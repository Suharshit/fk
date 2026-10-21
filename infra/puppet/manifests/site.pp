# Classify nodes by the custom fact `node_role` (written by Ansible to /etc/node_role)
node default {
  include fk::base

  case $facts['node_role'] {
    'app':   { include fk::app }
    'lb':    { include fk::lb }
    default: { }
  }
}
