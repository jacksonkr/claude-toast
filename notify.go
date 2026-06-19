package main

// notification is the platform-independent payload handed to the per-OS
// showNotification implementation (see notify_<goos>.go).
type notification struct {
	AppName  string   // app identity shown by the OS (e.g. "Claude")
	Title    string   // bold first line / window title
	Lines    []string // body lines, top to bottom
	IconPath string   // absolute path to a PNG image, may be ""
}
