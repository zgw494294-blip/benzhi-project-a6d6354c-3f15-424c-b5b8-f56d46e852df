package domain

func (a *Aggregate) Clone() (*Aggregate, error) {
	result := *a
	result.Replicates = make(map[string]Replicate, len(a.Replicates))
	for id, replicate := range a.Replicates {
		result.Replicates[id] = replicate
	}
	result.Observations = make(map[string][]Observation, len(a.Observations))
	for id, observations := range a.Observations {
		result.Observations[id] = observations
	}
	result.Deviations = make(map[string]Deviation, len(a.Deviations))
	for id, deviation := range a.Deviations {
		result.Deviations[id] = deviation
	}
	result.ReviewItems = make(map[string]ReviewItem, len(a.ReviewItems))
	for id, item := range a.ReviewItems {
		result.ReviewItems[id] = item
	}
	result.Reviews = append([]Review(nil), a.Reviews...)
	result.Audit = append([]AuditEntry(nil), a.Audit...)
	return &result, nil
}
