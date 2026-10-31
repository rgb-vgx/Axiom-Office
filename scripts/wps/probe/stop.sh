#!/bin/bash
# stop.sh APP [WORKDIR]: dung app + server + Xvfb cua start.sh. Moi tien trinh chay setsid nen giet ca nhom theo
# pid ghi lai - KHONG dung pkill -f (mau khop ca dong lenh cua chinh shell dang chay no).
W=${2:-/tmp/axiom-wps-probe/$1}
for f in app server xvfb; do
  p=$(cat "$W/$f.pid" 2>/dev/null) || continue
  kill -TERM -- "-$p" 2>/dev/null || kill "$p" 2>/dev/null || true
done
