resource "icinga2_host" "test" {
  hostname      = "test"
  address       = "127.0.0.1"
  check_command = "hostalive"
  zone          = "master"
}

resource "time_static" "downtime_start" {}

resource "icinga2_downtime" "test" {
  type         = "Host"
  filter       = "host.name==\"${icinga2_host.test.id}\""
  author       = "terraform"
  comment      = "Initial downtime"
  start_time   = time_static.downtime_start.unix
  end_time     = time_static.downtime_start.unix + 3600
  all_services = false
}
