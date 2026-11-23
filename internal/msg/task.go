package msg

func init() {
	add(map[string]entry{
		"err.install":    {EN: "the task %q could not be registered", NL: "de taak %q kon niet worden geregistreerd"},
		"err.uninstall":  {EN: "the task %q could not be removed", NL: "de taak %q kon niet worden verwijderd"},
		"task.installed": {EN: "Task %q registered.", NL: "Taak %q geregistreerd."},
		"task.removed":   {EN: "Task %q removed.", NL: "Taak %q verwijderd."},
	})
}
