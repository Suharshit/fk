# @summary nginx load balancer built from facts and Hiera data.
class fk::lb (
  String        $app_name     = lookup('fk::app_name'),
  Integer       $app_port     = lookup('fk::app_port'),
  Array[String] $app_backends = lookup('app_backends'),
) {
  # custom function (functions/backend_list.pp) → ['10.0.0.5:8000', ...]
  $backends = fk::backend_list($app_backends, $app_port)

  package { 'nginx':
    ensure => installed,
  }

  file { '/etc/nginx/sites-available/default':
    ensure  => file,
    content => template('fk/lb.conf.erb'),
    require => Package['nginx'],
    notify  => Service['nginx'],
  }

  service { 'nginx':
    ensure => running,
    enable => true,
  }
}
