package tools

import (
	"strings"

	"axiomoffice/core/internal/model"
)

// Ket qua mo ho cua lenh ghi (audit agent runtime 26/10/2026, F01). Bridge bao loi nhung lenh co the DA chay trong Office:
// CommandWorker tra HOST_BUSY sau 120 s trong khi lenh COM van chay tiep (khong huy duoc), Core het gio cho bridge, add-in
// WPS het gio sau khi da nhan lenh. Truoc day Core coi do la "khong sua gi": revision giu nguyen, va model goi lai y het
// lenh do -> doan van / bang / slide xuat hien hai lan. Gio: coi nhu tai lieu CO THE da doi, va chan goi lai y het cho
// toi khi model doc lai tai lieu.

// uncertainMarkers: cach cac bridge bao "lenh da gui, ket qua khong ro". HOST_BUSY "was not run" (lenh con xep hang bi
// bo) va "not sent, safe to retry" cua WPS la lenh CHUA chay - khong nam day.
var uncertainMarkers = []string{
	"has not finished",             // CommandWorker.cs: HOST_BUSY ... (it is still running)
	"bridge did not answer within", // office/bridge.go: Core het gio cho /cmd
	"bridge timeout",
	"may have been applied", // wpsbridge: add-in nhan lenh roi khong tra loi kip
}

// uncertainOutcome: loi nay cho thay lenh co the da chay xong trong Office.
func uncertainOutcome(errorText string) bool {
	for _, marker := range uncertainMarkers {
		if strings.Contains(errorText, marker) {
			return true
		}
	}
	return false
}

// writesTemporarily: lenh doc nhung ghi tam vao tai lieu roi tra lai (et.whatIf) - het gio giua chung thi o co the con gia
// tri thu.
var writesTemporarily = map[string]bool{"et.whatIf": true}

func mayChangeDocument(action string) bool {
	return ChangesDocument(action) || writesTemporarily[action]
}

// uncertainNote: noi them vao loi tra cho model.
const uncertainNote = "The command may still have been applied in the document (the host was busy or did not answer in " +
	"time). Do NOT send it again blindly: first read the affected part of the document (e.g. writer.getText, et.readRange, " +
	"wpp.listSlides) to see whether the change is already there, then continue."

func uncertainKey(action string, params map[string]any) string {
	return action + " " + model.MarshalRelaxed(params)
}

// rememberUncertain: lenh ghi nay co ket qua khong ro.
func (run *RunContext) rememberUncertain(action string, params map[string]any) {
	if run.uncertain == nil {
		run.uncertain = map[string]bool{}
	}
	run.uncertain[uncertainKey(action, params)] = true
}

// checkUncertainRetry: goi lai Y HET mot lenh ghi co ket qua khong ro khi chua doc lai tai lieu -> tu choi. Lenh doc thanh
// cong xoa danh sach (model da co dip thay trang thai that).
func (run *RunContext) checkUncertainRetry(action string, params map[string]any) string {
	if !run.uncertain[uncertainKey(action, params)] {
		return ""
	}
	return "blocked: the previous identical " + action + " call may already have been applied (its result was unknown). " +
		"Read the document first to check whether the change is already there; only send it again if it is missing."
}

func (run *RunContext) forgetUncertain() { run.uncertain = nil }
