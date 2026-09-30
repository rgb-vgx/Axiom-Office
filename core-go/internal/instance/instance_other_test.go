//go:build !windows

package instance

import "testing"

// Mot Core cho moi nguoi dung (muc 4.3): khoa dang giu thi instance thu hai phai bi tu choi.
// (Truong hop hai TIEN TRINH that duoc bo e2e phu o ca hai nen tang; o day kiem tra ngay trong tien trinh,
// chi chay tren Linux/macOS vi mutex cua Windows la reentrant theo thread.)
func TestAcquireIsExclusive(t *testing.T) {
	name := "Local\\AxiomOffice.Core.Test" + t.Name()

	first, ok, err := Acquire(name)
	if err != nil || !ok {
		t.Fatalf("lan dau phai giu duoc khoa: ok=%t err=%v", ok, err)
	}

	if second, ok2, err2 := Acquire(name); ok2 || second != nil || err2 != nil {
		t.Fatalf("lan hai phai bi tu choi: ok=%t lock=%v err=%v", ok2, second, err2)
	}

	first.Release()
	if after, ok3, err3 := Acquire(name); !ok3 || after == nil || err3 != nil {
		t.Fatalf("sau khi nha khoa phai giu lai duoc: ok=%t err=%v", ok3, err3)
	} else {
		after.Release()
	}

	// Ten khac nhau thi khong tranh nhau (nhieu ban Core cho cac cau hinh khac nhau khi test).
	a, okA, _ := Acquire(name + ".a")
	b, okB, _ := Acquire(name + ".b")
	if !okA || !okB {
		t.Fatal("hai ten khac nhau phai giu duoc dong thoi")
	}
	a.Release()
	b.Release()
}

func TestReleaseNilIsSafe(t *testing.T) {
	var lock *Lock
	lock.Release() // khong duoc panic
}
