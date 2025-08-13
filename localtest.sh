#!/bin/bash
rm shadow.log
bash build.sh
sleep 10
date
./shadowyapparatus node > shadow.stdout 2> shadow.stderr
