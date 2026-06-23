package transcribe

import "testing"

func TestIsStorePythonStub(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{`C:\Users\me\AppData\Local\Microsoft\WindowsApps\python.exe`, true},
		{`C:\Users\me\AppData\Local\Microsoft\WindowsApps\python3.exe`, true},
		{`C:\Users\me\AppData\Local\Programs\Python\Python312\python.exe`, false},
		{`/usr/bin/python3`, false},
	}
	for _, tc := range cases {
		if got := isStorePythonStub(tc.path); got != tc.want {
			t.Errorf("isStorePythonStub(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
