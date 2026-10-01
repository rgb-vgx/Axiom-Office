package model

import (
	"bytes"
	"encoding/json"
)

// decodeFirst giai ma gia tri JSON dau tien trong body, bo qua phan duoi.
//
// Mot so may chu tuong thich OpenAI noi them rac sau JSON hoan chinh - thuong gap nhat la
// `data: [DONE]` cua SSE (gap that voi proxy 9router ngay 01/10/2026: body la
// `{...chat.completion...}data: [DONE]`). json.Unmarshal doi CA body phai la mot gia tri JSON nen
// bao `invalid character 'd' after top-level value`; json.Decoder chi doc mot gia tri roi dung nen
// chiu duoc duoi rac do.
func decodeFirst(body []byte, target any) error {
	return json.NewDecoder(bytes.NewReader(body)).Decode(target)
}
