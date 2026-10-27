package module

// All lists the built-in modules for this OS, in display order.
func All() []Module {
	return append(platform(), Go(), Packages(), Docker(), Android())
}
