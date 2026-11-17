package model

import "strings"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Short returns the short, upper-case name of the course the way the
// prototype shows it: "DanceFIT Set 35" is "SET 35", "DanceFIT Hot Extra" is
// "HOT EXTRA", and a name that ends with " DanceFIT" loses that suffix. The
// production names carry a registered mark and a subtitle, `DanceFIT® Set 35:
// "Cùng nhảy cùng hát"`: the mark goes and the name stops at the first colon,
// so that it is "SET 35" too.
func (c Course) Short() string { return strings.ToUpper(c.ShortName()) }

// ShortName is Short in the case of the name: "Set 35".
func (c Course) ShortName() string {
	name := strings.ReplaceAll(c.Name, "®", "")
	if head, _, ok := strings.Cut(name, ":"); ok {
		name = head
	}
	name = strings.TrimSpace(name)
	if rest, ok := strings.CutPrefix(name, "DanceFIT "); ok {
		name = rest
	} else {
		name = strings.TrimSuffix(name, " DanceFIT")
	}

	return strings.TrimSpace(name)
}
