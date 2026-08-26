package domain

import "encoding/json"

func (a *Aggregate) Clone() (*Aggregate, error) {
	b, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	var result Aggregate
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	if result.Replicates == nil {
		result.Replicates = map[string]Replicate{}
	}
	if result.Observations == nil {
		result.Observations = map[string][]Observation{}
	}
	if result.Deviations == nil {
		result.Deviations = map[string]Deviation{}
	}
	if result.ReviewItems == nil {
		result.ReviewItems = map[string]ReviewItem{}
	}
	return &result, nil
}
