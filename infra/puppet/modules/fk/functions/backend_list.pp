# @summary Turn ['10.0.0.5', '10.0.0.6'] and 8000 into ['10.0.0.5:8000', '10.0.0.6:8000'].
function fk::backend_list(Array[String] $ips, Integer $port) >> Array[String] {
  $ips.map |$ip| { "${ip}:${port}" }
}
