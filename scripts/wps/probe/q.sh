#!/bin/bash
# q.sh APP NAME [WORKDIR] < code.js  -> gui mot job cho app dang chay, in ket qua JSON (cho toi 60 giay)
J=${3:-/tmp/axiom-wps-probe/$1}/jobs
cat > $J/$2.js
for i in $(seq 120); do [ -f $J/$2.out.json ] && { cat $J/$2.out.json; exit 0; }; sleep 0.5; done
echo "TIMEOUT"
