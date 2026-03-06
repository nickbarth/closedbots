package osctrl

import tk "modernc.org/tk9.0"

func restoreMainWindowToForeground() {
	tk.WmDeiconify(tk.App)
	tk.Update()

	// Ask the WM to briefly keep the window on top so it is raised,
	// then hand control back to normal stacking.
	tk.WmAttributes(tk.App, tk.Topmost(true))
	tk.Update()
	tk.WmAttributes(tk.App, tk.Topmost(false))

	// Force keyboard focus back to the app after restoring.
	tk.Focus(tk.Force(tk.App))
	tk.Update()
}
