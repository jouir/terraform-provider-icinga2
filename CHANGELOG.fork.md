## 0.9.0 (March 29, 2026)

ENHANCEMENTS:
* Sync with upstream
* Add imports of icinga2_host and icinga2_hostgroup resources
* Retry logic in icinga2_host and icinga2_hostgroup creation and deletion

## 0.8.0 (March 25, 2026)

BUG FIXES:
* Do not raise an error if the icinga2_hostgroup is not found on the infrastructure when deleted

ENHANCEMENTS:
* Add zone support to icinga2_hostgroup

## 0.7.1 (March 18, 2026)

BUG FIXES:
* Retry to create host when deadline exceeded

## 0.7.0 (March 16, 2026)

ENHANCEMENTS:
* Add icinga2_downtime resource
* Add zone attribute to icinga2_host resource
