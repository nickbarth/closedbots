package domain

func ResetStepStatuses(p *Task) {
	for i := range p.Steps {
		p.Steps[i].Status = StepPending
		p.Steps[i].LastError = ""
		p.Steps[i].Attempts = 0
	}
}
