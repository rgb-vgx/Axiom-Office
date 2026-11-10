# REPL đo object model WPS thật

Công cụ dev, không phải test tự động. Dùng khi cần **đo** một API của WPS trước khi viết fake hoặc đặc tả, hoặc khi
muốn chạy handler thật của `src/AxiomOffice.WPS/commands.js` trên WPS thật mà không cần Core hay model. Mọi số đo
"đo trên WPS 12.1.2 thật" trong `tests/wps/REFERENCE.md` từ 01/11/2026 đều lấy bằng bộ này.

Một add-in tạm (`addin/`) cứ 0,4 giây hỏi `GET /job`. Server (`server.py`) đưa ra file `<WORKDIR>/jobs/<tên>.js`;
add-in chạy nó như thân một hàm `(app, show)` rồi `POST /result`. `commands.js` được phục vụ thẳng từ
`src/AxiomOffice.WPS`, nên job gọi được `AxiomCommands.execute(app, "wpp.checkLayout", {...})`.

Không đụng HOME và hồ sơ WPS thật: app chạy dưới Xvfb với HOME tạm trong `<WORKDIR>/home`.

```bash
scripts/wps/probe/start.sh wpp 93 47993 core-go/internal/templates/default.pptx   # wps | wpp | et
# Lần đầu WPS hỏi tin cậy add-in: bấm OK
DISPLAY=:93 xdotool mousemove 696 461 click 1
echo 'return Number(app.ActivePresentation.Slides.Count)' | scripts/wps/probe/q.sh wpp dem
scripts/wps/probe/shot.sh 93 /tmp/man-hinh.png       # chụp màn hình ảo
scripts/wps/probe/stop.sh wpp
```

- `WORKDIR` mặc định `/tmp/axiom-wps-probe/<app>`; tham số thứ 5 của `start.sh` (thứ 3 của `q.sh`, thứ 2 của
  `stop.sh`) đổi được, khi cần chạy hai app cùng lúc thì dùng hai display và hai cổng khác nhau.
- `show(v)` trong job đổi object COM thành JSON (sâu 2 tầng). Muốn chắc chắn thì tự `JSON.stringify` kết quả.
- Không dùng `pkill -f` để dọn: mẫu khớp cả dòng lệnh của chính shell đang chạy. `stop.sh` giết theo pid đã ghi.
