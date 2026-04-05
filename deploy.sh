#!/bin/sh
exec rsync -rv monni.sh monni-arm fonts images service-account.json third_party/fbink root@$1:/mnt/us/
