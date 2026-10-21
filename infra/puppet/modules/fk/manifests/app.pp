# @summary Keep the app service healthy and its temp files cleaned.
class fk::app (
  String  $app_name        = lookup('fk::app_name'),
  Integer $cleanup_minutes = lookup('fk::cleanup_minutes'),
) {
  service { $app_name:
    ensure => running,
    enable => true,
  }

  cron { "${app_name}-cleanup":
    command => "find /var/lib/${app_name} -mindepth 1 -mmin +${cleanup_minutes} -delete",
    user    => 'root',
    minute  => '*/15',
  }
}
